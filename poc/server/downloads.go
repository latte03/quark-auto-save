package server

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"qas-poc/quark"
)

type downloadItem struct {
	RelPath string
	FID     string
}

// downloadJob is one task's local download batch; it moves through
// queued -> running -> done/failed and keeps counters for the UI.
type downloadJob struct {
	ID          string
	TaskName    string
	Root        string
	Items       []downloadItem
	Concurrency int
	MinFreeGB   int64
	Overwrite   bool
	Client      *quark.Client

	mu       sync.Mutex
	status   string
	done     int
	failed   int
	skipped  int
	bytes    int64
	message  string
	started  time.Time
	finished time.Time

	aborted atomic.Bool
}

type downloadJobView struct {
	ID         string `json:"id"`
	TaskName   string `json:"task_name"`
	Root       string `json:"root"`
	Status     string `json:"status"`
	Total      int    `json:"total"`
	Done       int    `json:"done"`
	Failed     int    `json:"failed"`
	Skipped    int    `json:"skipped"`
	Bytes      int64  `json:"bytes"`
	Message    string `json:"message,omitempty"`
	StartedAt  string `json:"started_at,omitempty"`
	FinishedAt string `json:"finished_at,omitempty"`
}

type downloadQueue struct {
	mu     sync.Mutex
	jobs   map[string]*downloadJob
	order  []string
	queue  chan *downloadJob
	logger *log.Logger
	http   *http.Client
}

func newDownloadQueue(logger *log.Logger) *downloadQueue {
	queue := &downloadQueue{
		jobs:   map[string]*downloadJob{},
		queue:  make(chan *downloadJob, 32),
		logger: logger,
		http: &http.Client{
			Transport: &http.Transport{
				ResponseHeaderTimeout: 30 * time.Second,
				IdleConnTimeout:       90 * time.Second,
			},
		},
	}
	go queue.worker()
	return queue
}

func (q *downloadQueue) Enqueue(job *downloadJob) string {
	q.mu.Lock()
	job.ID = fmt.Sprintf("dl-%d", time.Now().UnixNano())
	job.status = "queued"
	q.jobs[job.ID] = job
	q.order = append(q.order, job.ID)
	q.mu.Unlock()
	q.queue <- job
	return job.ID
}

func (q *downloadQueue) worker() {
	for job := range q.queue {
		q.run(job)
		q.mu.Lock()
		kept := q.order[:0]
		for _, id := range q.order {
			if id == job.ID {
				continue
			}
			kept = append(kept, id)
		}
		q.order = append(kept, job.ID)
		for len(q.order) > 20 {
			oldest := q.order[0]
			if old, ok := q.jobs[oldest]; ok && old.currentStatus() != "queued" && old.currentStatus() != "running" {
				delete(q.jobs, oldest)
				q.order = q.order[1:]
				continue
			}
			break
		}
		q.mu.Unlock()
	}
}

func (q *downloadQueue) Snapshot() []downloadJobView {
	q.mu.Lock()
	defer q.mu.Unlock()
	views := make([]downloadJobView, 0, len(q.order))
	for _, id := range q.order {
		job, ok := q.jobs[id]
		if !ok {
			continue
		}
		views = append(views, job.view())
	}
	for i, j := 0, len(views)-1; i < j; i, j = i+1, j-1 {
		views[i], views[j] = views[j], views[i]
	}
	return views
}

func (j *downloadJob) currentStatus() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	return j.status
}

func (j *downloadJob) setStatus(status string) {
	j.mu.Lock()
	j.status = status
	j.mu.Unlock()
}

func (j *downloadJob) fail(message string) {
	j.mu.Lock()
	if j.status != "failed" {
		j.status = "failed"
	}
	if j.message == "" {
		j.message = message
	}
	j.mu.Unlock()
	j.aborted.Store(true)
}

func (j *downloadJob) addDone(bytes int64) {
	j.mu.Lock()
	j.done++
	j.bytes += bytes
	j.mu.Unlock()
}

func (j *downloadJob) addSkipped() {
	j.mu.Lock()
	j.skipped++
	j.mu.Unlock()
}

func (j *downloadJob) addFailed(message string) {
	j.mu.Lock()
	j.failed++
	if j.message == "" {
		j.message = message
	}
	j.mu.Unlock()
}

func (j *downloadJob) view() downloadJobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	view := downloadJobView{
		ID:       j.ID,
		TaskName: j.TaskName,
		Root:     j.Root,
		Status:   j.status,
		Total:    len(j.Items),
		Done:     j.done,
		Failed:   j.failed,
		Skipped:  j.skipped,
		Bytes:    j.bytes,
		Message:  j.message,
	}
	if !j.started.IsZero() {
		view.StartedAt = j.started.Format(time.RFC3339)
	}
	if !j.finished.IsZero() {
		view.FinishedAt = j.finished.Format(time.RFC3339)
	}
	return view
}

func (q *downloadQueue) run(job *downloadJob) {
	job.setStatus("running")
	job.mu.Lock()
	job.started = time.Now()
	job.mu.Unlock()
	defer func() {
		job.mu.Lock()
		job.finished = time.Now()
		if job.status != "failed" {
			job.status = "done"
		}
		job.mu.Unlock()
	}()

	if message := validateLocalRoot(job.Root); message != "" {
		job.fail(message)
		q.logf("✗ 下载 %s：%s", job.TaskName, message)
		return
	}
	if check := checkLocalDir(job.Root); !check.OK {
		job.fail(check.Message)
		q.logf("✗ 下载 %s：%s", job.TaskName, check.Message)
		return
	}
	if message := ensureFreeSpace(job.Root, job.MinFreeGB); message != "" {
		job.fail(message)
		q.logf("✗ 下载 %s：%s", job.TaskName, message)
		return
	}

	q.logf("⬇ 开始下载 %s：%d 个文件 → %s", job.TaskName, len(job.Items), job.Root)
	concurrency := job.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 8 {
		concurrency = 8
	}
	sem := make(chan struct{}, concurrency)
	var wait sync.WaitGroup
	for _, item := range job.Items {
		if job.aborted.Load() {
			break
		}
		wait.Add(1)
		go func(entry downloadItem) {
			defer wait.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			if job.aborted.Load() {
				return
			}
			q.transfer(job, entry)
		}(item)
	}
	wait.Wait()

	view := job.view()
	q.logf("⬇ 下载结束 %s：完成 %d，跳过 %d，失败 %d，%s",
		job.TaskName, view.Done, view.Skipped, view.Failed, formatBytes(view.Bytes))
}

func (q *downloadQueue) transfer(job *downloadJob, item downloadItem) {
	rel, ok := safeRelPath(item.RelPath)
	if !ok {
		job.addFailed(fmt.Sprintf("非法路径 %q", item.RelPath))
		return
	}
	root := filepath.Clean(job.Root)
	target := filepath.Join(root, rel)
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		job.addFailed(fmt.Sprintf("路径越界 %q", item.RelPath))
		return
	}

	if info, err := os.Stat(target); err == nil && !info.IsDir() && !job.Overwrite {
		job.addSkipped()
		return
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		job.addFailed(fmt.Sprintf("创建目录失败: %v", err))
		return
	}
	if message := ensureFreeSpace(job.Root, job.MinFreeGB); message != "" {
		job.fail(message)
		return
	}

	links, cookies, err := job.Client.Download([]string{item.FID})
	if err != nil {
		job.addFailed(fmt.Sprintf("获取下载地址失败: %v", err))
		q.logf("✗ %s：获取下载地址失败: %v", rel, err)
		return
	}
	if len(links) == 0 || links[0].DownloadURL == "" {
		job.addFailed("未返回下载地址")
		return
	}

	size, err := q.fetchFile(links[0].DownloadURL, cookies, target)
	if err != nil {
		job.addFailed(fmt.Sprintf("%s: %v", rel, err))
		q.logf("✗ %s：%v", rel, err)
		return
	}
	job.addDone(size)
	q.logf("✓ %s（%s）", rel, formatBytes(size))
}

// fetchFile downloads into <target>.part with Range resume and renames on
// success. Interrupted transfers keep the .part file for the next attempt.
func (q *downloadQueue) fetchFile(url, cookie, target string) (int64, error) {
	part := target + ".part"
	var offset int64
	if info, err := os.Stat(part); err == nil && info.Size() > 0 {
		offset = info.Size()
	}

	request, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("构造请求失败: %w", err)
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if offset > 0 {
		request.Header.Set("Range", fmt.Sprintf("bytes=%d-", offset))
	}
	response, err := q.http.Do(request)
	if err != nil {
		return 0, fmt.Errorf("下载失败: %w", err)
	}
	defer response.Body.Close()

	flags := os.O_CREATE | os.O_WRONLY
	switch response.StatusCode {
	case http.StatusPartialContent:
		flags |= os.O_APPEND
	case http.StatusOK:
		offset = 0
		flags |= os.O_TRUNC
	case http.StatusRequestedRangeNotSatisfiable:
		if info, statErr := os.Stat(part); statErr == nil && info.Size() > 0 {
			if err := os.Rename(part, target); err == nil {
				return info.Size(), nil
			}
		}
		return 0, fmt.Errorf("下载地址拒绝续传")
	default:
		return 0, fmt.Errorf("下载失败: HTTP %d", response.StatusCode)
	}

	file, err := os.OpenFile(part, flags, 0o644)
	if err != nil {
		return 0, fmt.Errorf("写入失败: %w", err)
	}
	written, copyErr := io.CopyBuffer(file, response.Body, make([]byte, 256*1024))
	closeErr := file.Close()
	if copyErr != nil {
		return offset + written, fmt.Errorf("传输中断，可续传: %w", copyErr)
	}
	if closeErr != nil {
		return offset + written, fmt.Errorf("写入失败: %w", closeErr)
	}
	if err := os.Rename(part, target); err != nil {
		return offset + written, fmt.Errorf("重命名失败: %w", err)
	}
	return offset + written, nil
}

func (q *downloadQueue) logf(format string, args ...any) {
	if q.logger != nil {
		q.logger.Printf(format, args...)
	}
}

func ensureFreeSpace(root string, minFreeGB int64) string {
	if minFreeGB <= 0 {
		return ""
	}
	required := uint64(minFreeGB) * 1024 * 1024 * 1024
	if free := freeBytes(root); free < required {
		return fmt.Sprintf("可用空间不足：剩余 %s，低于保留阈值 %d GB", formatBytes(int64(free)), minFreeGB)
	}
	return ""
}

func safeRelPath(rel string) (string, bool) {
	parts := strings.Split(strings.ReplaceAll(rel, "\\", "/"), "/")
	var clean []string
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" || part == "." {
			continue
		}
		if part == ".." || strings.ContainsRune(part, 0) {
			return "", false
		}
		clean = append(clean, part)
	}
	if len(clean) == 0 {
		return "", false
	}
	return filepath.Join(clean...), true
}

func formatBytes(bytes int64) string {
	if bytes <= 0 {
		return "0 B"
	}
	units := []string{"B", "KB", "MB", "GB", "TB"}
	value := float64(bytes)
	index := 0
	for value >= 1024 && index < len(units)-1 {
		value /= 1024
		index++
	}
	if index == 0 {
		return strconv.FormatInt(bytes, 10) + " B"
	}
	return fmt.Sprintf("%.1f %s", value, units[index])
}

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
		items = append(items, downloadItem{RelPath: rel, FID: node.Data.FID})
	})
	return items
}

func (s *Server) enqueueTaskDownload(task *quark.Task, tree *quark.Tree, settings localDownloadSettings, client *quark.Client, logf func(string, ...any)) {
	if task.LocalDownload == nil || !task.LocalDownload.Enable {
		return
	}
	root := strings.TrimSpace(task.LocalDownload.Dir)
	if root == "" {
		root = settings.Dir
	}
	if root == "" {
		logf("⚠ 已开启本地下载，但未配置下载目录")
		return
	}
	if message := validateLocalRoot(root); message != "" {
		logf("✗ 本地下载目录无效：%s", message)
		return
	}
	items := collectDownloadItems(tree, task)
	if len(items) == 0 {
		return
	}
	id := s.downloads.Enqueue(&downloadJob{
		TaskName:    task.TaskName,
		Root:        root,
		Items:       items,
		Concurrency: settings.Concurrency,
		MinFreeGB:   settings.MinFreeGB,
		Overwrite:   settings.Overwrite,
		Client:      client,
	})
	logf("⬇ 已加入本地下载队列：%d 个文件 → %s（%s）", len(items), root, id)
}

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"jobs": s.downloads.Snapshot()},
	})
}
