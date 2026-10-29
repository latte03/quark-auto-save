package quark

import (
	"encoding/json"
	"fmt"
	"testing"
)

type stubPlugin struct {
	name     string
	active   bool
	defaults map[string]any
	ran      bool
	err      error
}

func (s *stubPlugin) Name() string                 { return s.name }
func (s *stubPlugin) Active() bool                 { return s.active }
func (s *stubPlugin) TaskDefaults() map[string]any { return s.defaults }
func (s *stubPlugin) Run(task *Task, env *PluginEnv) error {
	s.ran = true
	return s.err
}

func TestMergePluginDefaults(t *testing.T) {
	var task Task
	raw := `{"taskname":"剧集","addition":{"aria2":{"auto_download":true},"custom":{"a":1},"alist_sync":{"enable":true}}}`
	if err := task.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	plugins := []TaskPlugin{
		&stubPlugin{name: "aria2", defaults: map[string]any{
			"auto_download": false, "download_subdir": false, "save_path": "", "pause": false,
		}},
		&stubPlugin{name: "alist_sync", defaults: map[string]any{
			"enable": false, "save_path": "", "verify_path": "", "full_path_mode": false,
		}},
	}
	if err := task.MergePluginDefaults(plugins); err != nil {
		t.Fatal(err)
	}
	var addition map[string]map[string]any
	if err := json.Unmarshal(task.Addition, &addition); err != nil {
		t.Fatal(err)
	}
	if addition["aria2"]["auto_download"] != true {
		t.Errorf("existing task value was overwritten: %v", addition["aria2"])
	}
	if addition["aria2"]["pause"] != false || addition["aria2"]["download_subdir"] != false {
		t.Errorf("aria2 defaults missing: %v", addition["aria2"])
	}
	if addition["alist_sync"]["enable"] != true || addition["alist_sync"]["save_path"] != "" {
		t.Errorf("alist_sync merged = %v", addition["alist_sync"])
	}
	if addition["custom"]["a"] != float64(1) {
		t.Errorf("unknown section lost: %v", addition["custom"])
	}
	section := task.AdditionSection("alist_sync")
	if len(section) == 0 {
		t.Fatal("AdditionSection returned nothing")
	}
}

func TestRunTaskPluginsIsolatesFailures(t *testing.T) {
	var logs []string
	first := &stubPlugin{name: "first", active: true, err: fmt.Errorf("boom")}
	second := &stubPlugin{name: "second", active: true}
	inactive := &stubPlugin{name: "inactive"}
	task := &Task{}
	env := &PluginEnv{Log: func(format string, args ...any) {
		logs = append(logs, fmt.Sprintf(format, args...))
	}}
	RunTaskPlugins(task, []TaskPlugin{first, inactive, second}, env)
	if !first.ran || !second.ran || inactive.ran {
		t.Fatalf("ran flags: first=%v second=%v inactive=%v", first.ran, second.ran, inactive.ran)
	}
	if len(logs) != 1 || logs[0] != "插件 first 出错: boom" {
		t.Fatalf("logs = %v", logs)
	}
}

var _ PluginAccount = (*Client)(nil)
