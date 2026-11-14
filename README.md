# QAS · 夸克自动转存

自托管的夸克网盘自动转存控制台：Go 服务端 + React 控制台，打包为飞牛 fnOS 应用（应用 ID `qas`，端口 `8710`）。受众是单个管理员，所有配置都落在一个 `config.json` 里。

## 功能

- **任务追更**：一个任务 = 一个分享链接 + 保存路径 + 魔法正则；按任务设置多个运行时刻（精确到分钟、按星期过滤），也可设为「仅执行一次」。
- **仅下载任务**：跳过分享转存，直接把网盘中已有目录的文件增量同步到 NAS（云端有、本地缺才下载），支持定时与一次性。
- **魔法正则**：稳定 `id` 的命名规则，改名不影响已引用它的任务；任务里以下拉两行展示规则名与正则。
- **插件**：Emby / Plex / 飞牛影视 v1·v2 / Alist 四个变体 / Aria2 / 自动云解压 / SmartStrm，按任务开关，跟随转存结果执行。
- **本地下载**：任务级开关、目录与同名覆盖，双增量（网盘新增 + 本地缺失补齐）、大文件分段并发（≥64MB）、Range 续传、暂停/继续/取消、实时进度与速度、并发上限、最小剩余空间；文件页有下载队列。
- **通知**：Console / Bark / Telegram / SMTP / Server 酱 / 钉钉 / 飞书等渠道，签到与任务结果可推送。
- **账号**：扫码登录直接写入主账号 Cookie；多账号时第一个账号执行转存、全部账号执行签到。
- **运行记录**：落盘日志，可配置保留条数并自动清理；支持试运行与整轮重跑。
- **MCP**：serve 进程内置 `/mcp` 端点（Streamable HTTP，协议 `2025-06-18`），外部 Agent 用控制台地址 + API token 直连，无需在本机运行任何 qas 进程。

## 目录

```
poc/            Go 服务端（module qas-poc），子命令在 main.go：
                probe / ls / save / rename / run / sign / notify-test / serve / dump
  quark         夸克网盘客户端与配置模型
  server        REST + SSE + MCP HTTP 端点 + 静态资源
  plugins       插件协议与实现
  notify        通知渠道
  signin        签到编排
  fnos/          fnOS 打包（manifest、生命周期脚本、图标、build.sh）
  tools/         Python 对照生成器（Go 测试用夹具）
web/            React 19 + Vite 8 + TypeScript + shadcn（Base UI）控制台
skills/         内置技能 quark-auto-save（Agent 通过 HTTP API 管理任务）
```

## 本地开发

依赖 Go 1.26+、Node 22+、pnpm。

```bash
# 服务端（默认读 CONFIG_PATH 或 ./config/quark_config.json）
cd poc && go run . serve -config ../config/quark_config.json -addr 127.0.0.1:8710

# 控制台（Vite 把 /api 代理到 127.0.0.1:8710）
cd web && pnpm install && pnpm dev
```

首次启动时若 `webui` 账号为空，服务端会在日志里打印随机密码；登录后可在设置里改。

## 构建与打包

```bash
cd web && pnpm build                 # 产物在 web/dist
poc/fnos/build.sh                    # 需要 fnpack，输出 dist/qas_<version>_x86.fpk
```

## 测试

```bash
cd poc && go test ./...              # 含真实任务夹具与 Python 对照
cd web && pnpm typecheck && pnpm test
```

## 外部 Agent 接入

`serve` 进程内置 MCP 端点（Streamable HTTP，协议 `2025-06-18`），外部 Agent 不需要在本机运行任何 qas 进程，填控制台地址即可直连。

1. **拿 API token**：控制台「设置 → 外部接入」复制；或用 WebUI 账号调 `POST /api/login`（响应含 `token`）；或带会话读 `GET /api/config` 的 `api_token` 字段。
2. **接入 MCP**（URL 为 `http://<host>:8710/mcp`，认证走 `Authorization: Bearer <token>`）：

   ```bash
   # Claude Code
   claude mcp add --transport http qas http://192.168.1.10:8710/mcp \
     --header "Authorization: Bearer <token>"
   ```

   其他支持 MCP HTTP 的客户端（Cursor、Cherry Studio 等）等价配置：

   ```json
   {
     "mcpServers": {
       "qas": {
         "url": "http://192.168.1.10:8710/mcp",
         "headers": { "Authorization": "Bearer <token>" }
       }
     }
   }
   ```

3. **可用工具**：状态、任务、配置（自动脱敏）、网盘浏览、分享校验与转存预览、魔法改名预览、资源搜索、运行记录与日志等待等读工具；任务增删改、立即运行（`write=true` 必须同时 `confirm=true`）、文件重命名/删除/建目录、签到、通知测试等写工具。以 `-mcp-read-only`（或环境变量 `QAS_MCP_READ_ONLY=1`）启动 serve 可隐藏全部写工具。

不走 MCP 的 Agent 可以直接调用 REST API：全部 `/api/*` 路由接受 `Authorization: Bearer <token>`（或 `?token=` 供 SSE/脚本使用），响应统一为 `{"success", "message", "data"}`；[skills/quark-auto-save](skills/quark-auto-save/SKILL.md) 内置了对应的 Agent 技能手册与纯标准库 Python CLI。

> 注意：API token 由 WebUI 账号密码派生，**修改 WebUI 密码会使 token 与全部会话失效**，Agent 需要重新配置；token 权限与 WebUI 登录完全相同，请仅在可信网络中使用。

## 配置与安全

- 配置文件默认 `config/quark_config.json`（fnOS 安装后为应用私有目录），里面含夸克 Cookie、WebUI 密码与推送渠道密钥，不要提交到仓库。
- WebUI 用 HttpOnly Cookie 会话；接口与 SSE 在会话失效时统一返回 401，控制台会退回登录页。
- 转存与重命名默认演练，只有 `-write` 才真正执行；转存产生的文件不会被自动删除。

产品背景与取舍见 [归档的产品简报](docs/archive/PRODUCT.md)。
