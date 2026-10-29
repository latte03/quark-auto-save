package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"qas-poc/mcp"
	"qas-poc/notify"
	"qas-poc/plugins"
	"qas-poc/quark"
	"qas-poc/server"
	"qas-poc/signin"
)

type commonFlags struct {
	config     string
	cookieFile string
	debug      bool
}

func (c *commonFlags) bind(fs *flag.FlagSet) {
	fs.StringVar(&c.config, "config", "", "配置文件路径（默认 CONFIG_PATH 或 ./config/quark_config.json）")
	fs.StringVar(&c.cookieFile, "cookie-file", "", "原始 Cookie 文件，优先于 -config（也可用 QUARK_COOKIE_FILE）")
	fs.BoolVar(&c.debug, "debug", false, "打印请求 URL 与响应码（敏感参数打码）")
}

func (c *commonFlags) client() *quark.Client {
	cookie, source, err := loadCookie(c.cookieFile, c.config)
	if err != nil {
		fatalf("%v", err)
	}
	client := quark.New(cookie)
	client.SetDebug(c.debug)
	fmt.Printf("凭据 %s · 移动端参数 %v\n", source, client.HasMobileParams())
	return client
}

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
		scheduled, err := task.IsScheduled(time.Now())
		if err != nil {
			fmt.Fprintf(os.Stderr, "✗ 任务周期解析失败: %v\n", err)
			continue
		}
		if !scheduled {
			fmt.Println("任务不在运行周期内，跳过")
			continue
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

// loadOptionalConfig returns nil when no config exists but a raw cookie file
// was supplied, which keeps `sign` usable in cookie-only deployments.
func loadOptionalConfig(explicit string) (*quark.Config, error) {
	path, err := findConfig(explicit)
	if err != nil {
		if explicit != "" || os.Getenv("CONFIG_PATH") != "" {
			return nil, err
		}
		return nil, nil
	}
	return quark.LoadConfig(path)
}

func cliClients(common commonFlags, cfg *quark.Config) ([]*quark.Client, string) {
	file := common.cookieFile
	if file == "" {
		file = os.Getenv("QUARK_COOKIE_FILE")
	}
	if file != "" {
		raw, err := os.ReadFile(file)
		if err != nil {
			fatalf("读取 Cookie 文件失败: %v", err)
		}
		cookie, err := validateCookie(string(raw))
		if err != nil {
			fatalf("%v", err)
		}
		return []*quark.Client{quark.New(cookie)}, file
	}
	if cfg == nil {
		return nil, ""
	}
	var clients []*quark.Client
	for _, cookie := range cfg.Cookies() {
		clients = append(clients, quark.New(cookie))
	}
	source := cfg.Path
	if source == "" {
		source = "配置"
	}
	return clients, source
}

func modeLabel(write bool) string {
	if write {
		return "执行模式"
	}
	return "演练模式（加 -write 才执行）"
}

func selectTasks(tasks []*quark.Task, spec string) ([]*quark.Task, error) {
	spec = strings.TrimSpace(spec)
	if spec == "" {
		return tasks, nil
	}
	seen := map[int]bool{}
	var out []*quark.Task
	for _, token := range strings.Split(spec, ",") {
		token = strings.TrimSpace(token)
		if token == "" {
			continue
		}
		if index, err := strconv.Atoi(token); err == nil {
			if index < 1 || index > len(tasks) {
				return nil, fmt.Errorf("任务序号 %d 超出范围 1..%d", index, len(tasks))
			}
			if !seen[index-1] {
				seen[index-1] = true
				out = append(out, tasks[index-1])
			}
			continue
		}
		found := false
		for i, task := range tasks {
			if strings.Contains(task.TaskName, token) {
				found = true
				if !seen[i] {
					seen[i] = true
					out = append(out, task)
				}
			}
		}
		if !found {
			return nil, fmt.Errorf("没有名称包含 %q 的任务", token)
		}
	}
	return out, nil
}

// readTaskArg accepts inline JSON or a file path for one-off runs.
func readTaskArg(arg string) (*quark.Task, error) {
	arg = strings.TrimSpace(arg)
	data := []byte(arg)
	if !strings.HasPrefix(arg, "{") {
		raw, err := os.ReadFile(arg)
		if err != nil {
			return nil, fmt.Errorf("读取任务文件失败: %w", err)
		}
		data = raw
	}
	var task quark.Task
	if err := json.Unmarshal(data, &task); err != nil {
		return nil, fmt.Errorf("任务格式错误: %w", err)
	}
	task.RemoveFields("index", "status", "latest", "next_run", "progress")
	task.SavePath = quark.NormalizePath(task.SavePath)
	if strings.TrimSpace(task.TaskName) == "" || task.ShareURL == "" || task.SavePath == "" || task.SavePath == "/" {
		return nil, errors.New("临时任务需要 taskname、shareurl、savepath")
	}
	if _, err := quark.ExtractShareURL(task.ShareURL); err != nil {
		return nil, errors.New("分享链接格式无效")
	}
	return &task, nil
}

func mustConfigPath(explicit string) string {
	path, err := findConfig(explicit)
	if err != nil {
		fatalf("%v", err)
	}
	return path
}

func findConfig(explicit string) (string, error) {
	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates,
		"./config/quark_config.json",
		"../config/quark_config.json",
		"./quark_config.json",
		"../quark_config.json",
	)
	for _, path := range candidates {
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			return path, nil
		}
	}
	return "", errors.New("找不到配置文件；用 -config 指定，或设置 CONFIG_PATH")
}

type dumpTask struct {
	TaskName               string `json:"taskname"`
	ShareURL               string `json:"shareurl"`
	SavePath               string `json:"savepath"`
	Pattern                string `json:"pattern,omitempty"`
	Replace                string `json:"replace,omitempty"`
	UpdateSubdir           string `json:"update_subdir,omitempty"`
	UpdateSubdirResaveMode bool   `json:"update_subdir_resave_mode,omitempty"`
	IgnoreExtension        bool   `json:"ignore_extension,omitempty"`
	StartFID               string `json:"startfid,omitempty"`
	EndDate                string `json:"enddate,omitempty"`
	RunWeek                []int  `json:"runweek,omitempty"`
}

type dumpFixture struct {
	Notes      string                       `json:"notes"`
	CapturedAt string                       `json:"captured_at"`
	MagicRegex []quark.Rule                 `json:"magic_regex"`
	Tasks      []dumpTask                   `json:"tasks"`
	Share      map[string][]quark.ShareFile `json:"share"`
	Dest       map[string][]quark.ShareFile `json:"dest"`
}

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

func webuiCredentials(cfg *quark.Config) (string, string) {
	var webui struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if cfg != nil {
		if raw := cfg.Raw("webui"); len(raw) > 0 {
			_ = json.Unmarshal(raw, &webui)
		}
	}
	if webui.Username == "" {
		webui.Username = os.Getenv("WEBUI_USERNAME")
	}
	if webui.Username == "" {
		webui.Username = "admin"
	}
	if webui.Password == "" {
		webui.Password = os.Getenv("WEBUI_PASSWORD")
	}
	return webui.Username, webui.Password
}

func envTruthy(key string) bool {
	switch strings.ToLower(strings.TrimSpace(os.Getenv(key))) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func cmdDump(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	common.bind(fs)
	tasksSpec := fs.String("tasks", "", "任务序号或名称片段，逗号分隔；空为全部")
	out := fs.String("out", "quark/testdata/real_tasks.json", "夹具输出路径")
	depth := fs.Int("depth", 3, "分享目录递归深度")
	fs.Parse(args)

	configPath := mustConfigPath(common.config)
	cfg, err := quark.LoadConfig(configPath)
	if err != nil {
		fatalf("读取配置失败: %v", err)
	}
	tasks, err := selectTasks(cfg.Tasks, *tasksSpec)
	if err != nil {
		fatalf("%v", err)
	}
	client := common.client()

	fixture := dumpFixture{
		Notes:      "生成方式：qaspoc dump（真实分享与目标目录快照，不含 Cookie）",
		CapturedAt: time.Now().Format(time.RFC3339),
		MagicRegex: cfg.MagicRegex,
		Share:      map[string][]quark.ShareFile{},
		Dest:       map[string][]quark.ShareFile{},
	}
	destPaths := map[string]bool{}

	for _, task := range tasks {
		ref, err := quark.ExtractShareURL(task.ShareURL)
		if err != nil {
			fatalf("解析分享链接失败: %v", err)
		}
		stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
		if err != nil {
			fatalf("获取 stoken 失败（%s）: %v", task.TaskName, err)
		}
		chains, err := walkShare(client, ref.PwdID, stoken.Stoken, ref.PdirFID, *depth, fixture.Share)
		if err != nil {
			fatalf("读取分享失败（%s）: %v", task.TaskName, err)
		}
		base := quark.NormalizePath(task.SavePath)
		destPaths[base] = true
		for _, chain := range chains {
			destPaths[base+"/"+chain] = true
		}
		fixture.Tasks = append(fixture.Tasks, dumpTask{
			TaskName:               task.TaskName,
			ShareURL:               task.ShareURL,
			SavePath:               task.SavePath,
			Pattern:                task.Pattern,
			Replace:                task.Replace,
			UpdateSubdir:           task.UpdateSubdir,
			UpdateSubdirResaveMode: task.UpdateSubdirResaveMode,
			IgnoreExtension:        task.IgnoreExtension,
			StartFID:               task.StartFID,
			EndDate:                task.EndDate,
			RunWeek:                task.RunWeek,
		})
	}

	for path := range destPaths {
		fids, err := client.GetFids([]string{path})
		if err != nil {
			fatalf("解析目标目录失败（%s）: %v", path, err)
		}
		if len(fids) == 0 {
			fixture.Dest[path] = []quark.ShareFile{}
			continue
		}
		list, err := client.LsDir(fids[0].FID)
		if err != nil {
			fatalf("读取目标目录失败（%s）: %v", path, err)
		}
		if list == nil {
			list = []quark.ShareFile{}
		}
		fixture.Dest[path] = list
	}

	for _, files := range fixture.Share {
		for i := range files {
			files[i].ShareFIDToken = ""
		}
	}
	for _, files := range fixture.Dest {
		for i := range files {
			files[i].ShareFIDToken = ""
		}
	}
	encoded, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		fatalf("编码夹具失败: %v", err)
	}
	if dir := filepath.Dir(*out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("创建目录失败: %v", err)
		}
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fatalf("写入夹具失败: %v", err)
	}
	fmt.Printf("已写入 %s：任务 %d 个，分享目录 %d 个，目标目录 %d 个\n",
		*out, len(fixture.Tasks), len(fixture.Share), len(fixture.Dest))
}

// walkShare records every share directory (up to depth) and returns the
// relative name chains of its subdirectories, which the pipeline may use as
// savepath suffixes.
func walkShare(client *quark.Client, pwdID, stoken, pdirFID string, depth int, out map[string][]quark.ShareFile) ([]string, error) {
	detail, err := client.GetDetail(pwdID, stoken, pdirFID, 0, 0)
	if err != nil {
		return nil, err
	}
	if detail.List == nil {
		detail.List = []quark.ShareFile{}
	}
	out[pdirFID] = detail.List
	var chains []string
	if depth <= 0 {
		return chains, nil
	}
	for _, file := range detail.List {
		if !file.Dir {
			continue
		}
		nested, err := walkShare(client, pwdID, stoken, file.FID, depth-1, out)
		if err != nil {
			return nil, err
		}
		chains = append(chains, file.FileName)
		for _, chain := range nested {
			chains = append(chains, file.FileName+"/"+chain)
		}
	}
	return chains, nil
}

func cmdProbe(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("probe", flag.ExitOnError)
	common.bind(fs)
	share := fs.String("share", "", "分享链接")
	pdir := fs.String("pdir", "0", "分享内子目录 fid，默认根目录")
	fs.Parse(args)
	if *share == "" {
		fatalf("probe 需要 -share")
	}

	client := common.client()
	ref := mustShareRef(*share)
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		fatalf("获取 stoken 失败: %v", err)
	}
	fmt.Printf("分享 ID %s · passcode %q\n", ref.PwdID, ref.Passcode)
	fmt.Printf("stoken %s\n", mask(stoken.Stoken))

	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, *pdir, 1, 1)
	if err != nil {
		fatalf("读取分享详情失败: %v", err)
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	fmt.Printf("分享标题 %q · %d 项\n", title, len(detail.List))
	printFiles(detail.List, 10)
}

func cmdLs(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("ls", flag.ExitOnError)
	common.bind(fs)
	path := fs.String("path", "/", "目录路径")
	fid := fs.String("fid", "", "直接指定目录 fid（优先于 -path）")
	fs.Parse(args)

	client := common.client()
	target := *fid
	if target == "" {
		if *path == "" || *path == "/" {
			target = "0"
		} else {
			fids, err := client.GetFids([]string{*path})
			if err != nil {
				fatalf("解析目录 fid 失败: %v", err)
			}
			if len(fids) == 0 {
				fatalf("目录不存在: %s", *path)
			}
			target = fids[0].FID
		}
	}
	list, err := client.LsDir(target)
	if err != nil {
		fatalf("列目录失败: %v", err)
	}
	fmt.Printf("目录 %s（fid %s）· %d 项\n", *path, target, len(list))
	printFiles(list, len(list))
}

func cmdSave(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("save", flag.ExitOnError)
	common.bind(fs)
	share := fs.String("share", "", "分享链接")
	dest := fs.String("dest", "", "保存目录，例如 /poc-verify")
	limit := fs.Int("limit", 1, "最多转存几个顶层条目")
	write := fs.Bool("write", false, "真正执行转存；缺省只演练")
	renameTest := fs.Bool("rename-test", true, "转存后改名并改回，验证 rename 接口")
	fs.Parse(args)
	if *share == "" || *dest == "" {
		fatalf("save 需要 -share 与 -dest")
	}
	if *limit < 1 {
		fatalf("-limit 至少为 1")
	}

	client := common.client()
	ref := mustShareRef(*share)
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		fatalf("获取 stoken 失败: %v", err)
	}
	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, "0", 0, 0)
	if err != nil {
		fatalf("读取分享失败: %v", err)
	}
	picked := pickFiles(detail.List, *limit)
	if len(picked) == 0 {
		fatalf("分享为空，没有可转存的内容")
	}

	destFID := ""
	if fids, err := client.GetFids([]string{*dest}); err == nil && len(fids) > 0 {
		destFID = fids[0].FID
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	fmt.Printf("分享 %q · 选中 %d 项 · 目标 %s", title, len(picked), *dest)
	if destFID == "" {
		fmt.Print("（目录不存在）")
	} else {
		fmt.Printf("（fid %s）", destFID)
	}
	fmt.Println()
	for _, f := range picked {
		kind, meta := "文件", humanSize(f.Size)
		if f.Dir {
			kind = "目录"
			if f.IncludeItems > 0 {
				meta = fmt.Sprintf("%d 项", f.IncludeItems)
			}
		}
		fmt.Printf("  [%s] %s · %s\n", kind, f.FileName, meta)
	}
	if !*write {
		fmt.Println("演练结束：加 -write 才会真正转存。")
		return
	}
	if destFID == "" {
		destFID, err = client.Mkdir(*dest)
		if err != nil {
			fatalf("创建目录失败: %v", err)
		}
		fmt.Printf("已创建目录 %s（fid %s）\n", *dest, destFID)
	}

	fids := make([]string, 0, len(picked))
	tokens := make([]string, 0, len(picked))
	names := make([]string, 0, len(picked))
	for _, f := range picked {
		fids = append(fids, f.FID)
		tokens = append(tokens, f.ShareFIDToken)
		names = append(names, f.FileName)
	}
	taskID, err := client.SaveFile(fids, tokens, destFID, ref.PwdID, stoken.Stoken)
	if err != nil {
		fatalf("提交转存失败: %v", err)
	}
	fmt.Printf("转存任务 %s，等待完成...\n", taskID)
	topFIDs, err := client.QueryTask(taskID)
	if err != nil {
		fatalf("等待转存完成失败: %v", err)
	}
	fmt.Printf("转存完成，顶层 fid %v\n", topFIDs)

	saved, err := client.LsDir(destFID)
	if err != nil {
		fatalf("读取目标目录失败: %v", err)
	}
	byFID := map[string]quark.ShareFile{}
	for _, f := range saved {
		byFID[f.FID] = f
	}
	if len(topFIDs) == 0 {
		for _, f := range saved {
			for _, name := range names {
				if f.FileName == name {
					topFIDs = append(topFIDs, f.FID)
				}
			}
		}
	}
	for _, fid := range topFIDs {
		name := byFID[fid].FileName
		if name == "" {
			name = "(未在目录列表中匹配到名字)"
		}
		fmt.Printf("  ✓ %s · %s\n", fid, name)
	}

	if *renameTest && len(topFIDs) > 0 {
		renameRoundTrip(client, topFIDs[0], byFID[topFIDs[0]].FileName, destFID)
	}
	fmt.Printf("测试文件保留在 %s，PoC 不会删除。\n", *dest)
}

func renameRoundTrip(client *quark.Client, fid, name, destFID string) {
	if name == "" || strings.HasPrefix(name, "poc-") {
		fmt.Println("rename 验证跳过：没有可用的文件名")
		return
	}
	renamed := "poc-" + name
	if err := client.Rename(fid, renamed); err != nil {
		fmt.Printf("  ✗ rename 失败: %v\n", err)
		return
	}
	if !nameExists(client, destFID, renamed) {
		fmt.Printf("  ✗ rename 返回成功，但目录中没找到 %q（可能仍在处理）\n", renamed)
	} else {
		fmt.Printf("  ✓ rename 成功：%s -> %s\n", name, renamed)
	}
	if err := client.Rename(fid, name); err != nil {
		fmt.Printf("  ✗ 改回原名失败，请手动改回：fid %s -> %q\n", fid, name)
		return
	}
	if nameExists(client, destFID, name) {
		fmt.Printf("  ✓ 已改回原名：%s\n", name)
	}
}

func nameExists(client *quark.Client, destFID, name string) bool {
	list, err := client.LsDir(destFID)
	if err != nil {
		return false
	}
	for _, f := range list {
		if f.FileName == name {
			return true
		}
	}
	return false
}

func cmdRename(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("rename", flag.ExitOnError)
	common.bind(fs)
	fid := fs.String("fid", "", "文件 fid")
	name := fs.String("name", "", "新文件名")
	write := fs.Bool("write", false, "真正执行重命名")
	fs.Parse(args)
	if *fid == "" || *name == "" {
		fatalf("rename 需要 -fid 与 -name")
	}
	if !*write {
		fmt.Printf("演练：将把 %s 重命名为 %q；加 -write 才执行。\n", *fid, *name)
		return
	}
	client := common.client()
	if err := client.Rename(*fid, *name); err != nil {
		fatalf("rename 失败: %v", err)
	}
	fmt.Printf("✓ %s -> %s\n", *fid, *name)
}

func pickFiles(list []quark.ShareFile, limit int) []quark.ShareFile {
	var picked []quark.ShareFile
	for _, f := range list {
		if !f.Dir {
			picked = append(picked, f)
			if len(picked) >= limit {
				return picked
			}
		}
	}
	for _, f := range list {
		if f.Dir {
			picked = append(picked, f)
			if len(picked) >= limit {
				break
			}
		}
	}
	return picked
}

func printFiles(list []quark.ShareFile, limit int) {
	if len(list) == 0 {
		fmt.Println("  (空目录)")
		return
	}
	for i, f := range list {
		if limit > 0 && i >= limit {
			fmt.Printf("  ... 还有 %d 项\n", len(list)-limit)
			return
		}
		kind := "文件"
		meta := humanSize(f.Size)
		if f.Dir {
			kind = "目录"
			meta = fmt.Sprintf("%d 项", f.IncludeItems)
		}
		fmt.Printf("  [%s] %s · %s\n", kind, f.FileName, meta)
	}
}

func humanSize(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for n/div >= unit && exp < 5 {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(n)/float64(div), "KMGTPE"[exp])
}

func mustShareRef(raw string) quark.ShareRef {
	ref, err := quark.ExtractShareURL(raw)
	if err != nil {
		fatalf("%v", err)
	}
	return ref
}

func mask(s string) string {
	if len(s) <= 12 {
		return "***"
	}
	return s[:6] + "..." + s[len(s)-4:]
}

func loadCookie(cookieFile, explicit string) (string, string, error) {
	if cookieFile == "" {
		cookieFile = os.Getenv("QUARK_COOKIE_FILE")
	}
	if cookieFile != "" {
		raw, err := os.ReadFile(cookieFile)
		if err != nil {
			return "", "", fmt.Errorf("读取 Cookie 文件失败: %w", err)
		}
		cookie, err := validateCookie(string(raw))
		return cookie, cookieFile, err
	}

	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates,
		"./config/quark_config.json",
		"../config/quark_config.json",
		"./quark_config.json",
		"../quark_config.json",
	)
	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		cookie, err := readCookie(path)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", path, err)
			continue
		}
		return cookie, path, nil
	}
	if lastErr != nil {
		return "", "", lastErr
	}
	return "", "", errors.New("找不到配置文件；用 -config 指定，或设置 CONFIG_PATH")
}

func readCookie(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		Cookie json.RawMessage `json:"cookie"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("解析 JSON 失败: %w", err)
	}
	var list []string
	if err := json.Unmarshal(doc.Cookie, &list); err == nil {
		if len(list) > 0 {
			return validateCookie(list[0])
		}
	}
	var single string
	if err := json.Unmarshal(doc.Cookie, &single); err == nil {
		return validateCookie(single)
	}
	return "", errors.New("没有 cookie 字段")
}

func validateCookie(cookie string) (string, error) {
	cookie = strings.TrimSpace(cookie)
	if strings.Contains(cookie, "Your pan.quark.cn") ||
		(len(cookie) < 100 &&
			!strings.Contains(cookie, "__pus") &&
			!strings.Contains(cookie, "__kp") &&
			!strings.Contains(cookie, "kps=")) {
		return "", errors.New("cookie 是占位值（示例配置只有 59 字符），请换成真实 Cookie")
	}
	return cookie, nil
}

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}
