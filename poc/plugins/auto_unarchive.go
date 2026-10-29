package plugins

import (
	"encoding/json"
	"fmt"
	"regexp"
	"time"

	"qas-poc/quark"
)

type autoUnarchiveConfig struct {
	GlobalEnable  any `json:"global_enable"`
	MaxConcurrent int `json:"max_concurrent"`
}

type autoUnarchiveTaskConfig struct {
	Enable          bool `json:"enable"`
	AutoClean       bool `json:"auto_clean"`
	AutoCleanZipdir bool `json:"auto_clean_zipdir"`
}

// AutoUnarchive expands archives in the cloud and moves the files back.
type AutoUnarchive struct {
	deps          Deps
	cfg           autoUnarchiveConfig
	globalEnabled bool
}

func NewAutoUnarchive(section json.RawMessage, deps Deps) *AutoUnarchive {
	plugin := &AutoUnarchive{deps: deps, cfg: autoUnarchiveConfig{MaxConcurrent: 3}}
	_ = decodeTaskConfig(section, &plugin.cfg)
	plugin.globalEnabled = looseBool(plugin.cfg.GlobalEnable)
	return plugin
}

func (p *AutoUnarchive) Name() string { return "auto_unarchive" }
func (p *AutoUnarchive) Active() bool { return true }

func (p *AutoUnarchive) TaskDefaults() map[string]any {
	return map[string]any{
		"enable":            false,
		"auto_clean":        true,
		"auto_clean_zipdir": false,
	}
}

var reArchiveExt = regexp.MustCompile(`(?i)\.(zip|rar|7z)$`)

func (p *AutoUnarchive) Run(task *quark.Task, env *quark.PluginEnv) error {
	config := autoUnarchiveTaskConfig{AutoClean: true}
	if err := decodeTaskConfig(task.AdditionSection(p.Name()), &config); err != nil {
		return fmt.Errorf("task config: %w", err)
	}
	if !p.globalEnabled && !config.Enable {
		return nil
	}
	if env == nil || env.Tree == nil || env.Account == nil {
		return nil
	}
	savepath := quark.NormalizePath(task.SavePath)
	target := env.SavepathFID[savepath]
	if target == "" {
		return nil
	}
	var archives []*quark.TreeNode
	for _, node := range env.Tree.AllNodes() {
		if node.Data.IsDir || !reArchiveExt.MatchString(node.Name) {
			continue
		}
		archives = append(archives, node)
	}
	if len(archives) == 0 {
		return nil
	}
	p.deps.logf("📦 [%s] 共有 %d 个压缩包，逐个解压", task.TaskName, len(archives))

	var moveFIDs, cleanFIDs []string
	for _, node := range archives {
		zipFID := node.Data.FID
		zipName := node.Data.FileNameRe
		if zipName == "" {
			zipName = node.Data.FileName
		}
		mainName, _ := splitExt(zipName)
		taskID, err := env.Account.Unarchive(zipFID, target)
		if err != nil {
			p.deps.logf("  ❌ 提交失败: %s (%v)", zipName, err)
			continue
		}
		p.deps.logf("  ▶️ 提交解压: %s", zipName)
		result, err := env.Account.WaitTask(taskID, 30*time.Minute)
		if err != nil {
			p.deps.logf("  ⚠️ 任务异常: %s %v", zipName, err)
			continue
		}
		p.deps.logf("  ✅ 解压完成: %s", zipName)
		subdirFID := ""
		for _, item := range result.UnarchiveList {
			if item.FileName == mainName {
				subdirFID = item.FID
				break
			}
		}
		if subdirFID == "" {
			continue
		}
		if config.AutoClean {
			cleanFIDs = append(cleanFIDs, zipFID)
			if config.AutoCleanZipdir {
				cleanFIDs = append(cleanFIDs, subdirFID)
			} else if err := env.Account.Rename(subdirFID, zipName); err != nil {
				p.deps.logf("  ⚠️ 占位重命名失败: %v", err)
			}
		} else {
			cleanFIDs = append(cleanFIDs, subdirFID)
		}
		items, err := env.Account.LsDir(subdirFID)
		if err != nil {
			p.deps.logf("  ⚠️ 读取解压目录失败: %v", err)
			continue
		}
		for _, item := range items {
			moveFIDs = append(moveFIDs, item.FID)
		}
		if len(items) == 1 {
			item := items[0]
			_, ext := splitExt(item.FileName)
			newName := mainName + ext
			if err := env.Account.Rename(item.FID, newName); err == nil {
				p.deps.logf("    └─ 重命名: %s", newName)
			}
		}
	}
	if len(moveFIDs) == 0 {
		return nil
	}
	p.deps.logf("🚀 任务全部解压完成，开始批量移动 %d 个文件...", len(moveFIDs))
	if err := env.Account.MoveFiles(moveFIDs, target); err != nil {
		return nil
	}
	if len(cleanFIDs) > 0 {
		if _, err := env.Account.Delete(cleanFIDs); err == nil {
			p.deps.logf("🧹 批量清理完成")
		}
	}
	return nil
}
