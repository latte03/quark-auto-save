// Package plugins contains Go ports of the upstream quark-auto-save plugins.
package plugins

import (
	"encoding/json"
	"strings"

	"qas-poc/quark"
)

// Filter honours the upstream PLUGIN_FLAGS convention ("-alist,-aria2").
func Filter(list []quark.TaskPlugin, flags string) []quark.TaskPlugin {
	if strings.TrimSpace(flags) == "" {
		return list
	}
	disabled := map[string]bool{}
	for _, flag := range strings.Split(flags, ",") {
		disabled[strings.TrimSpace(flag)] = true
	}
	var out []quark.TaskPlugin
	for _, plugin := range list {
		if disabled["-"+plugin.Name()] {
			continue
		}
		out = append(out, plugin)
	}
	return out
}

// Build constructs every ported plugin in the upstream priority order.
// A plugin without a config section stays inactive (except auto_unarchive,
// which is always active and gated per task).
func Build(config map[string]json.RawMessage, deps Deps) []quark.TaskPlugin {
	section := func(name string) json.RawMessage {
		return config[name]
	}
	return []quark.TaskPlugin{
		NewAutoUnarchive(section("auto_unarchive"), deps),
		NewSmartStrm(section("smartstrm"), deps),
		NewAlist(section("alist"), deps),
		NewAlistStrm(section("alist_strm"), deps),
		NewAlistStrmGen(section("alist_strm_gen"), deps),
		NewAlistSync(section("alist_sync"), deps),
		NewAria2(section("aria2"), deps),
	}
}

// ConfigDefaults describes the global settings each ported plugin understands.
func ConfigDefaults() map[string]map[string]any {
	return map[string]map[string]any{
		"auto_unarchive": {"global_enable": false, "max_concurrent": 3},
		"smartstrm":      {"webhook": "", "strmtask": "", "xlist_path_fix": ""},
		"alist":          {"url": "", "token": "", "storage_id": ""},
		"alist_strm":     {"url": "", "cookie": "", "config_id": ""},
		"alist_strm_gen": {
			"url": "", "token": "", "storage_id": "",
			"strm_save_dir": "/media", "strm_replace_host": "",
		},
		"alist_sync": {
			"url": "", "token": "", "quark_storage_id": "",
			"save_storage_id": "", "tv_mode": "",
		},
		"aria2": {"host_port": "172.17.0.1:6800", "secret": "", "dir": "/Downloads"},
	}
}

// Configured reports which plugins have enough settings to activate; it does
// not contact the services.
func Configured(config map[string]json.RawMessage) map[string]bool {
	values := func(name string) map[string]any {
		out := map[string]any{}
		if raw := config[name]; len(raw) > 0 {
			_ = json.Unmarshal(raw, &out)
		}
		return out
	}
	filled := func(name string, keys ...string) bool {
		section := values(name)
		for _, key := range keys {
			if text, _ := section[key].(string); text == "" {
				return false
			}
		}
		return true
	}
	return map[string]bool{
		"auto_unarchive": true,
		"smartstrm":      filled("smartstrm", "webhook", "strmtask"),
		"alist":          filled("alist", "url", "token", "storage_id"),
		"alist_strm":     filled("alist_strm", "url", "cookie", "config_id"),
		"alist_strm_gen": filled("alist_strm_gen", "url", "token", "storage_id"),
		"alist_sync":     filled("alist_sync", "url", "token"),
		"aria2":          filled("aria2", "host_port", "secret"),
	}
}
