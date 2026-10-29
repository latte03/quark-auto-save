package plugins

import (
	"encoding/json"
	"net/url"
	"strings"

	"qas-poc/quark"
)

type alistConfig struct {
	URL       string `json:"url"`
	Token     string `json:"token"`
	StorageID string `json:"storage_id"`
}

// Alist refreshes the mounted directory that corresponds to the task savepath.
type Alist struct {
	deps      Deps
	cfg       alistConfig
	client    *alistClient
	active    bool
	mountPath string
	quarkRoot string
}

func NewAlist(section json.RawMessage, deps Deps) *Alist {
	plugin := &Alist{deps: deps}
	if len(section) == 0 {
		return plugin
	}
	if err := decodeTaskConfig(section, &plugin.cfg); err != nil {
		deps.logf("Alist刷新: 配置解析失败 %v", err)
		return plugin
	}
	if plugin.cfg.URL == "" || plugin.cfg.Token == "" {
		return plugin
	}
	plugin.client = newAlistClient(plugin.cfg.URL, plugin.cfg.Token, deps)
	if !plugin.getInfo() {
		return plugin
	}
	mount, root, ok := plugin.client.storageMount(plugin.cfg.StorageID, "Alist刷新")
	if !ok {
		return plugin
	}
	plugin.mountPath, plugin.quarkRoot = mount, root
	plugin.active = true
	return plugin
}

func (p *Alist) Name() string                 { return "alist" }
func (p *Alist) Active() bool                 { return p.active }
func (p *Alist) TaskDefaults() map[string]any { return nil }

func (p *Alist) getInfo() bool {
	status, body, err := p.client.get("/api/admin/setting/list", url.Values{"group": {"1"}}, p.client.headers())
	if err != nil {
		p.deps.logf("获取Alist信息出错: %v", err)
		return false
	}
	if status != 200 {
		p.deps.logf("Alist刷新: 连接失败❌ status %d", status)
		return false
	}
	response, err := decodeAlist(body)
	if err != nil {
		p.deps.logf("获取Alist信息出错: %v", err)
		return false
	}
	if response.Code != 200 {
		p.deps.logf("Alist刷新: 连接失败❌ %s", response.Message)
		return false
	}
	var settings []struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(response.Data, &settings); err != nil {
		p.deps.logf("获取Alist信息出错: %v", err)
		return false
	}
	second, first := "", ""
	if len(settings) > 1 {
		second = strings.TrimSpace(settings[1].Value)
	}
	if len(settings) > 0 {
		first = strings.TrimSpace(settings[0].Value)
	}
	p.deps.logf("Alist刷新: %s %s", second, first)
	return true
}

func (p *Alist) Run(task *quark.Task, env *quark.PluginEnv) error {
	if task.SavePath == "" || p.quarkRoot == "" || !strings.HasPrefix(task.SavePath, p.quarkRoot) {
		return nil
	}
	relative := strings.Replace(task.SavePath, p.quarkRoot, "", 1)
	alistPath := joinAlistPath(p.mountPath, relative)
	p.refresh(alistPath)
	return nil
}

func (p *Alist) refresh(alistPath string) bool {
	files, response, err := p.client.fsList(alistPath, true)
	_ = files
	if err == nil && response.Code == 200 {
		p.deps.logf("📁 Alist刷新：目录[%s] 成功✅", alistPath)
		return true
	}
	message := response.Message
	if err != nil {
		message = err.Error()
	}
	if response.Code != 200 && strings.Contains(strings.ToLower(message), "object not found") {
		if alistPath == "/" || alistPath == p.mountPath {
			p.deps.logf("📁 Alist刷新：根目录不存在，请检查 Alist 配置")
			return false
		}
		parent := alistPath
		if index := strings.LastIndexByte(parent, '/'); index > 0 {
			parent = parent[:index]
		} else {
			parent = "/"
		}
		p.deps.logf("📁 Alist刷新：[%s] 不存在，转父目录 [%s]", alistPath, parent)
		return p.refresh(parent)
	}
	p.deps.logf("📁 Alist刷新：失败❌ %s", message)
	return false
}
