package plugins

import (
	"encoding/json"
	"fmt"
	"path"
	"regexp"
	"strings"
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

const (
	// Quark flips an unarchive task to status=2 before the extracted entries can
	// be enumerated (measured: the payload shows up ~0.3-0.5s after the task
	// reports done, sometimes much later under load). Listing the folder once
	// right after status=2 therefore comes back empty often enough to lose a
	// random part of a season, so the folder is polled instead.
	extractPollInterval = 500 * time.Millisecond
	extractPollAttempts = 30 // ~15s per round
	// unarchiveTaskWait is the ceiling for one unarchive task. Extraction of a
	// season-sized archive is a server-side copy, so this is generous.
	unarchiveTaskWait = 30 * time.Minute
)

// AutoUnarchive expands archives in the cloud and moves the files back.
type AutoUnarchive struct {
	deps          Deps
	cfg           autoUnarchiveConfig
	globalEnabled bool
	// sleep is replaceable so tests do not wait for the polling windows.
	sleep func(time.Duration)
}

// archiveJob tracks one archive through decompress -> move -> clean so that the
// archive and its extract folder are only deleted once the payload was
// observed in the task directory.
type archiveJob struct {
	zipFID  string
	zipName string
	target  string
	subdir  string
	items   []quark.ShareFile
	// names are the file names expected in target once the move is done.
	names []string
}

func (j *archiveJob) movedInto(moved map[string]bool) bool {
	if len(j.items) == 0 {
		return false
	}
	for _, item := range j.items {
		if !moved[item.FID] {
			return false
		}
	}
	return true
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
	account := env.Account
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

	var jobs []*archiveJob
	// Files are grouped per destination so that every directory gets a single
	// MoveFiles call; moveTargets keeps first-seen order for determinism.
	var moveTargets []string
	moveFiles := make(map[string][]string)
	for _, node := range archives {
		job := p.unpackOne(account, node, rootTarget, env.SavepathFID)
		if job == nil {
			continue
		}
		if _, seen := moveFiles[job.target]; !seen {
			moveTargets = append(moveTargets, job.target)
		}
		for _, item := range job.items {
			moveFiles[job.target] = append(moveFiles[job.target], item.FID)
		}
		jobs = append(jobs, job)
	}

	total := 0
	for _, target := range moveTargets {
		total += len(moveFiles[target])
	}
	if total == 0 {
		return nil
	}
	p.deps.logf("🚀 任务全部解压完成，开始批量移动 %d 个文件...", total)
	moved := make(map[string]bool, total)
	for _, target := range moveTargets {
		fids := moveFiles[target]
		if len(fids) == 0 {
			continue
		}
		if err := account.MoveFiles(fids, target); err != nil {
			p.deps.logf("  ❌ 移动 %d 个文件失败: %v", len(fids), err)
			continue
		}
		for _, fid := range fids {
			moved[fid] = true
		}
	}
	// Read the destinations back: a move that the server drops must not pass as
	// success, because the cleanup below is what makes a mistake permanent.
	verified := make(map[string]map[string]bool, len(moveTargets))
	for _, target := range moveTargets {
		if present := p.verifyMoved(account, target, jobs, moved); present != nil {
			verified[target] = present
		}
	}
	// The archives cleaned up below no longer exist in the drive, so their tree
	// entries must not reach the local download batch: a job built from the
	// pre-unpack tree would ask Quark for those fids and get 404 file not found.
	if cleaned := p.finish(account, config, jobs, moved, verified); len(cleaned) > 0 {
		if marked := markConsumed(env.Tree, cleaned); marked > 0 {
			p.deps.logf("  ℹ️ 已展开的压缩包不再进入本地下载队列: %d 个", marked)
		}
	}
	return nil
}

// markConsumed flags the tree entries whose archive file was removed after a
// verified unpack, so the download collector skips them instead of queuing a
// fid the drive no longer has.
func markConsumed(tree *quark.Tree, cleaned map[string]bool) int {
	if tree == nil || len(cleaned) == 0 {
		return 0
	}
	marked := 0
	tree.Walk(func(node *quark.TreeNode, isRoot bool) {
		if isRoot || node.Data.IsDir || !cleaned[node.Data.FID] {
			return
		}
		node.Data.Consumed = true
		marked++
	})
	return marked
}

// unpackOne decompresses one archive and returns the job to move later, or nil
// when the archive must be left untouched. Keeping the archive untouched is
// deliberate: deleting it is what turned a transient Quark delay into
// permanently missing episodes.
func (p *AutoUnarchive) unpackOne(account quark.PluginAccount, node *quark.TreeNode, rootTarget string, savepathFID map[string]string) *archiveJob {
	zipFID := node.Data.FID
	zipName := node.Data.FileNameRe
	if zipName == "" {
		zipName = node.Data.FileName
	}
	dirPath := archiveDirPath(node)
	target := ""
	if dirPath != "" {
		target = savepathFID[dirPath]
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
		return nil
	}
	mainName, _ := splitExt(zipName)
	taskID, err := account.Unarchive(zipFID, target)
	if err != nil {
		p.deps.logf("  ❌ 提交失败: %s (%v)", zipName, err)
		return nil
	}
	if dirPath != "" && target != rootTarget {
		p.deps.logf("  ▶️ 提交解压: %s → %s", zipName, dirPath)
	} else {
		p.deps.logf("  ▶️ 提交解压: %s", zipName)
	}
	result, err := account.WaitTask(taskID, unarchiveTaskWait)
	if err != nil {
		p.deps.logf("  ⚠️ 任务异常: %s %v（保留压缩包，下次运行重试）", zipName, err)
		return nil
	}
	p.deps.logf("  ✅ 解压完成: %s", zipName)

	subdir, items, err := p.awaitExtract(account, taskID, result, mainName, zipName)
	if err != nil {
		p.deps.logf("  ⚠️ 读取解压目录失败: %s %v（保留压缩包，下次运行重试）", zipName, err)
		return nil
	}
	if subdir == "" {
		p.deps.logf("  ⚠️ 解压结果里找不到解压目录: %s（保留压缩包，下次运行重试）", zipName)
		return nil
	}
	if len(items) == 0 {
		p.deps.logf("  ⚠️ 解压目录为空: %s（保留压缩包与解压目录，下次运行重试；要立刻重试可删除该压缩包后手动运行任务）", zipName)
		return nil
	}
	job := &archiveJob{
		zipFID:  zipFID,
		zipName: zipName,
		target:  target,
		subdir:  subdir,
		items:   items,
		names:   make([]string, 0, len(items)),
	}
	for _, item := range items {
		job.names = append(job.names, item.FileName)
	}
	if len(items) == 1 {
		item := items[0]
		_, ext := splitExt(item.FileName)
		newName := mainName + ext
		if err := account.Rename(item.FID, newName); err != nil {
			p.deps.logf("  ⚠️ 重命名失败: %s → %s %v", item.FileName, newName, err)
		} else {
			p.deps.logf("    └─ 重命名: %s", newName)
			job.names[0] = newName
		}
	}
	return job
}

// awaitExtract waits until the extracted folder actually lists something.
// Quark reports the unarchive finished before the entries exist, so the poll is
// the whole point; when the folder still looks empty after one full window the
// task result is read again, because the folder itself can show up late (or
// under a new fid) as well.
func (p *AutoUnarchive) awaitExtract(account quark.PluginAccount, taskID string, result quark.TaskResult, mainName, label string) (string, []quark.ShareFile, error) {
	subdir := extractDirFID(result, mainName)
	var lastErr error
	for round := 0; round < 2; round++ {
		if subdir != "" {
			items, err := p.pollExtractDir(account, subdir, label)
			if err != nil {
				lastErr = err
			} else {
				lastErr = nil
				if len(items) > 0 {
					return subdir, items, nil
				}
			}
		}
		if round == 1 || taskID == "" {
			break
		}
		refreshed, err := account.WaitTask(taskID, unarchiveTaskWait)
		if err != nil {
			if lastErr == nil {
				lastErr = err
			}
			break
		}
		next := extractDirFID(refreshed, mainName)
		if next == "" || next == subdir {
			break
		}
		subdir = next
	}
	return subdir, nil, lastErr
}

// pollExtractDir lists one folder until it has entries, roughly 15s in total.
func (p *AutoUnarchive) pollExtractDir(account quark.PluginAccount, pdirFID, label string) ([]quark.ShareFile, error) {
	var lastErr error
	for attempt := 0; attempt < extractPollAttempts; attempt++ {
		items, err := account.LsDir(pdirFID)
		if err != nil {
			lastErr = err
		} else {
			lastErr = nil
			if len(items) > 0 {
				if attempt > 0 {
					// Worth logging: it is the delay that used to lose episodes.
					p.deps.logf("    ⏳ 解压结果延迟出现: %s（第 %d 次轮询，约 %.1fs）",
						label, attempt+1, (time.Duration(attempt) * extractPollInterval).Seconds())
				}
				return items, nil
			}
		}
		if attempt+1 < extractPollAttempts {
			p.pause(extractPollInterval)
		}
	}
	return nil, lastErr
}

// extractDirFID picks the folder the unarchive created: normally the entry that
// carries the archive's base name. The single-entry fallback keeps archives
// whose base name Quark rewrites from being skipped silently.
func extractDirFID(result quark.TaskResult, mainName string) string {
	for _, item := range result.UnarchiveList {
		if item.FID != "" && item.FileName == mainName {
			return item.FID
		}
	}
	if len(result.UnarchiveList) == 1 && looksLikeDir(result.UnarchiveList[0]) {
		return result.UnarchiveList[0].FID
	}
	return ""
}

// looksLikeDir reports whether an unarchive result entry is a directory. The
// list carries no "dir" flag, so an entry without size but with items does.
func looksLikeDir(item quark.ShareFile) bool {
	return item.FID != "" && (item.Dir || item.IncludeItems > 0 || item.Size == 0)
}

// verifyMoved re-reads the destination and returns the names it holds, so the
// cleanup can require positive evidence that the files landed.
func (p *AutoUnarchive) verifyMoved(account quark.PluginAccount, target string, jobs []*archiveJob, moved map[string]bool) map[string]bool {
	var want []string
	for _, job := range jobs {
		if job.target == target && job.movedInto(moved) {
			want = append(want, job.names...)
		}
	}
	if len(want) == 0 {
		return nil
	}
	entries, err := account.LsDir(target)
	if err != nil {
		p.deps.logf("  ⚠️ 移动后无法核对目标目录: %v", err)
		return nil
	}
	present := make(map[string]bool, len(entries))
	for _, entry := range entries {
		if entry.FileName != "" {
			present[entry.FileName] = true
		}
	}
	var missing []string
	for _, name := range want {
		if !present[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		p.deps.logf("  ⚠️ 移动后目标目录缺少 %d 个文件: %s", len(missing), strings.Join(missing, "、"))
	}
	return present
}

// finish removes archives whose payload is confirmed in the task directory.
// Everything else stays in the drive, because deleting an archive that never
// produced its files is how episodes disappear for good. It returns the archive
// fids that were actually deleted, so the caller can keep the transfer diff
// from treating them as downloadable files.
func (p *AutoUnarchive) finish(account quark.PluginAccount, config autoUnarchiveTaskConfig, jobs []*archiveJob, moved map[string]bool, verified map[string]map[string]bool) map[string]bool {
	var cleanFIDs []string
	cleanedArchive := map[string]bool{}
	for _, job := range jobs {
		if !job.movedInto(moved) {
			p.deps.logf("  ⚠️ 移动不完整: %s（保留压缩包与解压目录，下次运行重试）", job.zipName)
			continue
		}
		if present := verified[job.target]; present != nil && !containsAll(present, job.names) {
			p.deps.logf("  ⚠️ 目标目录里看不到 %s 的文件（保留压缩包与解压目录，下次运行核对）", job.zipName)
			continue
		}
		switch {
		case !config.AutoClean:
			// The archive itself is the placeholder here, so only the emptied
			// extract folder is dropped.
			cleanFIDs = append(cleanFIDs, job.subdir)
		case config.AutoCleanZipdir:
			cleanFIDs = append(cleanFIDs, job.zipFID, job.subdir)
			cleanedArchive[job.zipFID] = true
		default:
			// Rename the emptied folder to the archive name as a placeholder so
			// the next transfer diff treats this archive as saved. That check
			// also honours ignore_extension, which matches the saved video, so
			// the placeholder is only created after the files really landed:
			// otherwise it would hide a missing episode for good.
			if err := account.Rename(job.subdir, job.zipName); err != nil {
				p.deps.logf("  ⚠️ 占位重命名失败: %s %v（保留压缩包，下次运行重试）", job.zipName, err)
				continue
			}
			cleanFIDs = append(cleanFIDs, job.zipFID)
			cleanedArchive[job.zipFID] = true
		}
	}
	if len(cleanFIDs) == 0 {
		return nil
	}
	if _, err := account.Delete(cleanFIDs); err != nil {
		p.deps.logf("  ⚠️ 批量清理失败: %v", err)
		return nil
	}
	p.deps.logf("🧹 批量清理完成")
	return cleanedArchive
}

func containsAll(present map[string]bool, names []string) bool {
	for _, name := range names {
		if !present[name] {
			return false
		}
	}
	return true
}

func (p *AutoUnarchive) pause(d time.Duration) {
	if p.sleep != nil {
		p.sleep(d)
		return
	}
	time.Sleep(d)
}
