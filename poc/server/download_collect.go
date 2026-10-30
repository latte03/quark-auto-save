package server

import (
	"strings"

	"qas-poc/quark"
)

// collectDownloadItems turns saved tree leaves into local download items,
// keeping paths relative to the task's save root.
func collectDownloadItems(tree *quark.Tree, task *quark.Task) []downloadItem {
	prefix := strings.TrimSuffix(strings.TrimSpace(task.SavePath), "/")
	var items []downloadItem
	tree.Walk(func(node *quark.TreeNode, isRoot bool) {
		if isRoot || node.Data.IsDir || node.Data.Planned || node.Data.FID == "" {
			return
		}
		rel := node.Data.Path
		if prefix != "" && strings.HasPrefix(rel, prefix) {
			rel = strings.TrimPrefix(rel, prefix)
		}
		rel = strings.TrimPrefix(rel, "/")
		if rel == "" {
			rel = node.Data.FileNameRe
		}
		items = append(items, downloadItem{RelPath: rel, FID: node.Data.FID, Size: node.Data.Size})
	})
	return items
}

// missingWalkBudget caps the cloud listing per run so a large save tree cannot
// serialize an unbounded number of Quark calls on the run's critical path.
const missingWalkBudget = 4000

// collectMissingDownloads walks the task's cloud save path and keeps the
// files without a local copy, so a run also repairs gaps from earlier runs.
// Entries in skip (this run's transfers) are ignored: they are already
// queued with their final names.
func (s *Server) collectMissingDownloads(client *quark.Client, task *quark.Task, root string, overwrite bool, skip map[string]bool, logf func(string, ...any)) []downloadItem {
	savepath := quark.NormalizePath(task.SavePath)
	if len(savepath) > 1 {
		savepath = strings.TrimRight(savepath, "/")
	}
	if savepath == "/" {
		return nil
	}
	resolved, err := client.GetFids([]string{savepath})
	if err != nil {
		logf("✗ 读取云端保存目录失败，跳过本地补齐：%v", err)
		return nil
	}
	rootFID := ""
	for _, item := range resolved {
		if item.FilePath == savepath && item.FID != "" {
			rootFID = item.FID
			break
		}
	}
	if rootFID == "" {
		logf("✗ 云端保存目录不存在，跳过本地补齐：%s", savepath)
		return nil
	}

	var items []downloadItem
	visited := map[string]bool{}
	walked := 0
	truncated := false
	var walk func(fid, prefix string, depth int)
	walk = func(fid, prefix string, depth int) {
		if fid == "" || depth > 16 || visited[fid] || truncated {
			return
		}
		if walked >= missingWalkBudget {
			truncated = true
			return
		}
		walked++
		visited[fid] = true
		entries, err := client.LsDir(fid)
		if err != nil {
			logf("✗ 读取云端目录失败 %s：%v", savepath+prefix, err)
			return
		}
		for _, entry := range entries {
			rel := entry.FileName
			if prefix != "" {
				rel = prefix + "/" + entry.FileName
			}
			if entry.Dir {
				walk(entry.FID, rel, depth+1)
				continue
			}
			walked++
			if walked > missingWalkBudget {
				truncated = true
				return
			}
			if entry.FID == "" || skip[entry.FID] {
				continue
			}
			if !overwrite && localFileExists(root, rel) {
				continue
			}
			items = append(items, downloadItem{RelPath: rel, FID: entry.FID, Size: entry.Size})
		}
	}
	walk(rootFID, "", 0)
	if truncated {
		logf("⚠ 云端保存目录较大，本次只检查了前 %d 个条目：%s", missingWalkBudget, savepath)
	}
	return items
}

// enqueueTaskDownload queues the run's new files plus any cloud files that
// are still missing locally. It reports whether a job was created; onFinish
// runs when that job reaches a terminal status.
func (s *Server) enqueueTaskDownload(task *quark.Task, tree *quark.Tree, settings localDownloadSettings, client *quark.Client, logf func(string, ...any), onFinish func(status string)) bool {
	if task.LocalDownload == nil || !task.LocalDownload.Enable {
		return false
	}
	root := strings.TrimSpace(task.LocalDownload.Dir)
	if root == "" {
		root = settings.Dir
	}
	if root == "" {
		logf("⚠ 已开启本地下载，但未配置下载目录")
		return false
	}
	if message := validateLocalRoot(root); message != "" {
		logf("✗ 本地下载目录无效：%s", message)
		return false
	}
	overwrite := task.LocalDownload.Overwrite
	var items []downloadItem
	if tree != nil {
		items = collectDownloadItems(tree, task)
	}
	// The cloud diff above only sees this run's transfers. Comparing the
	// whole save path with local files also repairs gaps left by earlier
	// runs that transferred but never reached the NAS.
	skip := make(map[string]bool, len(items))
	for _, item := range items {
		skip[item.FID] = true
	}
	missing := s.collectMissingDownloads(client, task, root, overwrite, skip, logf)
	logf("本地增量：本次新增 %d，云端已有但本地缺失 %d", len(items), len(missing))
	items = append(items, missing...)
	if len(items) == 0 {
		logf("⚠ 已开启本地下载：云端与本地都已是最新，无需下载")
		return false
	}
	job := &downloadJob{
		TaskName:    task.TaskName,
		Root:        root,
		Items:       items,
		Concurrency: settings.Concurrency,
		MinFreeGB:   settings.MinFreeGB,
		Overwrite:   overwrite,
		Client:      client,
		OnFinish:    onFinish,
	}
	// Byte progress is only meaningful when every item carries its size.
	complete := true
	for _, item := range items {
		if item.Size <= 0 {
			complete = false
			break
		}
		job.TotalBytes += item.Size
	}
	if !complete {
		job.TotalBytes = 0
	}
	id := s.downloads.Enqueue(job)
	logf("⬇ 已加入本地下载队列：%d 个文件 → %s（%s）", len(items), root, id)
	return true
}
