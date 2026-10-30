package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"

	"qas-poc/quark"
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

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "✗ "+format+"\n", args...)
	os.Exit(1)
}
