package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"qas-poc/notify"
	"qas-poc/signin"
)

func cmdSign(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("sign", flag.ExitOnError)
	common.bind(fs)
	notifyPush := fs.Bool("notify", true, "推送签到结果")
	fs.Parse(args)

	cfg, err := loadOptionalConfig(common.config)
	if err != nil {
		fatalf("读取配置失败: %v", err)
	}
	clients, source := cliClients(common, cfg)
	if len(clients) == 0 {
		fatalf("未配置 Cookie")
	}
	for _, client := range clients {
		client.SetDebug(common.debug)
	}
	pushCfg := notify.Config{}
	if cfg != nil {
		pushCfg = notify.Decode(cfg.Raw("push_config"))
	}

	fmt.Printf("凭据 %s · 账号 %d 个\n", source, len(clients))
	var collected []string
	results := signin.Run(clients, signin.Options{
		Out:        os.Stdout,
		Collect:    func(text string) { collected = append(collected, text) },
		SignNotify: pushCfg.SignNotifyEnabled(),
	})
	signed, failed := 0, 0
	for _, result := range results {
		if result.Signed {
			signed++
		}
		if result.Err != "" {
			failed++
		}
	}
	fmt.Printf("\n===============结果===============\n")
	fmt.Printf("签到账号 %d 个，新签 %d 个，异常 %d 个\n", len(results), signed, failed)

	if *notifyPush && len(collected) > 0 {
		fmt.Println("===============推送通知===============")
		sent := notify.New(os.Stdout).Send(context.Background(), pushCfg, "【夸克自动转存】", strings.Join(collected, "\n"))
		if summary := notify.Summarize(sent); summary != "" {
			fmt.Println(summary)
		}
	}
}
