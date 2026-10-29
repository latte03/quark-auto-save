# QAS · 夸克自动转存

自托管的夸克网盘自动转存控制台：Go 服务端 + React 控制台，打包为飞牛 fnOS 应用（应用 ID `qas`，端口 `8710`）。受众是单个管理员，所有配置都落在一个 `config.json` 里。

## 功能

- **任务追更**：一个任务 = 一个分享链接 + 保存路径 + 魔法正则；按任务设置多个运行时刻（精确到分钟、按星期过滤），也可设为「仅执行一次」。
- **魔法正则**：稳定 `id` 的命名规则，改名不影响已引用它的任务；任务里以下拉两行展示规则名与正则。
- **插件**：Emby / Plex / 飞牛影视 v1·v2 / Alist 四个变体 / Aria2 / 自动云解压 / SmartStrm，按任务开关，跟随转存结果执行。
- **本地下载**：任务级开关与目录，Range 续传、并发上限、最小剩余空间、同名覆盖；文件页有下载队列。
- **通知**：Console / Bark / Telegram / SMTP / Server 酱 / 钉钉 / 飞书等渠道，签到与任务结果可推送。
- **账号**：扫码登录直接写入主账号 Cookie；多账号时第一个账号执行转存、全部账号执行签到。
- **运行记录**：落盘日志，可配置保留条数并自动清理；支持试运行与整轮重跑。
- **MCP**：`qas mcp` 以 stdio 方式提供工具，供 Claude / Cursor 等客户端调用。

## 目录

```
poc/            Go 服务端（module qas-poc），子命令在 main.go：
                probe / ls / save / rename / run / sign / notify-test / serve / mcp / dump
  quark         夸克网盘客户端与配置模型
  server        REST + SSE + 静态资源
  plugins       插件协议与实现
  notify        通知渠道
  signin        签到编排
  fnos/          fnOS 打包（manifest、生命周期脚本、图标、build.sh）
  tools/         Python 对照生成器（Go 测试用夹具）
web/            React 19 + Vite 8 + TypeScript + shadcn（Base UI）控制台
skills/         内置技能（供 Agent 调用本项目的 CLI）
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

## 配置与安全

- 配置文件默认 `config/quark_config.json`（fnOS 安装后为应用私有目录），里面含夸克 Cookie、WebUI 密码与推送渠道密钥，不要提交到仓库。
- WebUI 用 HttpOnly Cookie 会话；接口与 SSE 在会话失效时统一返回 401，控制台会退回登录页。
- 转存与重命名默认演练，只有 `-write` 才真正执行；转存产生的文件不会被自动删除。

产品背景与取舍见 [归档的产品简报](docs/archive/PRODUCT.md)。
