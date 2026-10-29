package plugins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"qas-poc/quark"
)

type aria2Config struct {
	HostPort string `json:"host_port"`
	Secret   string `json:"secret"`
	Dir      string `json:"dir"`
}

type aria2TaskConfig struct {
	AutoDownload   bool   `json:"auto_download"`
	DownloadSubdir bool   `json:"download_subdir"`
	SavePath       string `json:"save_path"`
	Pause          bool   `json:"pause"`
}

// Aria2 hands fresh Quark download URLs to an Aria2 JSON-RPC endpoint.
type Aria2 struct {
	deps   Deps
	cfg    aria2Config
	rpcURL string
	active bool
}

func NewAria2(section json.RawMessage, deps Deps) *Aria2 {
	plugin := &Aria2{deps: deps, cfg: aria2Config{HostPort: "172.17.0.1:6800", Dir: "/Downloads"}}
	if len(section) == 0 {
		return plugin
	}
	if err := decodeTaskConfig(section, &plugin.cfg); err != nil {
		deps.logf("Aria2下载: 配置解析失败 %v", err)
		return plugin
	}
	if plugin.cfg.HostPort == "" || plugin.cfg.Secret == "" {
		return plugin
	}
	plugin.rpcURL = rpcEndpoint(plugin.cfg.HostPort)
	if plugin.getVersion() {
		plugin.active = true
	}
	return plugin
}

func (p *Aria2) Name() string { return "aria2" }
func (p *Aria2) Active() bool { return p.active }

func (p *Aria2) TaskDefaults() map[string]any {
	return map[string]any{
		"auto_download":   false,
		"download_subdir": false,
		"save_path":       "",
		"pause":           false,
	}
}

func rpcEndpoint(endpoint string) string {
	if strings.Contains(endpoint, "://") {
		scheme, rest, _ := strings.Cut(endpoint, "://")
		hostPort, rpcPath, hasPath := strings.Cut(rest, "/")
		if !hasPath || rpcPath == "" {
			rpcPath = "jsonrpc"
		}
		return strings.ToLower(scheme) + "://" + hostPort + "/" + rpcPath
	}
	return "http://" + endpoint + "/jsonrpc"
}

func (p *Aria2) rpc(method string, params []any) (map[string]any, error) {
	if params == nil {
		params = []any{}
	}
	if p.cfg.Secret != "" {
		params = append([]any{"token:" + p.cfg.Secret}, params...)
	}
	status, body, err := postJSONURL(p.deps.client(), p.rpcURL, map[string]any{
		"jsonrpc": "2.0",
		"id":      "quark-auto-save",
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return nil, err
	}
	if status != http.StatusOK {
		return nil, fmt.Errorf("aria2 status %d", status)
	}
	var response map[string]any
	if err := json.Unmarshal(body, &response); err != nil {
		return nil, err
	}
	if rpcErr, ok := response["error"]; ok && rpcErr != nil {
		return response, fmt.Errorf("aria2 error: %v", rpcErr)
	}
	return response, nil
}

func (p *Aria2) getVersion() bool {
	response, err := p.rpc("aria2.getVersion", nil)
	if err != nil {
		p.deps.logf("Aria2下载: 连接失败%v", err)
		return false
	}
	result, _ := response["result"].(map[string]any)
	if result == nil {
		p.deps.logf("Aria2下载: 连接失败")
		return false
	}
	p.deps.logf("Aria2下载: v%v", result["version"])
	return true
}

func (p *Aria2) Run(task *quark.Task, env *quark.PluginEnv) error {
	config := aria2TaskConfig{}
	if err := decodeTaskConfig(task.AdditionSection(p.Name()), &config); err != nil {
		return fmt.Errorf("task config: %w", err)
	}
	if !config.AutoDownload {
		return nil
	}
	if env == nil || env.Tree == nil || env.Account == nil {
		return nil
	}
	nodes := env.Tree.AllNodes()
	sort.SliceStable(nodes, func(i, j int) bool {
		return nodes[i].Data.Path < nodes[j].Data.Path
	})
	var fids, paths []string
	for _, node := range nodes {
		if !node.Data.IsDir {
			if node.Data.FID == "" {
				continue
			}
			fids = append(fids, node.Data.FID)
			paths = append(paths, node.Data.Path)
			continue
		}
		if node != env.Tree.Root() && config.DownloadSubdir && node.Data.FID != "" {
			p.recursiveDir(env.Account, node.Data.FID, node.Data.Path, &fids, &paths)
		}
	}
	if len(fids) == 0 {
		p.deps.logf("Aria2下载: 没有下载任务，跳过")
		return nil
	}
	items, cookie, err := env.Account.Download(fids)
	if err != nil {
		p.deps.logf("Aria2下载: 获取下载链接失败 %v", err)
		return nil
	}
	if cookie == "" {
		cookie = env.Account.CookieHeader()
	}
	for index, item := range items {
		if index >= len(paths) || item.DownloadURL == "" {
			continue
		}
		filePath := paths[index]
		var localPath string
		if config.SavePath != "" {
			localPath = fmt.Sprintf("%s/%s/%s", p.cfg.Dir, strings.Trim(config.SavePath, "/"), baseName(filePath))
		} else {
			localPath = p.cfg.Dir + filePath
		}
		p.deps.logf("📥 Aria2下载: %s", filePath)
		_, err := p.rpc("aria2.addUri", []any{
			[]string{item.DownloadURL},
			map[string]any{
				"header": []string{
					"Cookie: " + cookie,
					"User-Agent: " + env.Account.UserAgent(),
				},
				"out":   baseName(localPath),
				"dir":   dirName(localPath),
				"pause": strconv.FormatBool(config.Pause),
			},
		})
		if err != nil {
			p.deps.logf("Aria2下载: 错误%v", err)
		}
	}
	return nil
}

func (p *Aria2) recursiveDir(account quark.PluginAccount, fid, parentPath string, fids, paths *[]string) {
	p.deps.logf("Aria2下载: 递归目录 %s", parentPath)
	items, err := account.LsDir(fid)
	if err != nil {
		p.deps.logf("Aria2下载: 读取目录失败 %v", err)
		return
	}
	for _, item := range items {
		itemPath := item.FileName
		if parentPath != "" {
			itemPath = parentPath + "/" + item.FileName
		}
		if item.Dir {
			p.recursiveDir(account, item.FID, itemPath, fids, paths)
			continue
		}
		*fids = append(*fids, item.FID)
		*paths = append(*paths, itemPath)
	}
}

func dirName(name string) string {
	if index := strings.LastIndexByte(name, '/'); index >= 0 {
		return name[:index]
	}
	return "."
}
