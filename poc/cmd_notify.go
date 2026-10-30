package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"qas-poc/notify"
	"qas-poc/quark"
)

func cmdNotifyTest(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("notify-test", flag.ExitOnError)
	common.bind(fs)
	title := fs.String("title", "【夸克自动转存】", "通知标题")
	content := fs.String("content", "", "通知内容，缺省带时间戳")
	fs.Parse(args)

	configPath := mustConfigPath(common.config)
	cfg, err := quark.LoadConfig(configPath)
	if err != nil {
		fatalf("读取配置失败: %v", err)
	}
	pushCfg := notify.Decode(cfg.Raw("push_config"))
	channels := notify.Channels(pushCfg)
	if len(channels) == 0 {
		fatalf("未配置推送渠道")
	}
	if *content == "" {
		*content = "通知测试\n\n" + time.Now().Format("2006-01-02 15:04:05")
	}

	fmt.Printf("配置 %s · 渠道 %d 个：%s\n", configPath, len(channels), strings.Join(channels, "、"))
	results := notify.New(os.Stdout).Send(context.Background(), pushCfg, *title, *content)
	if summary := notify.Summarize(results); summary != "" {
		fmt.Println(summary)
	}
	for _, result := range results {
		if result.Err != nil {
			os.Exit(1)
		}
	}
}
