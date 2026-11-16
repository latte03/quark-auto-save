#!/usr/bin/env python3
"""对比夸克 CDN 对不同 Range 跨度的给速（只读探测，不写盘）。

背景与结论见 docs/quark-download-throttle.md：请求声明的跨度越大越容易被限速，
跨度到 GB 级时会掉到约 100 KB/s。

用法：
    python3 docs/quark_span_probe.py --fid <ndrive fid> [--spans 8,64,256,900] [--seconds 6]
    QAS_COOKIE='...' python3 docs/quark_span_probe.py --fid <fid>   # 不用配置文件时

Cookie 从 QAS 配置读（默认 /vol1/@appconf/qas/config.json，可用 --config 指定），
下载地址与 Cookie 都不会打印；curl 的 URL 也只经 0600 配置文件传递。
"""
import argparse
import json
import os
import subprocess
import tempfile
import time
import urllib.error
import urllib.parse
import urllib.request

API = "https://drive-pc.quark.cn/1/clouddrive/file/download?pr=ucpro&fr=pc&uc_param_str="
UA = ("Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) "
      "quark-cloud-drive/3.14.2 Chrome/112.0.5615.165 Electron/24.1.3.8 Safari/537.36 Channel/pckk_other_ch")
MB = 1 << 20


def read_cookie(config):
    if os.environ.get("QAS_COOKIE"):
        return os.environ["QAS_COOKIE"]
    with open(config) as handle:
        return json.load(handle)["cookie"][0]


def issue_url(cookie, fid):
    """同一个 API 调用 QAS 自己用的：只要 content-type 和 UA。"""
    request = urllib.request.Request(
        API, data=json.dumps({"fids": [fid]}).encode(), method="POST",
        headers={"Cookie": cookie, "User-Agent": UA, "Content-Type": "application/json"})
    with urllib.request.urlopen(request, timeout=20) as response:
        envelope = json.load(response)
        fresh = response.headers.get_all("Set-Cookie") or []
    if not envelope.get("data"):
        raise SystemExit("download api: code=%s message=%s"
                         % (envelope.get("code"), envelope.get("message")))
    jar = "; ".join(part.split(";")[0] for part in fresh if "=" in part)
    return envelope["data"][0]["download_url"], jar or cookie


def like_qas(url, jar, start, span, seconds):
    """走 QAS 同样的指纹：夸克客户端 UA + 新下发 Cookie + 精确 Range。"""
    request = urllib.request.Request(url, headers={
        "Cookie": jar, "User-Agent": UA,
        "Range": "bytes=%d-%d" % (start, start + span - 1)})
    total, began = 0, time.time()
    try:
        with urllib.request.urlopen(request, timeout=30) as response:
            while time.time() - began < seconds:
                chunk = response.read(65536)
                if not chunk:
                    break
                total += len(chunk)
    except urllib.error.HTTPError as exc:
        raise SystemExit("HTTP %d at offset %d - 起点可能超出文件末尾，用 --start-mb 调小"
                         % (exc.code, start))
    return total / MB / max(time.time() - began, 1e-6)


def with_curl(url, jar, start, span, seconds, http_version):
    """用 curl 复核，顺便看 HTTP/1.1 与 HTTP/2 是否不同。"""
    with tempfile.TemporaryDirectory() as workdir:
        conf = os.path.join(workdir, "curl.cfg")
        with open(conf, "w") as handle:
            handle.write("\n".join([
                'url = "%s"' % url,
                'header = "cookie: %s"' % jar,
                'header = "user-agent: %s"' % UA,
                'range = "%d-%d"' % (start, start + span - 1),
                'output = "/dev/null"',
                "max-time = %d" % seconds,
                'write-out = "%{speed_download} %{http_version}"',
            ]) + "\n")
        os.chmod(conf, 0o600)
        result = subprocess.run(["curl", "-s", "-K", conf, http_version],
                                capture_output=True, text=True, timeout=seconds + 30)
    fields = result.stdout.split()
    if len(fields) != 2:
        raise SystemExit("curl: %s" % (result.stderr.strip()[:80] or "no output"))
    return float(fields[0]) / MB, fields[1]


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--fid", required=True, help="网盘文件 fid")
    parser.add_argument("--config", default="/vol1/@appconf/qas/config.json")
    parser.add_argument("--spans", default="8,64,256,900", help="逗号分隔，单位 MB")
    parser.add_argument("--seconds", type=float, default=6.0, help="每个跨度读多久")
    parser.add_argument("--start-mb", type=int, default=0, help="首个探测区间的起点（MB）")
    parser.add_argument("--skip-curl", action="store_true", help="只跑 Python 探测")
    args = parser.parse_args()

    cookie = read_cookie(args.config)
    url, jar = issue_url(cookie, args.fid)
    print("cdn host:", urllib.parse.urlsplit(url).hostname)

    start = args.start_mb * MB
    print("\n%-10s %12s" % ("span", "MiB/s"))
    for span_mb in [int(value) for value in args.spans.split(",") if value.strip()]:
        span = span_mb * MB
        rate = like_qas(url, jar, start, span, args.seconds)
        print("%-10s %12.2f" % ("%d MB" % span_mb, rate))
        start += span
        time.sleep(1)

    if not args.skip_curl:
        print("\n%-24s %10s %6s" % ("curl", "MiB/s", "http"))
        for label, version in (("--http1.1, 64MB", "--http1.1"), ("--http2, 64MB", "--http2")):
            rate, proto = with_curl(url, jar, start, 64 * MB, args.seconds, version)
            print("%-24s %10.2f %6s" % (label, rate, proto))


if __name__ == "__main__":
    main()
