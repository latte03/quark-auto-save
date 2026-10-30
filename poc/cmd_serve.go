package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"qas-poc/server"
)

func cmdServe(args []string) {
	fs := flag.NewFlagSet("serve", flag.ExitOnError)
	config := fs.String("config", "", "配置文件路径（默认 CONFIG_PATH 或 ./config/quark_config.json）")
	addr := fs.String("addr", "127.0.0.1:8710", "监听地址")
	static := fs.String("static", "", "前端构建目录（默认自动探测 web/dist）")
	data := fs.String("data", "", "运行数据目录（默认 TRIM_PKGVAR 或配置文件所在目录）")
	version := fs.String("version", "0.1.0-poc", "版本号")
	fs.Parse(args)

	configPath := mustConfigPath(*config)
	logger := log.New(os.Stdout, "[qas] ", log.LstdFlags)
	staticDir := *static
	if staticDir == "" {
		for _, candidate := range []string{"web/dist", "../web/dist"} {
			if info, err := os.Stat(filepath.Join(candidate, "index.html")); err == nil && !info.IsDir() {
				staticDir = candidate
				break
			}
		}
	}
	instance, err := server.New(server.Options{
		ConfigPath: configPath,
		DataDir:    *data,
		StaticDir:  staticDir,
		Version:    *version,
		Logger:     logger,
	})
	if err != nil {
		fatalf("启动失败: %v", err)
	}

	httpServer := &http.Server{
		Addr:              *addr,
		Handler:           instance.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = httpServer.Shutdown(shutdownCtx)
	}()

	label := staticDir
	if label == "" {
		label = "未构建（仅 API）"
	}
	logger.Printf("配置 %s · 静态资源 %s", configPath, label)
	logger.Printf("控制台地址 http://%s", *addr)
	if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		fatalf("启动失败: %v", err)
	}
}
