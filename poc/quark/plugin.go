package quark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// PluginAccount is the account surface plugins may use.
type PluginAccount interface {
	LsDir(pdirFID string) ([]ShareFile, error)
	Download(fids []string) ([]DownloadItem, string, error)
	MoveFiles(fids []string, toPdirFID string) error
	Unarchive(fid, toPdirFID string) (string, error)
	WaitTask(taskID string, timeout time.Duration) (TaskResult, error)
	Delete(fids []string) (string, error)
	Rename(fid, fileName string) error
	CookieHeader() string
	UserAgent() string
}

type PluginEnv struct {
	Account     PluginAccount
	Tree        *Tree
	SavepathFID map[string]string
	Log         func(format string, args ...any)
	HTTP        *http.Client
}

// TaskPlugin mirrors the upstream plugin lifecycle subset we support:
// every plugin validates its global config, exposes task defaults and runs
// after a task saved something new.
type TaskPlugin interface {
	Name() string
	Active() bool
	TaskDefaults() map[string]any
	Run(task *Task, env *PluginEnv) error
}

// AdditionSection returns the raw per-task config for one plugin.
func (t *Task) AdditionSection(name string) json.RawMessage {
	if len(t.Addition) == 0 {
		return nil
	}
	var sections map[string]json.RawMessage
	if err := json.Unmarshal(t.Addition, &sections); err != nil {
		return nil
	}
	return sections[name]
}

// MergePluginDefaults mirrors merge_dicts(task.addition, task_plugins_config):
// existing task values win, missing keys are filled from plugin defaults.
func (t *Task) MergePluginDefaults(plugins []TaskPlugin) error {
	defaults := map[string]any{}
	for _, plugin := range plugins {
		if values := plugin.TaskDefaults(); values != nil {
			defaults[plugin.Name()] = values
		}
	}
	if len(defaults) == 0 {
		return nil
	}
	existing := map[string]any{}
	if len(t.Addition) > 0 {
		if err := json.Unmarshal(t.Addition, &existing); err != nil {
			return fmt.Errorf("task %s addition: %w", t.TaskName, err)
		}
	}
	merged := mergeAnyMap(existing, defaults)
	encoded, err := json.Marshal(merged)
	if err != nil {
		return err
	}
	t.Addition = encoded
	return nil
}

func mergeAnyMap(existing, defaults map[string]any) map[string]any {
	out := make(map[string]any, len(existing)+len(defaults))
	for key, value := range existing {
		out[key] = value
	}
	for key, value := range defaults {
		current, present := out[key]
		if !present {
			out[key] = value
			continue
		}
		currentMap, okCurrent := current.(map[string]any)
		defaultMap, okDefault := value.(map[string]any)
		if okCurrent && okDefault {
			out[key] = mergeAnyMap(currentMap, defaultMap)
		}
	}
	return out
}

// RunTaskPlugins runs active plugins in registry order; a failing plugin is
// reported and does not abort the others.
func RunTaskPlugins(task *Task, plugins []TaskPlugin, env *PluginEnv) {
	for _, plugin := range plugins {
		if !plugin.Active() {
			continue
		}
		if err := plugin.Run(task, env); err != nil {
			env.Log("插件 %s 出错: %v", plugin.Name(), err)
		}
	}
}
