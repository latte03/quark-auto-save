package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"qas-poc/notify"
	"qas-poc/plugins"
	"qas-poc/quark"
	"qas-poc/signin"
)

func cmdRun(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("run", flag.ExitOnError)
	common.bind(fs)
	tasksSpec := fs.String("tasks", "", "任务序号或名称片段，逗号分隔；空为全部")
	taskArg := fs.String("task", "", "临时任务 JSON 或文件路径；执行后不写入配置")
	write := fs.Bool("write", false, "真正执行转存与改名；缺省只演练")
	notifyPush := fs.Bool("notify", true, "按 push_config 推送签到与任务通知")
	saveConfig := fs.Bool("save-config", false, "运行后回写配置（失效标记）")
	fs.Parse(args)

	configPath := mustConfigPath(common.config)
	cfg, err := quark.LoadConfig(configPath)
	if err != nil {
		fatalf("读取配置失败: %v", err)
	}
	var tasks []*quark.Task
	inline := strings.TrimSpace(*taskArg) != ""
	if inline {
		if *tasksSpec != "" {
			fatalf("临时任务与 -tasks 不能同时指定")
		}
		task, err := readTaskArg(*taskArg)
		if err != nil {
			fatalf("%v", err)
		}
		tasks = []*quark.Task{task}
	} else {
		selected, err := selectTasks(cfg.Tasks, *tasksSpec)
		if err != nil {
			fatalf("%v", err)
		}
		tasks = selected
	}
	if len(tasks) == 0 {
		fatalf("没有选中任何任务")
	}
	client := common.client()
	runner := quark.NewRunner(client, cfg.MagicRegex)
	runner.DryRun = !*write
	runner.CreateDirs = *write
	pushCfg := notify.Decode(cfg.Raw("push_config"))

	var collected []string
	if *write && *tasksSpec == "" && !inline {
		fmt.Println("===============签到任务===============")
		clients, _ := cliClients(common, cfg)
		results := signin.Run(clients, signin.Options{
			Out:        os.Stdout,
			Collect:    func(text string) { collected = append(collected, text) },
			SignNotify: pushCfg.SignNotifyEnabled(),
		})
		signed := 0
		for _, result := range results {
			if result.Signed {
				signed++
			}
		}
		fmt.Printf("签到账号 %d 个，新签 %d 个\n\n", len(results), signed)
	}

	pluginHTTP := &http.Client{Timeout: 15 * time.Second}
	pluginLog := func(format string, args ...any) { fmt.Printf(format+"\n", args...) }
	pluginList := plugins.Filter(plugins.Build(cfg.Plugins, plugins.Deps{HTTP: pluginHTTP, Log: pluginLog}), os.Getenv("PLUGIN_FLAGS"))
	activePlugins := 0
	var activeNames []string
	for _, plugin := range pluginList {
		if plugin.Active() {
			activePlugins++
			activeNames = append(activeNames, plugin.Name())
		}
	}
	fmt.Printf("配置 %s · 任务 %d 个 · %s\n", configPath, len(tasks), modeLabel(*write))
	if len(pluginList) > 0 {
		fmt.Printf("插件：启用 %d/%d %v\n", activePlugins, len(pluginList), activeNames)
	}
	if err := runner.UpdateSavepathFID(tasks); err != nil {
		fmt.Fprintf(os.Stderr, "✗ 预解析保存目录失败: %v\n", err)
	}

	ran, updated := 0, 0
	for i, task := range tasks {
		fmt.Printf("\n#%d------------------\n", i+1)
		fmt.Printf("任务名称: %s\n分享链接: %s\n保存路径: %s\n", task.TaskName, task.ShareURL, task.SavePath)
		if task.Pattern != "" {
			fmt.Printf("正则匹配: %s\n", task.Pattern)
		}
		if task.Replace != "" {
			fmt.Printf("正则替换: %s\n", task.Replace)
		}
		if task.UpdateSubdir != "" {
			fmt.Printf("更子目录: %s\n", task.UpdateSubdir)
		}
		if task.HasRunWeek() || task.EndDate != "" {
			fmt.Printf("运行周期: WK%v ~ %s\n", task.RunWeek, task.EndDate)
		}
		if !task.IsOnce() {
			scheduled, err := task.IsScheduled(time.Now())
			if err != nil {
				fmt.Fprintf(os.Stderr, "✗ 任务周期解析失败: %v\n", err)
				continue
			}
			if !scheduled {
				fmt.Println("任务不在运行周期内，跳过")
				continue
			}
		}
		ran++
		tree, err := runner.DoSaveTask(task)
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 任务失败: %v\n", err)
			continue
		}
		if err := task.MergePluginDefaults(pluginList); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 合并插件默认配置失败: %v\n", err)
		}
		if tree != nil {
			updated++
			switch {
			case activePlugins == 0:
			case !*write:
				fmt.Println("（演练）跳过插件执行")
			default:
				quark.RunTaskPlugins(task, pluginList, &quark.PluginEnv{
					Account:     client,
					Tree:        tree,
					SavepathFID: runner.SavepathFID(),
					Log:         pluginLog,
					HTTP:        pluginHTTP,
				})
			}
		}
	}

	if *saveConfig && !inline {
		if err := cfg.Save(); err != nil {
			fmt.Fprintf(os.Stderr, "✗ 回写配置失败: %v\n", err)
		} else {
			fmt.Printf("\n已回写配置 %s\n", configPath)
		}
	} else if *saveConfig && inline {
		fmt.Println("\n临时任务不会回写配置。")
	}
	fmt.Printf("\n===============结果===============\n")
	fmt.Printf("执行 %d 个任务，%d 个有新内容；%s\n", ran, updated, modeLabel(*write))
	if *write {
		fmt.Println("PoC 不会删除任何网盘文件。")
	}

	messages := append(collected, runner.Notifications...)
	if len(messages) > 0 {
		switch {
		case !*write:
			fmt.Printf("（演练）跳过推送：%d 条通知\n", len(messages))
		case !*notifyPush:
			fmt.Printf("已关闭推送：%d 条通知\n", len(messages))
		default:
			fmt.Println("===============推送通知===============")
			results := notify.New(os.Stdout).Send(context.Background(), pushCfg, "【夸克自动转存】", strings.Join(messages, "\n"))
			if summary := notify.Summarize(results); summary != "" {
				fmt.Println(summary)
			}
		}
	}
}
