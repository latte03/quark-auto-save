package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"qas-poc/notify"
	"qas-poc/plugins"
	"qas-poc/quark"
	"qas-poc/signin"
	"qas-poc/source"
)

// executeRun is the goroutine body behind /api/run, /api/sign and the scheduler.
func (s *Server) executeRun(run *Run, spec runSpec) error {
	started := time.Now()
	out := run.Writer()
	logf := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }

	snapshot := s.store.Snapshot()
	if len(snapshot.Cookies) == 0 {
		return errors.New("未配置 Cookie，请先在设置中填写")
	}
	write := spec.Write
	indices := spec.Indices

	logf("===============程序开始===============")
	logf("⏰ 执行时间: %s", started.Format("2006-01-02 15:04:05"))
	logf("")

	var collected []string
	if spec.SignOnly || (write && len(indices) == 0 && spec.Inline == nil) {
		logf("===============签到任务===============")
		accounts := make([]*quark.Client, 0, len(snapshot.Cookies))
		for _, cookie := range snapshot.Cookies {
			accounts = append(accounts, s.clientFor(cookie))
		}
		results := signin.Run(accounts, signin.Options{
			Out:        out,
			Collect:    func(text string) { collected = append(collected, text) },
			SignNotify: snapshot.Push.SignNotifyEnabled(),
		})
		signed := 0
		for _, result := range results {
			if result.Signed {
				signed++
			}
		}
		logf("签到账号 %d 个，新签 %d 个", len(results), signed)
		logf("")
	}
	if spec.SignOnly {
		s.sendNotifications(out, snapshot.Push, collected)
		logf("===============程序结束===============")
		return nil
	}

	selected := indices
	if len(selected) == 0 && spec.Inline == nil {
		for index, task := range snapshot.Tasks {
			// Finished one-shot tasks stay in the list as a record but are
			// not part of "run everything" anymore.
			if task.OnceFinished() {
				continue
			}
			selected = append(selected, index)
		}
	}

	type runJob struct {
		index int
		task  *quark.Task
	}
	var jobs []runJob
	if spec.Inline != nil {
		jobs = []runJob{{index: -1, task: spec.Inline}}
	} else {
		for _, index := range selected {
			if index >= 0 && index < len(snapshot.Tasks) {
				jobs = append(jobs, runJob{index: index, task: snapshot.Tasks[index]})
			}
		}
	}

	client := s.clientFor(snapshot.Cookies[0])
	runner := quark.NewRunner(client, snapshot.Magic)
	runner.DryRun = !write
	runner.CreateDirs = write
	runner.Out = out
	runner.Now = time.Now

	pluginHTTP := &http.Client{Timeout: 15 * time.Second}
	pluginLog := func(format string, args ...any) { logf(format, args...) }
	pluginList := plugins.Filter(
		plugins.Build(snapshot.Plugins, plugins.Deps{HTTP: pluginHTTP, Log: pluginLog}),
		os.Getenv("PLUGIN_FLAGS"),
	)
	activePlugins := 0
	for _, plugin := range pluginList {
		if plugin.Active() {
			activePlugins++
		}
	}
	if len(pluginList) > 0 {
		logf("🧩 插件：启用 %d/%d", activePlugins, len(pluginList))
	}
	if write {
		logf("模式：执行（会真正转存与改名）")
	} else {
		logf("模式：演练（不会写入网盘）")
	}
	if spec.Inline != nil {
		logf("临时任务：执行后不写入任务列表")
	}

	var runTasks []*quark.Task
	for _, job := range jobs {
		runTasks = append(runTasks, job.task)
	}
	if err := runner.UpdateSavepathFID(runTasks); err != nil {
		logf("✗ 预解析保存目录失败: %v", err)
	}

	stats := map[string]int{}
	bans := map[int]string{}
	additions := map[int]json.RawMessage{}
	completed := map[int]bool{}
	saved := 0
	executed := 0
	for position, job := range jobs {
		index := job.index
		task := job.task
		logf("")
		logf("#%d------------------", position+1)
		logf("任务名称: %s", task.TaskName)
		logf("分享链接: %s", task.ShareURL)
		logf("保存路径: %s", task.SavePath)
		if task.Pattern != "" {
			logf("正则匹配: %s", task.Pattern)
		}
		if task.Replace != "" {
			logf("正则替换: %s", task.Replace)
		}
		if task.UpdateSubdir != "" {
			logf("更子目录: %s", task.UpdateSubdir)
		}
		if task.HasRunWeek() {
			logf("运行星期: %v", task.RunWeek)
		}
		if len(task.RunTime) > 0 {
			logf("运行时间: %s", strings.Join(task.RunTime, "、"))
		}
		if task.IsOnce() {
			logf("运行方式: 单次")
		}
		if task.EndDate != "" {
			logf("截止日期: %s", task.EndDate)
		}
		if task.ShareURLBan != "" {
			logf("《%s》：%s", task.TaskName, task.ShareURLBan)
			continue
		}

		scheduled, err := task.IsScheduled(time.Now())
		if err != nil {
			logf("✗ 任务周期解析失败: %v", err)
			continue
		}
		if !scheduled {
			logf("任务不在运行周期内，跳过")
			continue
		}
		executed++

		tree, err := runner.DoSaveTask(task)
		if err != nil {
			logf("✗ 任务失败: %v", err)
			continue
		}
		if task.ShareURLBan != "" && (index < 0 || task.ShareURLBan != snapshot.Tasks[index].ShareURLBan) {
			if write && index >= 0 {
				bans[index] = task.ShareURLBan
			}
		}
		// A one-shot task is finished once a real run went through, even if
		// the share had nothing new; failures and bans keep it runnable.
		if write && index >= 0 && task.IsOnce() && !task.OnceFinished() && task.ShareURLBan == "" {
			completed[index] = true
		}
		if err := task.MergePluginDefaults(pluginList); err != nil {
			logf("✗ 合并插件默认配置失败: %v", err)
		}
		if write && index >= 0 && len(task.Addition) > 0 {
			additions[index] = task.Addition
		}
		if tree == nil || tree.Size() == 0 {
			continue
		}
		count := countSavedFiles(tree)
		if write {
			stats[task.TaskName] = count
		}
		saved += count
		if !write {
			logf("（演练）跳过插件执行")
			continue
		}
		if activePlugins > 0 {
			quark.RunTaskPlugins(task, pluginList, &quark.PluginEnv{
				Account:     client,
				Tree:        tree,
				SavepathFID: runner.SavepathFID(),
				Log:         pluginLog,
				HTTP:        pluginHTTP,
			})
		}
		s.enqueueTaskDownload(task, tree, snapshot.Download, client, logf)
	}

	if write {
		run.setStats(stats)
		if spec.Inline == nil {
			if err := s.store.ApplyRunResults(bans, additions, completed); err != nil {
				logf("✗ 回写配置失败: %v", err)
			}
		}
	}

	logf("")
	logf("===============结果===============")
	logf("执行 %d 个任务，%d 个文件%s", executed, saved, map[bool]string{true: "已转存", false: "待转存（演练）"}[write])
	logf("😃 运行时长: %.2fs", time.Since(started).Seconds())

	messages := append(collected, runner.Notifications...)
	if len(messages) > 0 {
		if write {
			s.sendNotifications(out, snapshot.Push, messages)
		} else {
			logf("（演练）跳过推送：%d 条通知", len(messages))
		}
	}
	return nil
}

// sendNotifications pushes the aggregated run summary; failures stay in the
// run log and never abort the transfer.
func (s *Server) sendNotifications(out io.Writer, cfg notify.Config, messages []string) {
	body := strings.TrimSpace(strings.Join(messages, "\n"))
	if body == "" {
		return
	}
	logf := func(format string, args ...any) { fmt.Fprintf(out, format+"\n", args...) }
	logf("===============推送通知===============")
	results := notify.New(out).Send(context.Background(), cfg, "【夸克自动转存】", body)
	if summary := notify.Summarize(results); summary != "" {
		logf("%s", summary)
	}
	logf("")
}

// countSavedFiles counts leaves in the saved tree (dry-run nodes included).
func countSavedFiles(tree *quark.Tree) int {
	count := 0
	tree.Walk(func(node *quark.TreeNode, isRoot bool) {
		if !isRoot && !node.Data.IsDir {
			count++
		}
	})
	return count
}

func (s *Server) clientFor(cookie string) *quark.Client {
	if s.newClient != nil {
		return s.newClient(cookie)
	}
	return quark.New(cookie)
}

func (s *Server) client() (*quark.Client, error) {
	snapshot := s.store.Snapshot()
	if len(snapshot.Cookies) == 0 {
		return nil, errors.New("未配置 Cookie，请先在设置中填写")
	}
	return s.clientFor(snapshot.Cookies[0]), nil
}

func apiMessage(err error) string {
	var apiErr *quark.APIError
	if errors.As(err, &apiErr) && apiErr.Message != "" {
		return apiErr.Message
	}
	return err.Error()
}

// ---------- browse ----------

type browseEntry struct {
	Name     string `json:"name"`
	FID      string `json:"fid"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Category string `json:"category,omitempty"`
	Count    int    `json:"count,omitempty"`
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dirPath := quark.NormalizePath(r.URL.Query().Get("path"))
	if dirPath == "" {
		dirPath = "/"
	}
	fid := "0"
	if dirPath != "/" {
		fids, err := client.GetFids([]string{dirPath})
		if err != nil {
			writeError(w, http.StatusBadGateway, apiMessage(err))
			return
		}
		if len(fids) == 0 {
			writeError(w, http.StatusNotFound, "目录不存在: "+dirPath)
			return
		}
		fid = fids[0].FID
	}
	files, err := client.LsDir(fid)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	entries := make([]browseEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, browseEntry{
			Name:     file.FileName,
			FID:      file.FID,
			IsDir:    file.Dir,
			Size:     file.Size,
			Category: file.ObjCategory,
			Count:    file.IncludeItems,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"path": dirPath, "fid": fid, "entries": entries},
	})
}

// ---------- share ----------

func (s *Server) handleValidateShare(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ShareURL string `json:"shareurl"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	ref, err := quark.ExtractShareURL(request.ShareURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "分享链接格式无效")
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"ok": false, "message": apiMessage(err)},
		})
		return
	}
	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, ref.PdirFID, 0, 0)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"ok": false, "message": apiMessage(err)},
		})
		return
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"ok":    true,
			"title": title,
			"count": len(detail.List),
			"path":  len(ref.Paths),
		},
	})
}

type shareFileView struct {
	Name     string `json:"name"`
	FID      string `json:"fid"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Category string `json:"category,omitempty"`
	Matched  bool   `json:"matched"`
	Saved    bool   `json:"saved"`
	Renamed  string `json:"renamed,omitempty"`
}

const sharePreviewLimit = 500

func validFID(fid string) bool {
	if len(fid) == 0 || len(fid) > 64 {
		return false
	}
	for _, char := range fid {
		if (char < '0' || char > '9') && (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

func (s *Server) handleShareDetail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ShareURL        string `json:"shareurl"`
		PdirFID         string `json:"pdir_fid"`
		TaskName        string `json:"taskname"`
		MagicID         string `json:"magic_id"`
		Pattern         string `json:"pattern"`
		Replace         string `json:"replace"`
		SavePath        string `json:"savepath"`
		UpdateSubdir    string `json:"update_subdir"`
		IgnoreExtension bool   `json:"ignore_extension"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	ref, err := quark.ExtractShareURL(request.ShareURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "分享链接格式无效")
		return
	}
	pdirFID := ref.PdirFID
	if request.PdirFID != "" {
		if !validFID(request.PdirFID) {
			writeError(w, http.StatusBadRequest, "目录标识格式无效")
			return
		}
		pdirFID = request.PdirFID
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, pdirFID, 0, 0)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	files := detail.List
	if len(files) == 1 && files[0].Dir && pdirFID == "0" {
		nested, err := client.GetDetail(ref.PwdID, stoken.Stoken, files[0].FID, 0, 0)
		if err == nil && len(nested.List) > 0 {
			files = nested.List
		}
	}

	var dirFiles []quark.ShareFile
	savepath := quark.NormalizePath(request.SavePath)
	if savepath != "/" {
		if fids, err := client.GetFids([]string{savepath}); err == nil && len(fids) > 0 {
			dirFiles, _ = client.LsDir(fids[0].FID)
		}
	}
	dirNames := make([]string, len(dirFiles))
	for i, file := range dirFiles {
		dirNames[i] = file.FileName
	}

	snapshot := s.store.Snapshot()
	magic := quark.NewMagic(snapshot.Magic)
	magic.SetTaskName(request.TaskName)
	pattern, replace := magic.ResolveTask(request.MagicID, request.Pattern, request.Replace)

	var views []shareFileView
	var pending []*quark.ShareFile
	indexOf := map[string]int{}
	for _, item := range files {
		file := item
		search := pattern
		if file.Dir && request.UpdateSubdir != "" {
			search = request.UpdateSubdir
		}
		matched := false
		if search != "" {
			matched, err = magic.Match(search, file.FileName)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("正则格式错误: %v", err))
				return
			}
		}
		view := shareFileView{
			Name:     file.FileName,
			FID:      file.FID,
			IsDir:    file.Dir,
			Size:     file.Size,
			Category: file.ObjCategory,
			Matched:  matched,
		}
		if matched {
			if file.Dir {
				view.Saved = magic.Exists(file.FileName, dirNames, false) != ""
			} else {
				nameRe, err := magic.Sub(pattern, replace, file.FileName)
				if err != nil {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("正则替换失败: %v", err))
					return
				}
				view.Renamed = nameRe
				view.Saved = magic.Exists(nameRe, dirNames, request.IgnoreExtension) != ""
				if !view.Saved {
					file.FileNameRe = nameRe
					pending = append(pending, &file)
					indexOf[file.FID] = len(views)
				}
			}
		}
		views = append(views, view)
	}

	if len(pending) > 0 {
		if err := magic.SetDirFileList(dirFiles, replace); err == nil {
			magic.SortFileList(pending)
		}
		for _, file := range pending {
			if index, ok := indexOf[file.FID]; ok {
				views[index].Renamed = file.FileNameRe
			}
		}
	}

	total := len(views)
	if total > sharePreviewLimit {
		views = views[:sharePreviewLimit]
	}
	if views == nil {
		views = []shareFileView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"title":    title,
			"savepath": savepath,
			"pattern":  pattern,
			"replace":  replace,
			"total":    total,
			"files":    views,
		},
	})
}

// ---------- rename preview ----------

type renamePreviewItem struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
	Renamed string `json:"renamed"`
}

func (s *Server) handleRenamePreview(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TaskName string   `json:"taskname"`
		MagicID  string   `json:"magic_id"`
		Pattern  string   `json:"pattern"`
		Replace  string   `json:"replace"`
		Files    []string `json:"files"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(request.Files) == 0 || len(request.Files) > 50 {
		writeError(w, http.StatusBadRequest, "一次可预览 1 到 50 个文件名")
		return
	}
	snapshot := s.store.Snapshot()
	magic := quark.NewMagic(snapshot.Magic)
	magic.SetTaskName(request.TaskName)
	pattern, replace := magic.ResolveTask(request.MagicID, request.Pattern, request.Replace)

	items := make([]renamePreviewItem, len(request.Files))
	var planned []*quark.ShareFile
	var plannedIndex []int
	for index, name := range request.Files {
		item := renamePreviewItem{Name: name, Renamed: name}
		if pattern != "" {
			matched, err := magic.Match(pattern, name)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("正则格式错误: %v", err))
				return
			}
			item.Matched = matched
			if matched {
				renamed, err := magic.Sub(pattern, replace, name)
				if err != nil {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("正则替换失败: %v", err))
					return
				}
				item.Renamed = renamed
				planned = append(planned, &quark.ShareFile{FileName: name, FileNameRe: renamed})
				plannedIndex = append(plannedIndex, index)
			}
		} else {
			item.Matched = true
		}
		items[index] = item
	}
	if len(planned) > 0 && strings.Contains(replace, "{I") {
		if err := magic.SetDirFileList(nil, replace); err == nil {
			magic.SortFileList(planned)
		}
		for position, file := range planned {
			items[plannedIndex[position]].Renamed = file.FileNameRe
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"pattern": pattern,
			"replace": replace,
			"files":   items,
		},
	})
}

// ---------- search ----------

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	keyword := strings.TrimSpace(r.URL.Query().Get("keyword"))
	if keyword == "" {
		writeError(w, http.StatusBadRequest, "搜索关键词不能为空")
		return
	}
	snapshot := s.store.Snapshot()
	options := snapshot.Sources
	cloudConfigured := options.CloudSaver.Enable && strings.TrimSpace(options.CloudSaver.Server) != ""
	cloudReady := cloudConfigured &&
		(strings.TrimSpace(options.CloudSaver.Token) != "" ||
			(strings.TrimSpace(options.CloudSaver.Username) != "" && strings.TrimSpace(options.CloudSaver.Password) != ""))
	panReady := options.PanSou.Enable && strings.TrimSpace(options.PanSou.Server) != ""
	if !cloudReady && !panReady {
		switch {
		case cloudConfigured:
			writeError(w, http.StatusBadRequest, "CloudSaver 缺少账号密码或 Token，请到「设置 → 搜索源」补全")
		case options.CloudSaver.Enable || options.PanSou.Enable:
			writeError(w, http.StatusBadRequest, "搜索源已启用但配置不完整，请到「设置 → 搜索源」补全")
		default:
			writeError(w, http.StatusBadRequest, "未配置搜索源，请先到「设置 → 搜索源」启用 PanSou 或 CloudSaver")
		}
		return
	}
	var skipped []string
	if options.CloudSaver.Enable && !cloudReady {
		skipped = append(skipped, "CloudSaver 配置不完整，已跳过")
	}
	options.CloudSaver.Enable = cloudReady
	options.PanSou.Enable = panReady
	options.HTTP = s.http
	results, messages, token := source.Search(keyword, r.URL.Query().Get("deep") == "1", options)
	if token != "" {
		if err := s.store.UpdateSourceToken(token); err != nil {
			s.logger.Printf("保存 CloudSaver token 失败: %v", err)
		}
	}
	messages = append(skipped, messages...)
	if results == nil {
		results = []source.Result{}
	}
	if messages == nil {
		messages = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"keyword":  keyword,
			"results":  results,
			"messages": messages,
		},
	})
}

// ---------- sign & notify ----------

func (s *Server) handleSign(w http.ResponseWriter, r *http.Request) {
	if len(s.store.Snapshot().Cookies) == 0 {
		writeError(w, http.StatusBadRequest, "未配置 Cookie，请先在设置中填写")
		return
	}
	run, err := s.startRun(runSpec{Label: "签到", Write: true, SignOnly: true})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "run_id": run.ID})
}

func (s *Server) handleNotifyTest(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Title   string `json:"title"`
		Content string `json:"content"`
	}
	_ = decodeJSON(r, &request)
	if strings.TrimSpace(request.Title) == "" {
		request.Title = "【夸克自动转存】"
	}
	if strings.TrimSpace(request.Content) == "" {
		request.Content = "通知测试\n\n" + time.Now().Format("2006-01-02 15:04:05")
	}

	cfg := s.store.Snapshot().Push
	if len(notify.Channels(cfg)) == 0 {
		writeError(w, http.StatusBadRequest, "未配置推送渠道")
		return
	}
	var log strings.Builder
	ctx, cancel := context.WithTimeout(r.Context(), 40*time.Second)
	defer cancel()
	results := notify.New(&log).Send(ctx, cfg, request.Title, request.Content)

	channels := make([]map[string]any, 0, len(results))
	for _, result := range results {
		item := map[string]any{"channel": result.Channel, "ok": result.Err == nil}
		if result.Err != nil {
			item["error"] = result.Err.Error()
		}
		channels = append(channels, item)
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"channels": channels,
			"log":      log.String(),
		},
	})
}

// ---------- files ----------

// resolveFID maps a cloud path (or an explicit fid) to a file id. The root is
// never a valid target for rename/delete.
func resolveFID(client *quark.Client, path, fid string) (string, error) {
	if value := strings.TrimSpace(fid); value != "" {
		if value == "0" {
			return "", errors.New("不能操作根目录")
		}
		return value, nil
	}
	path = quark.NormalizePath(path)
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	if path == "" {
		return "", errors.New("缺少 path 或 fid")
	}
	if path == "/" {
		return "", errors.New("不能操作根目录")
	}
	items, err := client.GetFids([]string{path})
	if err != nil {
		return "", err
	}
	for _, item := range items {
		if item.FilePath == path && item.FID != "" {
			return item.FID, nil
		}
	}
	if len(items) == 1 && items[0].FID != "" {
		return items[0].FID, nil
	}
	return "", fmt.Errorf("未找到: %s", path)
}

func (s *Server) guardIdle(w http.ResponseWriter) bool {
	if s.runs.ActiveID() != "" {
		writeError(w, http.StatusConflict, "任务运行中，请稍后再操作文件")
		return false
	}
	return true
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path    string `json:"path"`
		FID     string `json:"fid"`
		Confirm bool   `json:"confirm"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if !request.Confirm {
		writeError(w, http.StatusBadRequest, "删除需要显式确认")
		return
	}
	if !s.guardIdle(w) {
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fid, err := resolveFID(client, request.Path, request.FID)
	if err != nil {
		writeError(w, http.StatusBadRequest, apiMessage(err))
		return
	}
	taskID, err := client.Delete([]string{fid})
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	s.logger.Printf("删除文件: %s (fid=%s)", quark.NormalizePath(request.Path), fid)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"fid": fid, "task_id": taskID},
	})
}

func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
		FID  string `json:"fid"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	name := strings.TrimSpace(request.Name)
	switch {
	case name == "":
		writeError(w, http.StatusBadRequest, "新名称不能为空")
		return
	case name == "." || name == "..":
		writeError(w, http.StatusBadRequest, "新名称无效")
		return
	case strings.ContainsAny(name, "/\\"):
		writeError(w, http.StatusBadRequest, "新名称不能包含路径分隔符")
		return
	case len([]byte(name)) > 255:
		writeError(w, http.StatusBadRequest, "新名称过长")
		return
	}
	if !s.guardIdle(w) {
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fid, err := resolveFID(client, request.Path, request.FID)
	if err != nil {
		writeError(w, http.StatusBadRequest, apiMessage(err))
		return
	}
	if err := client.Rename(fid, name); err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	s.logger.Printf("重命名文件: %s → %s", quark.NormalizePath(request.Path), name)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"fid": fid, "name": name},
	})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	dirPath := quark.NormalizePath(request.Path)
	if len(dirPath) > 1 {
		dirPath = strings.TrimRight(dirPath, "/")
	}
	if dirPath == "" || dirPath == "/" {
		writeError(w, http.StatusBadRequest, "目录路径无效")
		return
	}
	if !s.guardIdle(w) {
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fid, err := client.Mkdir(dirPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	s.logger.Printf("新建目录: %s", dirPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"path": dirPath, "fid": fid},
	})
}
