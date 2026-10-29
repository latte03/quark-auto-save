package plugins

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"qas-poc/quark"
)

type alistStrmConfig struct {
	URL      string `json:"url"`
	Cookie   string `json:"cookie"`
	ConfigID string `json:"config_id"`
}

// AlistStrm triggers named runs of an external alist-strm instance.
type AlistStrm struct {
	deps   Deps
	cfg    alistStrmConfig
	client *alistClient
	active bool
}

func NewAlistStrm(section json.RawMessage, deps Deps) *AlistStrm {
	plugin := &AlistStrm{deps: deps}
	if len(section) == 0 {
		return plugin
	}
	if err := decodeTaskConfig(section, &plugin.cfg); err != nil {
		deps.logf("alist-strm: 配置解析失败 %v", err)
		return plugin
	}
	if plugin.cfg.URL == "" || plugin.cfg.Cookie == "" || plugin.cfg.ConfigID == "" {
		return plugin
	}
	plugin.client = newAlistClient(plugin.cfg.URL, "", deps)
	if plugin.getInfo() {
		plugin.active = true
	}
	return plugin
}

func (p *AlistStrm) Name() string                 { return "alist_strm" }
func (p *AlistStrm) Active() bool                 { return p.active }
func (p *AlistStrm) TaskDefaults() map[string]any { return nil }

var (
	reStrmConfigName = regexp.MustCompile(`value="(\d*)">\s*<strong>名称:</strong>([^<]+)`)
	reStrmAlert      = regexp.MustCompile(`role="alert">\s*([^<]+)\s*<button`)
)

func (p *AlistStrm) headers() map[string]string {
	return map[string]string{"Cookie": p.cfg.Cookie}
}

func (p *AlistStrm) getInfo() bool {
	status, body, err := p.client.get("/configs", nil, p.headers())
	if err != nil {
		p.deps.logf("获取alist-strm配置信息出错: %v", err)
		return false
	}
	if status >= 400 {
		p.deps.logf("获取alist-strm配置信息出错: status %d", status)
		return false
	}
	matches := reStrmConfigName.FindAllStringSubmatch(string(body), -1)
	if len(matches) == 0 {
		p.deps.logf("alist-strm配置运行: 匹配失败❌，请检查网络连通和cookie有效性")
		return false
	}
	wanted := map[string]bool{}
	for _, id := range strings.Split(p.cfg.ConfigID, ",") {
		wanted[strings.TrimSpace(id)] = true
	}
	var names []string
	for _, match := range matches {
		if wanted[match[1]] {
			names = append(names, strings.TrimSpace(match[2]))
		}
	}
	p.deps.logf("alist-strm配置运行: %v", names)
	return true
}

func (p *AlistStrm) Run(task *quark.Task, env *quark.PluginEnv) error {
	p.runSelectedConfigs(p.cfg.ConfigID)
	return nil
}

func (p *AlistStrm) runSelectedConfigs(configIDs string) bool {
	var ids []string
	for _, raw := range strings.Split(configIDs, ",") {
		id := strings.TrimSpace(raw)
		if _, err := strconv.Atoi(id); err != nil {
			p.deps.logf("🔗 alist-strm配置运行: 出错❌ id应以,分割")
			return false
		}
		ids = append(ids, id)
	}
	form := url.Values{}
	for _, id := range ids {
		form.Add("selected_configs", id)
	}
	form.Add("action", "run_selected")
	status, body, err := p.client.postForm("/run_selected_configs", form, p.headers())
	if err != nil {
		p.deps.logf("Error: %v", err)
		return false
	}
	if status >= 400 {
		p.deps.logf("Error: status %d", status)
		return false
	}
	if match := reStrmAlert.FindStringSubmatch(string(body)); match != nil {
		p.deps.logf("🔗 alist-strm配置运行: %s✅", strings.TrimSpace(match[1]))
		return true
	}
	p.deps.logf("🔗 alist-strm配置运行: 失败❌")
	return false
}
