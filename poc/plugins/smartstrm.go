package plugins

import (
	"encoding/json"
	"io"
	"net/http"

	"qas-poc/quark"
)

type smartStrmConfig struct {
	Webhook      string `json:"webhook"`
	StrmTask     string `json:"strmtask"`
	XListPathFix string `json:"xlist_path_fix"`
}

// SmartStrm triggers a SmartStrm webhook after a task saves files.
type SmartStrm struct {
	deps   Deps
	cfg    smartStrmConfig
	active bool
}

func NewSmartStrm(section json.RawMessage, deps Deps) *SmartStrm {
	plugin := &SmartStrm{deps: deps}
	if len(section) == 0 {
		return plugin
	}
	if err := decodeTaskConfig(section, &plugin.cfg); err != nil {
		deps.logf("smartstrm: 配置解析失败 %v", err)
		return plugin
	}
	if plugin.cfg.Webhook == "" || plugin.cfg.StrmTask == "" {
		return plugin
	}
	if plugin.getInfo() {
		plugin.active = true
	}
	return plugin
}

func (p *SmartStrm) Name() string                 { return "smartstrm" }
func (p *SmartStrm) Active() bool                 { return p.active }
func (p *SmartStrm) TaskDefaults() map[string]any { return nil }

func (p *SmartStrm) getInfo() bool {
	response, err := p.deps.client().Get(p.cfg.Webhook)
	if err != nil {
		p.deps.logf("SmartStrm 触发任务：连接出错 %v", err)
		return false
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		p.deps.logf("SmartStrm 触发任务：连接出错 %v", err)
		return false
	}
	var payload struct {
		Success bool   `json:"success"`
		Version string `json:"version"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		p.deps.logf("SmartStrm 触发任务：连接出错 %v", err)
		return false
	}
	if !payload.Success {
		p.deps.logf("SmartStrm 触发任务：连接失败 %s", payload.Message)
		return false
	}
	p.deps.logf("SmartStrm 触发任务: 连接成功 %s", payload.Version)
	return true
}

func (p *SmartStrm) Run(task *quark.Task, env *quark.PluginEnv) error {
	status, body, err := postJSONURL(p.deps.client(), p.cfg.Webhook, map[string]any{
		"event": "qas_strm",
		"data": map[string]any{
			"strmtask":       p.cfg.StrmTask,
			"savepath":       task.SavePath,
			"xlist_path_fix": p.cfg.XListPathFix,
		},
	})
	if err != nil {
		p.deps.logf("SmartStrm 触发任务：出错 %v", err)
		return nil
	}
	if status != http.StatusOK {
		p.deps.logf("SmartStrm 触发任务：出错 status %d", status)
		return nil
	}
	var payload struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
		Task    struct {
			Name        string `json:"name"`
			StoragePath string `json:"storage_path"`
		} `json:"task"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		p.deps.logf("SmartStrm 触发任务：出错 %v", err)
		return nil
	}
	if payload.Success {
		p.deps.logf("SmartStrm 触发任务: [%s] %s 成功✅", payload.Task.Name, payload.Task.StoragePath)
		return nil
	}
	p.deps.logf("SmartStrm 触发任务: %s", payload.Message)
	return nil
}
