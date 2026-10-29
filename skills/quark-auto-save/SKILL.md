---
name: quark-auto-save
description: 通过 HTTP API 管理 QAS 控制台（夸克自动转存 / quark-auto-save）：新增与修改追更任务、判断一次性还是订阅、修复失效分享链接、立即运行并跟日志、整理网盘文件。触发词：夸克转存、夸克订阅、auto-save、pan.quark.cn 链接、QAS 任务。
metadata:
  openclaw:
    emoji: "💾"
    homepage: "https://github.com/latte03/quark-auto-save"
    requires:
      env:
        - QAS_BASE_URL
        - QAS_TOKEN
      anyBins:
        - python3
    primaryEnv: QAS_TOKEN
---

# QAS 控制台

QAS 是一个自托管的夸克网盘转存服务：一个**任务** = 一个分享链接 + 保存路径 + 命名规则 + 运行时刻，按排程把新文件转存到网盘目录，并可触发本地下载与媒体库刷新。本技能通过它的 HTTP API 操作，不碰服务器上的文件。

## 前置

| 变量 | 说明 |
| --- | --- |
| `QAS_BASE_URL` | 必填，例如 `http://192.168.1.10:8710`（fnOS 应用的固定端口） |
| `QAS_TOKEN` | 推荐。服务端 API token，`GET /api/config` 里的 `api_token` |
| `QAS_USER` / `QAS_PASSWORD` | 没有 token 时用 WebUI 账号登录（会话只在本次调用内有效） |

脚本：`python3 {baseDir}/scripts/qas_client.py <命令>`，仅用标准库，无需安装依赖。

## 命令

```bash
python3 qas_client.py status                      # 任务数 / 失效数 / 当前运行
python3 qas_client.py tasks                       # 任务列表（精简字段）
python3 qas_client.py tasks --json                # 完整字段
python3 qas_client.py task "任务名"                # 单任务完整字段
python3 qas_client.py magic                       # 魔法规则：id / 名称 / 正则
python3 qas_client.py share "<shareurl>"          # 分享内容 + 命中/改名/已存在预览
python3 qas_client.py validate "<shareurl>"       # 链接是否还能打开
python3 qas_client.py add '{"taskname": "...", ...}' [--run --write]
python3 qas_client.py update "任务名" '{"savepath": "/new/path"}'
python3 qas_client.py rm "任务名" --yes
python3 qas_client.py run "任务名" [--write] [--wait]
python3 qas_client.py run --all [--write]
python3 qas_client.py run --payload @task.json [--write --wait]   # 临时任务，不入库
python3 qas_client.py browse "/影视/剧集"
python3 qas_client.py search "关键词" [--deep]
python3 qas_client.py runs --limit 5
python3 qas_client.py log <run_id> [--tail 40]    # 已结束的运行
python3 qas_client.py watch <run_id>              # 跟随运行中的日志
python3 qas_client.py sign [--wait]               # 全部账号签到
python3 qas_client.py mkdir "/影视/新目录"
python3 qas_client.py rename "/影视/a.mkv" "b.mkv"
python3 qas_client.py rm-file "/影视/a.mkv" --yes
python3 qas_client.py config [--full]             # 配置（永不输出 cookie / token）
python3 qas_client.py config-set '{"run_logs": {"keep": 30}}'
python3 qas_client.py notify-test | downloads | session
```

JSON 参数都可写成 `@文件路径`。

## 任务字段

```json
{
  "taskname": "示例剧集",
  "shareurl": "https://pan.quark.cn/s/abc123",
  "savepath": "/影视/剧集/示例剧集",
  "magic_id": "m3f2a1b0",
  "pattern": "",
  "replace": "",
  "runweek": [1, 2, 3, 4, 5, 6, 7],
  "runtime": ["20:00", "22:30"],
  "mode": "once",
  "update_subdir": "",
  "ignore_extension": false,
  "local_download": { "enable": true, "dir": "/vol1/1000/media/tv" },
  "addition": { "emby": { "enable": true, "server": "http://emby:8096", "apikey": "..." } }
}
```

- **必填**：`taskname`、`shareurl`、`savepath`（不能是 `/`）。
- `shareurl`：`https://pan.quark.cn/s/{id}`，带子目录时用 `#/list/share/{fid}`。
- `runtime`：`HH:MM` 列表，**任务自己的运行时刻**；没有全局排程。
- `runweek`：`1`=周一 … `7`=周日；缺省视为每天。
- `mode`：留空 = 定时追更；`once` = 仅执行一次。一次性任务跑完会保留在列表里并标记完成（`once_done_at` 由服务端写，不要手工设置），「运行全部」会跳过它们；改动其内容后重新变为待执行。
- `magic_id` + `pattern`/`replace`：二选一。用规则时只填 `magic_id`，`pattern` 留空；`pattern` 里写裸正则（不是规则名）。改名规则只改 `name`，不会影响任务。
- `update_subdir`：只追某个子目录（如 `S02`）；`ignore_extension`：判断「已存在」时忽略扩展名。
- `local_download`：任务级本地下载开关、目录与 `overwrite`（同名覆盖，默认关），目录需在管理员已授权范围内。开启后每次运行做两次增量：先转存网盘里新增的文件，再扫描保存目录，把云端有、本地没有的文件补下载。一次性任务要等下载队列跑完才会标记完成。
- `addition`：插件块，键见下。

### 魔法变量（写进 `replace`）

| 变量 | 含义 |
| --- | --- |
| `{TASKNAME}` | 任务名 |
| `{I}` | 组内序号（01、02…） |
| `{EXT}` | 扩展名 |
| `{CHINESE}` | 中文片段 |
| `{DATE}` / `{YEAR}` | 日期参数 / 年份 |
| `{S}` / `{SXX}` | 季号（1 / S01） |
| `{E}` | 集号 |
| `{PART}` | 上中下 / 一二三 |
| `{VER}` | 版本（如「国语版」） |

没有匹配到变量的文件不会被转存——先用 `share` 看 `renamed` 是否符合预期，再落库。

### 插件（`addition` 的键）

`emby`、`plex`、`fnv`（飞牛影视 v1）、`fnv_refresh_v2`（v2）、`alist`、`alist_strm_gen`、`alist_strm`、`alist_sync`、`aria2`、`auto_unarchive`、`smartstrm`。各插件字段见 `tasks --json` 里的 `task_plugins_default`，不要凭空编字段。

## 工作流

### 1. 从分享链接建任务

1. `validate "<shareurl>"`：`ok=false` 就换链接。
2. `share "<shareurl>"`：看标题、目录结构、文件量；压缩包单文件时优先选它并配合 `auto_unarchive`。
3. 与已有任务对齐习惯：`tasks --json` 看同类任务的 `savepath` / `magic_id` / `runtime`，新的保存路径和命名要与之一致。
4. 需要改名时先验证：`share "<shareurl>" --task '{"taskname":"...","magic_id":"..."}'`，确认 `renamed` 列正是想要的最终文件名。
5. 落库：`add '{...}'`。订阅型再加 `--run --write` 跑首轮（不加 `--write` 只演练）。

### 2. 定时追更还是只跑一次

- 连载、会更新 → 不设 `mode`，给 `runtime`（通常与同类任务一致）。
- 完结合集、单部电影 → `"mode": "once"`，`add` 时直接 `--run --write`。
- 拿不准就问一句：完结/连载决定它会不会长期占一条排程。

### 3. 修复失效链接

1. `tasks` 里 `status` 为 `banned`（即 `shareurl_ban` 非空）的优先处理。
2. `browse "<savepath>"` 看已存文件名与最新集号。
3. `search "<任务名>"` 找候选，逐个 `share` 验证：格式要与已有文件一致（`mkv` 对 `mkv`），集号要覆盖到已有最新集之后。
4. `update "任务名" '{"shareurl":"<新链接>","pattern":"","magic_id":"..."}'`——需要保持命名一致时再调规则。
5. `run "任务名" --write --wait`，看日志确认新增集数。

### 4. 运行与排错

- 一次只能跑一轮：`status` 的 `active_run` 非空时先 `watch` 它，或等结束。
- 跟日志：`run ... --wait`；历史：`runs` → `log <id> --tail 40`。
- 转存失败先看日志里的分享链接与正则报错，再决定改链接还是改规则。

### 5. 配置

可写键：`cookie`、`crontab`、`push_config`、`plugins`、`magic_regex`、`source`、`local_download`、`run_logs`。`cookie` 一律不通过 CLI 写（脚本会拒绝），扫码登录在控制台做。

## 输出

- `OK` 或 `OK <单行 JSON>`：成功；列表类命令已做字段裁剪，`--json` 才是全量。
- `ERROR: 原因`：失败，命令以非 0 退出。
- `log` / `watch`：先一行 JSON 摘要，随后逐行日志。

## 边界

- 默认演练：不写 `--write` 不会真的转存；`rm` / `rm-file` 必须显式 `--yes`。
- 不要打印或回显 cookie、`api_token`、推送密钥；需要凭据时让用户在控制台操作。
- 不要改 `webui` 密码，也不要直接删服务器上的配置文件。
- 删除任务不会删除已转存的文件；删除网盘文件是进回收站，仍属破坏性操作，先说明再执行。
