package main

import (
	"fmt"
	"os"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	switch os.Args[1] {
	case "probe":
		cmdProbe(os.Args[2:])
	case "ls":
		cmdLs(os.Args[2:])
	case "save":
		cmdSave(os.Args[2:])
	case "rename":
		cmdRename(os.Args[2:])
	case "run":
		cmdRun(os.Args[2:])
	case "sign":
		cmdSign(os.Args[2:])
	case "notify-test":
		cmdNotifyTest(os.Args[2:])
	case "serve":
		cmdServe(os.Args[2:])
	case "mcp":
		cmdMCP(os.Args[2:])
	case "dump":
		cmdDump(os.Args[2:])
	case "-h", "--help", "help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "未知命令 %q\n\n", os.Args[1])
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Print(`qaspoc — 夸克接口与转存流水线 PoC

命令:
  probe    只读：校验分享链接并列出文件
  ls       只读：列出网盘目录
  save     转存到指定目录（默认演练，加 -write 才执行）
  rename   重命名文件（必须加 -write）
  run      按配置执行任务与插件（默认演练，加 -write 才执行）
  sign     验证账号并领取每日空间
  notify-test 用 push_config 发送一条测试通知
  serve    启动 Web 控制台（REST + SSE，托管前端构建产物）
  mcp      以 MCP stdio 服务运行，供 Claude / Cursor 等客户端连接
  dump     把真实任务的分享与目标目录抓成离线夹具（只读）

示例:
  go run . ls -cookie-file ~/Downloads/quark-cookie.txt -path /
  go run . probe -cookie-file ~/Downloads/quark-cookie.txt -share "https://pan.quark.cn/s/xxxx"
  go run . run -config ../config/quark_config.json -cookie-file ~/Downloads/quark-cookie.txt
  go run . run -tasks 1,综艺 -write
  go run . run -task '{"taskname":"临时","shareurl":"https://pan.quark.cn/s/xxxx","savepath":"/video/tv/临时"}'
  go run . sign -config ../config/quark_config.json -notify=false
  go run . notify-test -config ../config/quark_config.json
  go run . serve -config ../config/quark_config.json -addr 127.0.0.1:8710
  go run . mcp -config ../config/quark_config.json -base-url http://127.0.0.1:8710
  go run . dump -tasks 1,2 -out quark/testdata/real_tasks.json

插件可用 PLUGIN_FLAGS 环境变量排除，例如 PLUGIN_FLAGS=-alist,-aria2。
只读命令不需要 -write；save/rename/run 产生的文件不会被删除。
`)
}
