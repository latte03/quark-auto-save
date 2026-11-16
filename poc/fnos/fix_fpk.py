#!/usr/bin/env python3
"""Fix the exec bits fnpack cannot store on a Windows filesystem.

fnpack takes the mode of every file from the disk it packs. Windows (NTFS) has
no exec bit, so a package built there carries 0666 for cmd/* — and fnOS
fork/execs those scripts during install, which fails with EACCES before any
script of ours runs (see the community report of `10237 .../cmd/install_init:
permission denied`). The app binary inside app.tgz has the same problem; it is
covered at run time by `chmod +x` in cmd/main.

Usage:
  fix_fpk.py <pkg.fpk>            rewrite only the outer modes (cmd/* -> 0755);
                                  app.tgz stays byte-identical, so the manifest
                                  checksum (= md5 of app.tgz) stays valid.
  fix_fpk.py <pkg.fpk> --deep     also rewrite app.tgz (binary -> 0755) and
                                  recompute the manifest checksum.
  fix_fpk.py <pkg.fpk> --check    only report; exit 1 when a needed mode is off.

The archive layout fnpack produces:
  <pkg>.fpk  = gzip(tar) of [app.tgz, cmd/*, config/*, wizard/*, manifest, ICON*.PNG]
  app.tgz    = gzip(tar) of the app/ directory contents
manifest.checksum = md5(app.tgz)
"""

import gzip
import hashlib
import io
import os
import sys
import tarfile

EXEC_DIRS = ("cmd",)


def wanted_mode(name: str, is_dir: bool, deep: bool) -> int:
    """The mode fnpack would record on a POSIX host."""
    if is_dir:
        return 0o755
    head = name.split("/", 1)[0]
    if head in EXEC_DIRS:
        return 0o755
    return 0o644


def inner_mode(name: str, is_dir: bool) -> int:
    """Modes inside app.tgz: the payload at the root is run, data is not."""
    if is_dir:
        return 0o755
    if "/" not in name:
        # app/ is flattened into target/, so root-level files are the binaries
        # and launch scripts that fnOS execs.
        return 0o755
    return 0o644


def read_tar(blob: bytes, gz: bool) -> tarfile.TarFile:
    return tarfile.open(fileobj=io.BytesIO(blob), mode="r:gz" if gz else "r:")


def rebuild(blob: bytes, gz: bool, mode_of, rewrite) -> tuple[bytes, list[str]]:
    """Copy every member, optionally replacing content, with corrected modes."""
    src = read_tar(blob, gz)
    out = io.BytesIO()
    problems: list[str] = []
    with tarfile.open(fileobj=out, mode="w:gz" if gz else "w:", format=tarfile.GNU_FORMAT) as dst:
        for info in src.getmembers():
            want = mode_of(info.name, info.isdir())
            if not info.isdir() and (info.mode & 0o111) != (want & 0o111):
                problems.append("%s: %04o -> %04o" % (info.name, info.mode, want))
            data = None
            if info.isfile() and rewrite:
                data = src.extractfile(info).read()
            new = tarfile.TarInfo(info.name)
            new.type = info.type
            new.mode = want
            new.mtime = info.mtime
            new.uid, new.gid = info.uid, info.gid
            new.uname, new.gname = info.uname, info.gname
            new.linkname = info.linkname
            if data is not None:
                new.size = len(data)
                dst.addfile(new, io.BytesIO(data))
            elif info.isfile():
                body = src.extractfile(info).read()
                new.size = len(body)
                dst.addfile(new, io.BytesIO(body))
            else:
                dst.addfile(new)
    return out.getvalue(), problems


def fix(path: str, deep: bool, check_only: bool) -> int:
    with open(path, "rb") as fh:
        raw = fh.read()
    outer = read_tar(raw, gz=True)

    inner_blob = outer.extractfile("app.tgz").read()
    manifest = outer.extractfile("manifest").read().decode("utf-8")

    if deep and not check_only:
        inner_new, _ = rebuild(inner_blob, gz=True, mode_of=inner_mode, rewrite=True)
        if inner_new != inner_blob:
            digest = hashlib.md5(inner_new).hexdigest()
            lines = []
            for line in manifest.splitlines():
                if line.strip().startswith("checksum"):
                    lines.append("checksum = " + digest)
                else:
                    lines.append(line)
            manifest = "\n".join(lines) + "\n"
            inner_blob = inner_new

    def mode_of(name, is_dir):
        return wanted_mode(name, is_dir, deep)

    def rewrite(info):
        if info.name == "app.tgz":
            return True
        if info.name == "manifest":
            return True
        return False

    # Rebuild the outer archive, swapping in the (possibly) new app.tgz and manifest.
    src = read_tar(raw, gz=True)
    body = io.BytesIO()
    problems: list[str] = []
    with tarfile.open(fileobj=body, mode="w:gz", format=tarfile.GNU_FORMAT) as dst:
        for info in src.getmembers():
            want = mode_of(info.name, info.isdir())
            if not info.isdir() and (info.mode & 0o111) != (want & 0o111):
                problems.append("%s: %04o -> %04o" % (info.name, info.mode, want))
            data = None
            if info.name == "app.tgz":
                data = inner_blob
            elif info.name == "manifest":
                data = manifest.encode("utf-8")
            elif info.isfile():
                data = src.extractfile(info).read()
            new = tarfile.TarInfo(info.name)
            new.type = info.type
            new.mode = want
            new.mtime = info.mtime
            new.uid, new.gid = info.uid, info.gid
            new.uname, new.gname = info.uname, info.gname
            new.linkname = info.linkname
            if data is not None:
                new.size = len(data)
                dst.addfile(new, io.BytesIO(data))
            elif info.isfile():
                raw_body = src.extractfile(info).read()
                new.size = len(raw_body)
                dst.addfile(new, io.BytesIO(raw_body))
            else:
                dst.addfile(new)

    if check_only:
        for line in problems:
            print("  " + line)
        if deep:
            inner = read_tar(inner_blob, gz=True)
            for info in inner.getmembers():
                if info.isfile() and "/" not in info.name and not info.mode & 0o111:
                    print("  app.tgz:%s: %04o -> 0755" % (info.name, info.mode))
                    problems.append(info.name)
        return 1 if problems else 0

    tmp = path + ".tmp"
    with open(tmp, "wb") as fh:
        fh.write(body.getvalue())
    os.replace(tmp, path)
    for line in problems:
        print("  fixed " + line)
    checksum = hashlib.md5(inner_blob).hexdigest()
    print("  checksum(app.tgz) = %s" % checksum)
    return 0


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if not a.startswith("-")]
    if not args:
        print(__doc__)
        return 2
    deep = "--deep" in argv
    check_only = "--check" in argv
    return fix(args[0], deep, check_only)


if __name__ == "__main__":
    sys.exit(main(sys.argv))
