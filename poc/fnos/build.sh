#!/bin/bash
set -euo pipefail

# qas fpk 出包。CI 跑在 Linux；开发机可以是 macOS（权限位真实保留）或
# Windows（MSYS/MINGW：NTFS 没有执行位，fnpack 只能把 cmd/* 和 app/qas 写成
# 0666，而 fnOS 在 install/start 时是直接 fork/exec cmd/* 的，装上去会 EACCES）。
#
#   bash poc/fnos/build.sh                 本机出包 → dist/qas_<ver>_x86.fpk
#   QAS_SKIP_WEB=1                         沿用现有 web/dist（CI 已单独构建前端）
#   QAS_FPK_NAS=1 bash poc/fnos/build.sh   在 NAS 上组装（权限位真实保留）
#   QAS_FPK_FIX=1|0                        强制/禁止打包后修权限位（Windows 默认开）
#   QAS_NAS_HOST=nas                       NAS 的 ssh 别名/主机（默认 nas）
#   QAS_NAS_FNPACK=fnpack                  远端 fnpack 可执行文件
#   FNPACK=/path/to/fnpack                 本机 fnpack（默认取 PATH 里的 fnpack）
#   PYTHON=/path/to/python                 修权限位用的解释器（默认 python3/python）

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
root="$(cd "$here/../.." && pwd)"
version="$(sed -n 's/^version=//p' "$here/manifest")"
fnpack="${FNPACK:-fnpack}"
out="$root/dist"
work="$(mktemp -d "${TMPDIR:-/tmp}/qas-fpk.XXXXXX")"
stage="$work/pkg"
trap 'rm -rf "$work"' EXIT

case "$(uname -s)" in
  MINGW*|MSYS*|CYGWIN*) windows_host=1 ;;
  *) windows_host=0 ;;
esac

# 前端产物不在 git 里（web/dist 被忽略），出包前重建，免得包里悄悄是旧界面。
# CI 已经单独构建过前端，用 QAS_SKIP_WEB=1 跳过，避免重复构建。
build_web() {
  if [ "${QAS_SKIP_WEB:-}" = "1" ]; then
    echo "→ 前端：跳过（QAS_SKIP_WEB=1），沿用现有 web/dist"
    return 0
  fi
  if ! command -v pnpm >/dev/null 2>&1; then
    if [ -f "$root/web/dist/index.html" ]; then
      echo "⚠ 前端：没有 pnpm，沿用现有 web/dist（可能已过期）" >&2
      return 0
    fi
    echo "✗ 前端：没有 pnpm，且 web/dist 不存在，无法出包" >&2
    return 1
  fi
  if [ ! -d "$root/web/node_modules" ]; then
    echo "→ 前端：安装依赖（pnpm install --frozen-lockfile）"
    (cd "$root/web" && pnpm install --frozen-lockfile)
  fi
  echo "→ 前端：构建（pnpm build）"
  (cd "$root/web" && pnpm build)
}

# Windows 打出的包 cmd/* 是 0666，fnOS 直接 exec 会 EACCES；这里按 POSIX 口径
# 改写 tar 头。默认只动外层（app.tgz 字节不变，manifest 的 checksum 仍然有效），
# 包里的 app/qas 由 cmd/main 启动时 chmod +x 兜底。
fix_modes() {
  local python_bin="${PYTHON:-}"
  if [ -z "$python_bin" ]; then
    local candidate
    for candidate in python3 python; do
      if command -v "$candidate" >/dev/null 2>&1; then
        python_bin="$candidate"
        break
      fi
    done
  fi
  if [ -z "$python_bin" ]; then
    echo "⚠ 没找到 python，跳过权限位修正（cmd/* 仍是 0666，装上去可能 EACCES）" >&2
    return 0
  fi
  echo "→ 权限位：cmd/* → 0755"
  "$python_bin" "$here/fix_fpk.py" "$target"
}

build_web

mkdir -p "$stage/app/web" "$out"
(cd "$root/poc" && GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$stage/app/qas" .)
cp -R "$root/web/dist" "$stage/app/web/dist"
cp -R "$here/app/ui" "$stage/app/ui"
cp -R "$here/cmd" "$here/config" "$here/wizard" "$stage/"
chmod +x "$stage"/cmd/*
cp "$here/manifest" "$here/ICON.PNG" "$here/ICON_256.PNG" "$stage/"

target="$out/qas_${version}_x86.fpk"

if [ "${QAS_FPK_NAS:-}" = "1" ]; then
  nas="${QAS_NAS_HOST:-nas}"
  remote_fnpack="${QAS_NAS_FNPACK:-fnpack}"
  echo "→ 上传构建目录到 $nas 组装（权限位真实保留）"
  tarball="$work/pkg.tgz"
  tar -czf "$tarball" -C "$stage" .
  remote="$(ssh "$nas" 'mktemp -d /tmp/qas-fpk.XXXXXX')"
  scp -q "$tarball" "$nas:$remote/pkg.tgz"
  ssh "$nas" "set -e; cd '$remote'; tar -xzf pkg.tgz; rm -f pkg.tgz; chmod +x cmd/* app/qas; $remote_fnpack build --directory ."
  scp -q "$nas:$remote/qas.fpk" "$target"
  echo "→ 权限位核对"
  ssh "$nas" "cd '$remote' && tar -tvf qas.fpk | sed -n '1,4p'"
  ssh "$nas" "rm -rf '$remote'"
  echo "$target"
  exit 0
fi

(cd "$stage" && "$fnpack" build --directory .)
cp "$stage/qas.fpk" "$target"

if [ "$windows_host" = "1" ] || [ "${QAS_FPK_FIX:-}" = "1" ]; then
  if [ "${QAS_FPK_FIX:-}" = "0" ]; then
    echo "⚠ 跳过权限位修正（QAS_FPK_FIX=0）" >&2
  else
    fix_modes
  fi
fi

echo "$target"

if [ "$windows_host" = "1" ]; then
  cat >&2 <<'MSG'
⚠ Windows 出包：cmd/* 已修正为 0755；包里的 app/qas 仍是 0666，靠 cmd/main
  启动时的 chmod +x 兜底（install/start 都能跑，已在 NAS 上实测）。要一个
  和 CI 逐位一致的包，用：
    QAS_FPK_NAS=1 bash poc/fnos/build.sh   （在 NAS 组装，权限位真实保留）
  需要连二进制权限也修好时可跑 poc/fnos/fix_fpk.py <fpk> --deep（会重写
  manifest 的 checksum）。
MSG
fi
