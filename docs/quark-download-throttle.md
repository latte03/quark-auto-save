# 夸克下载限速：真正起作用的是请求声明的 Range 跨度

> 结论先行：夸克 CDN 对**单个请求声明的字节区间**限速。跨度几百 MB 到 GB 级时会被打进
> 约 100 KB/s 的慢速通道；跨度 ≤ 256 MB 时按 5–9 MiB/s 正常给。跟 UA、Cookie、Referer、
> HTTP/1.1 还是 HTTP/2、开多少并发都无关。
> 2026-11-16 修复前 QAS 每段直接要 1.37 GB，16 路各 ~100 KB/s，合计 1.5 MB/s；
> 改成每次 ≤ 64 MB 后同一文件、同一账号跑到 **40 MiB/s**（26 倍）。

## 现象

《空枪》4K HDR（23.5 GB）本地下载稳定在 1.5 MiB/s 左右，16 个分段进度均匀推进。
`/vol1/@appdata/qas/qas.log` 里没有任何失败或重连，说明不是断流在拖慢，而是持续限速。
同期短小的探测请求（curl / Python）却能看到 5–10 MB/s，于是怀疑"下载指纹没做好被限速"。

## 测量环境

- NAS：fnOS，x86_64，被测文件 23,560,197,653 字节，CDN 主机 `dl-pc-zb-j.drive.quark.cn`
- 账号：配置里的主 Cookie；下载地址由 `POST /1/clouddrive/file/download` 现取
- 每次测量读固定秒数，用「读到字节数 / 墙钟时间」算速率；探测脚本只打印速率，不打印 URL 与 Cookie

## 数据一：跨度扫描（同一链接、同一 Cookie，单请求）

| 请求声明的跨度 | 速度 |
| --- | --- |
| 8 MB | 6.9 MiB/s |
| 16 MB | 8.5 MiB/s |
| 32 MB | **9.2 MiB/s** |
| 64 MB | 9.0 MiB/s |
| 128 MB | 6.9 MiB/s |
| 256 MB | 5.5 MiB/s |
| 900 MB | 0.09 MiB/s |
| 1200 MB | 0.09 MiB/s |

慢速通道落在 256 MB 与 900 MB 之间，未再细分；64 MB 既在最高速附近、又远离阈值，因此取它作上限。

## 数据二：被排除的变量（同 64 MB 跨度）

| 变量 | 结果 |
| --- | --- |
| 不带 Cookie | `412 Precondition Failed`（Cookie 是必需凭据，不是可选装饰） |
| 只带 `/file/download` 新下发的 Set-Cookie（QAS 的做法） | 5.8 MiB/s |
| 再加 `Referer: https://pan.quark.cn/` | 10.5 MiB/s |
| 换成 Chrome UA + `Accept` + `Accept-Language` + `Sec-Fetch-*` | 10.9 MiB/s |
| `curl --http1.1` / `curl --http2` | 9.29 / 8.81 MiB/s |

也就是说：QAS 现有的指纹（夸克 PC 客户端 UA + 新下发 Cookie + 精确 `Range`）本来就够用，
换成浏览器指纹没有收益，协议版本也不影响。

## 数据三：用 QAS 自己的 Go 客户端复测

为了排除"Python/curl 的行为不能代表 Go 实现"，临时加了一个带 `liveprobe` build tag 的测试，
直接调用包内的 `fetchSpan`（真实 Cookie、真实 UA、真实 URL）：

```text
span    64MB ->   6.95 MiB/s (43711031 bytes in 6.0s)
span  1200MB ->   0.09 MiB/s (588341 bytes in 6.0s)
```

结论与外部探测一致：**跨度**是唯一有效的解释变量。

## 修复

- `poc/server/segment.go`
  - 新增 `segmentRequestSpan = 64 << 20`（变量，便于测试缩小）
  - 新增 `fetchSpan`：发一次有界 `Range` 请求、按块写盘并上报进度
  - `fetchSegment` 改为按 `segmentRequestSpan` 逐块走完自己的分段，续传点仍取自 `segment.Done`
  - 分段规划、sidecar（`.part.json`）格式、进度语义都没动 → 升级可从旧进度直接续下
- `poc/server/segment_test.go`
  - `TestFetchSegmentSplitsIntoBoundedSpans`：跨度上限、请求数、内容一致
  - `TestFetchSegmentResumesInsideSpan`：从 `Done` 中途续下，首个请求起点正确

同时保留了原有保护：服务端忽略 `Range` 上界整块返回时，超出本段的内容会被裁掉
（`TestFetchSegmentClipsOverlongBody`）。

## 线上验证

2026-11-16，0.4.4 升级后对同一任务重新运行：

```text
21:40:08 ⬇ 开始下载 空枪：1 个文件 → /vol2/1000/影视热资源/电影/空枪
21:45:46 ✓ 空枪.2026.2160p.HDR10.mkv（21.9 GB）
21:45:46 ⬇ 下载结束 空枪：完成 1，跳过 0，失败 0，21.9 GB
```

- 30 秒采样：**1258 MB / 30 s ≈ 40 MiB/s（41.9 MB/s）**
- 剩余 11.4 GiB 用时 5 分 38 秒；升级前 1.5 MB/s 时同样的量要 2 小时
- 成品 23,560,197,653 字节，与云端一致；文件头 `1a 45 df a3`（Matroska）；`.part`/`.part.json` 已清理
- 16 个 worker 合计 40 MiB/s ⇒ Go 的 HTTP/2 多路复用（同主机共用一条连接）不是瓶颈

## 复现方法

只读探测即可，不需要暂停正在跑的任务（探测只读固定区间，不写盘）：

```bash
scp docs/quark_span_probe.py nas:/tmp/
ssh nas 'python3 /tmp/quark_span_probe.py --fid <fid> --spans 8,64,256,900 --seconds 4'
```

脚本从 QAS 配置读 Cookie（`--config` 可改，也可用 `QAS_COOKIE` 传），下载地址与 Cookie
都不打印，curl 复核时也只用 0600 配置文件传递。典型输出（2026-11-16，文件已下完后测）：

```text
cdn host: dl-pc-zb-j.drive.quark.cn

span              MiB/s
8 MB              10.18
64 MB             13.74
256 MB            13.74
900 MB             0.07

curl                          MiB/s   http
--http1.1, 64MB               14.29    1.1
--http2, 64MB                 13.97      2
```

对照当天有下载在跑时（带宽被占）同样是 64MB ≈ 7 MiB/s、900MB ≈ 0.09 MiB/s——量级差异稳定，
绝对值随占用波动。

要点：所有对比必须落在**同一文件、同一 Cookie、同一时段**，并且每次只改一个变量（跨度）；
只改 UA/Referer/Cookie 组合时保持跨度为 64 MB，否则会同时改两个变量、得不到结论。

## 注意事项

- 线路天花板：同一时段并发压测，这台 NAS 出口约 **46–48 MB/s** 封顶；16 个 worker 已经能打满，
  继续加段数没有收益，反而更容易触发账号侧的风控。
- 该限速属于 CDN 侧的**请求形态**判定，不是"反爬指纹"，所以别指望改 UA 能提速；
  但也别据此推断服务端其它接口的限速规则（转存、分享解析是另一套）。
- 探测会消耗真实流量与账号配额，做对比时控制单次时长（6–12 秒足够分辨 0.09 与 9 MiB/s 两个量级）。

## 相关提交

- `72cf089` perf: 分段请求按 64MB 拆分，绕开夸克的大区间限速
- `58c3885` fix: 存储空间未挂载时阻止本地下载，不再静默写进系统盘
- `d19d6f9` fix: 解压改为轮询并核对目标目录，下载队列按云端现状补齐
