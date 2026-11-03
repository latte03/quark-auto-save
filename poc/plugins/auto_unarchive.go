package plugins

import (
	"encoding/json"
	"fmt"
	"path"
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

// archiveDirPath returns the normalized cloud directory that holds the archive,
// or "" when the archive sits at an unusable location (no path, or disk root).
// Cloud paths are always "/"-separated, so the path package is the right tool.
func archiveDirPath(node *quark.TreeNode) string {
	if node == nil || node.Data.Path == "" {
		return ""
	}
	dir := path.Dir(node.Data.Path)
	if dir == "." || dir == "/" || dir == "" {
		return ""
	}
	dir = quark.NormalizePath(dir)
	if dir == "/" || dir == "." || dir == "/." {
		return ""
	}
	return dir
}

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
	rootTarget := env.SavepathFID[quark.NormalizePath(task.SavePath)]
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

	var cleanFIDs []string
	// Files are grouped per destination so that every directory gets a single
	// MoveFiles call; moveTargets keeps first-seen order for determinism.
	var moveTargets []string
	moveFiles := make(map[string][]string)
	for _, node := range archives {
		zipFID := node.Data.FID
		zipName := node.Data.FileNameRe
		if zipName == "" {
			zipName = node.Data.FileName
		}
		dirPath := archiveDirPath(node)
		target := ""
		if dirPath != "" {
			target = env.SavepathFID[dirPath]
		}
		if target == "" {
			if rootTarget != "" {
				if dirPath != "" {
					p.deps.logf("  ⚠️ 找不到 %s 的目录 fid，退回任务根目录", dirPath)
				} else {
					p.deps.logf("  ⚠️ 压缩包 %s 没有可用的所在目录，退回任务根目录", zipName)
				}
			}
			target = rootTarget
		}
		if target == "" {
			p.deps.logf("  ⚠️ 跳过压缩包: %s 找不到目标目录 fid", zipName)
			continue
		}
		mainName, _ := splitExt(zipName)
		taskID, err := env.Account.Unarchive(zipFID, target)
		if err != nil {
			p.deps.logf("  ❌ 提交失败: %s (%v)", zipName, err)
			continue
		}
		if dirPath != "" && target != rootTarget {
			p.deps.logf("  ▶️ 提交解压: %s → %s", zipName, dirPath)
		} else {
			p.deps.logf("  ▶️ 提交解压: %s", zipName)
		}
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
			if _, seen := moveFiles[target]; !seen {
				moveTargets = append(moveTargets, target)
			}
			moveFiles[target] = append(moveFiles[target], item.FID)
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
	total := 0
	for _, target := range moveTargets {
		total += len(moveFiles[target])
	}
	if total == 0 {
		return nil
	}
	p.deps.logf("🚀 任务全部解压完成，开始批量移动 %d 个文件...", total)
	for _, target := range moveTargets {
		fids := moveFiles[target]
		if len(fids) == 0 {
			continue
		}
		if err := env.Account.MoveFiles(fids, target); err != nil {
			p.deps.logf("  ❌ 移动 %d 个文件失败: %v", len(fids), err)
			return nil
		}
	}
	if len(cleanFIDs) > 0 {
		if _, err := env.Account.Delete(cleanFIDs); err != nil {
			p.deps.logf("  ⚠️ 批量清理失败: %v", err)
		} else {
			p.deps.logf("🧹 批量清理完成")
		}
	}
	return nil
}
