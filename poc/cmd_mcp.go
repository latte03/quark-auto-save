package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"qas-poc/mcp"
	"qas-poc/server"
)

func cmdMCP(args []string) {
	fs := flag.NewFlagSet("mcp", flag.ExitOnError)
	baseURL := fs.String("base-url", "", "QAS 控制台地址（默认 QAS_BASE_URL 或 http://127.0.0.1:8710）")
	token := fs.String("token", "", "API token（默认 QAS_TOKEN；缺省时用 -config 的 WebUI 账号派生）")
	config := fs.String("config", "", "配置文件路径（用于派生 token；默认 CONFIG_PATH 或 ./config/quark_config.json）")
	readOnly := fs.Bool("read-only", false, "只读模式：不暴露写工具")
	version := fs.String("version", "0.1.0-poc", "MCP server 版本")
	fs.Parse(args)

	base := strings.TrimSpace(*baseURL)
	if base == "" {
		base = strings.TrimSpace(os.Getenv("QAS_BASE_URL"))
	}
	if base == "" {
		base = "http://127.0.0.1:8710"
	}
	apiToken := strings.TrimSpace(*token)
	if apiToken == "" {
		apiToken = strings.TrimSpace(os.Getenv("QAS_TOKEN"))
	}
	if apiToken == "" {
		cfg, err := loadOptionalConfig(*config)
		if err != nil {
			fatalf("读取配置失败: %v", err)
		}
		if cfg == nil {
			fatalf("缺少 API token：设置 QAS_TOKEN，或用 -config 指定配置文件")
		}
		username, password := webuiCredentials(cfg)
		if password == "" {
			fatalf("配置里的 WebUI 密码为空，无法派生 API token")
		}
		apiToken = server.APIKey(username, password)
	}
	ro := *readOnly || envTruthy("QAS_MCP_READ_ONLY")

	logger := log.New(os.Stderr, "[qas-mcp] ", log.LstdFlags)
	logger.Printf("连接 %s · 只读=%v", base, ro)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := mcp.New(mcp.NewClient(base, apiToken), mcp.Options{
		Version:  *version,
		ReadOnly: ro,
		Logger:   logger,
	})
	if err := srv.Serve(ctx, os.Stdin, os.Stdout); err != nil {
		fatalf("MCP 退出: %v", err)
	}
}
