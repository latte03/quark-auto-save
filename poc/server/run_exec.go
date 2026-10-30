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
)

// runnableTask reports whether a manual run should include the task. One-shot
// tasks ignore the weekday and end date: the operator triggers them by hand,
// so a leftover schedule from the task they were converted from must not skip
// the run.
func runnableTask(task *quark.Task, now time.Time) (bool, error) {
	if task.IsOnce() {
		return true, nil
	}
	return task.IsScheduled(now)
}

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

		scheduled, err := runnableTask(task, time.Now())
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
				// 立即落盘，运行过程被打断时封锁标记也不会丢。
				if err := s.store.ApplyRunResults(map[int]string{index: task.ShareURLBan}, nil, nil); err != nil {
					logf("✗ 回写封锁标记失败: %v", err)
				}
			}
		}
		// A one-shot task finishes when its download job finishes; one that
		// needs no download is done as soon as the transfer succeeded.
		// Failures and bans keep it runnable.
		finishOnce := func(queued bool) {
			if !write || index < 0 || !task.IsOnce() || task.OnceFinished() || task.ShareURLBan != "" {
				return
			}
			if !queued {
				completed[index] = true
			}
		}
		var onDownloadDone func(string)
		// 临时任务不在任务列表里；装了回调会按重名误标已有的一次性任务。
		if write && index >= 0 && task.IsOnce() && !task.OnceFinished() {
			onDownloadDone = s.onceDoneCallback(task)
		}
		if err := task.MergePluginDefaults(pluginList); err != nil {
			logf("✗ 合并插件默认配置失败: %v", err)
		}
		if write && index >= 0 && len(task.Addition) > 0 {
			additions[index] = task.Addition
		}
		if tree == nil || tree.Size() == 0 {
			// Nothing new in the cloud, but the NAS may still be missing
			// files from earlier runs.
			queued := false
			if write {
				queued = s.enqueueTaskDownload(task, nil, snapshot.Download, client, logf, onDownloadDone)
			}
			finishOnce(queued)
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
		queued := s.enqueueTaskDownload(task, tree, snapshot.Download, client, logf, onDownloadDone)
		finishOnce(queued)
	}

	if write {
		run.setStats(stats)
		if spec.Inline == nil {
			if err := s.store.ApplyRunResults(nil, additions, completed); err != nil {
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

// onceDoneCallback returns the download completion hook for a one-shot task.
// Only a finished transfer completes it; canceled, failed and paused jobs stay
// runnable so the local copy is retried.
func (s *Server) onceDoneCallback(task *quark.Task) func(string) {
	return func(status string) {
		if status != "done" {
			return
		}
		if err := s.store.MarkOnceDone(task.TaskName); err != nil {
			s.logger.Printf("标记单次任务完成失败: %v", err)
		}
	}
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
