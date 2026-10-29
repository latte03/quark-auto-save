package server

import (
	"context"
	"errors"
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
	Size    int64
}

// downloadJob is one task's local download batch; it moves through
// queued -> running -> done/failed and keeps counters for the UI.
type downloadJob struct {
	ID          string
	TaskName    string
	Root        string
	Items       []downloadItem
	TotalBytes  int64
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
	pending  int64
	speed    int64
	speedAt  time.Time
	speedWin int64
	lastRead time.Time
	message  string
	started  time.Time
	finished time.Time

	// OnFinish runs once the job reaches a terminal status.
	OnFinish func(status string)

	aborted          atomic.Bool
	paused           atomic.Bool
	canceled         atomic.Bool
	inQueue          atomic.Bool
	finishedNotified atomic.Bool
	inflight         atomic.Int64

	ctx          context.Context
	cancel       context.CancelFunc
	pendingItems []downloadItem
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
	TotalBytes int64  `json:"total_bytes"`
	Speed      int64  `json:"speed"`
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
	job.pendingItems = append([]downloadItem(nil), job.Items...)
	q.jobs[job.ID] = job
	q.order = append(q.order, job.ID)
	q.mu.Unlock()
	job.inQueue.Store(true)
	q.queue <- job
	return job.ID
}

func (q *downloadQueue) get(id string) *downloadJob {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.jobs[id]
}

// ClearFinished forgets jobs that already ended so the queue stops listing
// them. Queued, running and paused jobs stay.
func (q *downloadQueue) ClearFinished() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	kept := q.order[:0]
	removed := 0
	for _, id := range q.order {
		job, ok := q.jobs[id]
		if !ok {
			continue
		}
		switch job.currentStatus() {
		case "queued", "running", "paused":
			kept = append(kept, id)
		default:
			delete(q.jobs, id)
			removed++
		}
	}
	q.order = kept
	return removed
}

// Pause interrupts in-flight transfers and stops new ones; partial data stays
// on disk so Resume continues where it stopped.
func (q *downloadQueue) Pause(id string) bool {
	job := q.get(id)
	if job == nil {
		return false
	}
	switch job.currentStatus() {
	case "queued", "running":
		job.pause()
		return true
	}
	return false
}

func (q *downloadQueue) Resume(id string) bool {
	job := q.get(id)
	if job == nil || !job.isPaused() {
		return false
	}
	job.resume()
	if job.inQueue.CompareAndSwap(false, true) {
		q.queue <- job
	}
	return true
}

func (q *downloadQueue) Cancel(id string) bool {
	job := q.get(id)
	if job == nil {
		return false
	}
	status := job.currentStatus()
	if status == "done" || status == "failed" || status == "canceled" {
		return false
	}
	job.canceled.Store(true)
	job.cancelRun()
	if status != "running" {
		job.markFinished("canceled")
		job.notifyFinish("canceled")
	}
	return true
}

func (q *downloadQueue) worker() {
	for job := range q.queue {
		job.inQueue.Store(false)
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
			if old, ok := q.jobs[oldest]; ok {
				switch old.currentStatus() {
				case "queued", "running", "paused":
				default:
					delete(q.jobs, oldest)
					q.order = q.order[1:]
					continue
				}
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
	j.cancelRun()
}

func (j *downloadJob) addDone(bytes int64, inflight int64) {
	j.mu.Lock()
	j.done++
	j.bytes += bytes
	j.pending -= inflight
	if j.pending < 0 {
		j.pending = 0
	}
	j.mu.Unlock()
}

// addProgress counts bytes arriving in the current transfer and derives a
// one-second-window speed for the UI.
func (j *downloadJob) addProgress(delta int64) {
	if delta <= 0 {
		return
	}
	now := time.Now()
	j.mu.Lock()
	j.pending += delta
	j.lastRead = now
	if j.speedAt.IsZero() {
		j.speedAt = now
	}
	j.speedWin += delta
	if elapsed := now.Sub(j.speedAt); elapsed >= time.Second {
		j.speed = int64(float64(j.speedWin) / elapsed.Seconds())
		j.speedAt = now
		j.speedWin = 0
	}
	j.mu.Unlock()
}

// dropProgress removes bytes from a transfer that failed; the partial file
// stays on disk for the next attempt but is not reported as downloaded.
func (j *downloadJob) dropProgress(inflight int64) {
	if inflight <= 0 {
		return
	}
	j.mu.Lock()
	j.pending -= inflight
	if j.pending < 0 {
		j.pending = 0
	}
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

func (j *downloadJob) isPaused() bool   { return j.paused.Load() }
func (j *downloadJob) isCanceled() bool { return j.canceled.Load() }

func (j *downloadJob) pause() {
	j.paused.Store(true)
	j.setStatus("paused")
	j.cancelRun()
}

func (j *downloadJob) resume() {
	j.paused.Store(false)
	j.mu.Lock()
	j.status = "queued"
	j.finished = time.Time{}
	j.mu.Unlock()
}

func (j *downloadJob) attachContext(ctx context.Context, cancel context.CancelFunc) {
	j.mu.Lock()
	j.ctx = ctx
	j.cancel = cancel
	j.mu.Unlock()
}

func (j *downloadJob) cancelRun() {
	j.mu.Lock()
	if j.cancel != nil {
		j.cancel()
	}
	j.mu.Unlock()
}

func (j *downloadJob) nextPending() (downloadItem, bool) {
	j.mu.Lock()
	defer j.mu.Unlock()
	if len(j.pendingItems) == 0 {
		return downloadItem{}, false
	}
	item := j.pendingItems[0]
	j.pendingItems = j.pendingItems[1:]
	return item, true
}

func (j *downloadJob) requeue(item downloadItem) {
	j.mu.Lock()
	j.pendingItems = append(j.pendingItems, item)
	j.mu.Unlock()
}

func (j *downloadJob) markFinished(status string) {
	j.mu.Lock()
	j.status = status
	if status != "paused" && status != "queued" && status != "running" {
		j.finished = time.Now()
	}
	j.mu.Unlock()
}

func (j *downloadJob) notifyFinish(status string) {
	if j.finishedNotified.Swap(true) {
		return
	}
	if j.OnFinish != nil {
		j.OnFinish(status)
	}
}

func (j *downloadJob) view() downloadJobView {
	j.mu.Lock()
	defer j.mu.Unlock()
	view := downloadJobView{
		ID:         j.ID,
		TaskName:   j.TaskName,
		Root:       j.Root,
		Status:     j.status,
		Total:      len(j.Items),
		Done:       j.done,
		Failed:     j.failed,
		Skipped:    j.skipped,
		Bytes:      j.bytes + j.pending,
		TotalBytes: j.TotalBytes,
		Message:    j.message,
	}
	// Speed is only meaningful while bytes are flowing; a stalled transfer
	// must not keep showing the last sample.
	if j.status == "running" && !j.lastRead.IsZero() && time.Since(j.lastRead) < 5*time.Second {
		view.Speed = j.speed
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
	if job.isCanceled() {
		job.markFinished("canceled")
		job.notifyFinish("canceled")
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	job.attachContext(ctx, cancel)
	defer cancel()

	// A job paused while still queued must not start when the worker reaches it.
	if job.isPaused() {
		job.markFinished("paused")
		return
	}

	job.mu.Lock()
	if job.started.IsZero() {
		job.started = time.Now()
	}
	job.status = "running"
	job.mu.Unlock()

	started := func() bool {
		if message := validateLocalRoot(job.Root); message != "" {
			job.fail(message)
			q.logf("✗ 下载 %s：%s", job.TaskName, message)
			return false
		}
		if check := checkLocalDir(job.Root); !check.OK {
			job.fail(check.Message)
			q.logf("✗ 下载 %s：%s", job.TaskName, check.Message)
			return false
		}
		if message := ensureFreeSpace(job.Root, job.MinFreeGB); message != "" {
			job.fail(message)
			q.logf("✗ 下载 %s：%s", job.TaskName, message)
			return false
		}
		return true
	}
	if !started() {
		job.markFinished("failed")
		job.notifyFinish("failed")
		return
	}

	q.logf("⬇ 开始下载 %s：%d 个文件 → %s", job.TaskName, len(job.Items), job.Root)
	q.execute(job, ctx)
	cancel()

	status := "done"
	switch {
	case job.isCanceled():
		status = "canceled"
	case job.isPaused():
		status = "paused"
	case job.aborted.Load():
		status = "failed"
	}
	job.markFinished(status)
	if status == "paused" {
		q.logf("⏸ 已暂停 %s（进度已保存，可继续）", job.TaskName)
		return
	}
	view := job.view()
	q.logf("⬇ 下载结束 %s：完成 %d，跳过 %d，失败 %d，%s",
		job.TaskName, view.Done, view.Skipped, view.Failed, formatBytes(view.Bytes))
	job.notifyFinish(status)
}

// execute drains the pending queue. Items interrupted by a pause are put back
// so Resume retries them; the file's partial data makes that a continuation.
func (q *downloadQueue) execute(job *downloadJob, ctx context.Context) {
	concurrency := job.Concurrency
	if concurrency < 1 {
		concurrency = 1
	}
	if concurrency > 8 {
		concurrency = 8
	}
	sem := make(chan struct{}, concurrency)
	var wait sync.WaitGroup

	stopped := func() bool {
		return ctx.Err() != nil || job.aborted.Load() || job.isCanceled() || job.isPaused()
	}
	for {
		if stopped() {
			break
		}
		item, ok := job.nextPending()
		if !ok {
			if job.inflight.Load() == 0 {
				break
			}
			time.Sleep(50 * time.Millisecond)
			continue
		}
		sem <- struct{}{}
		if stopped() {
			<-sem
			job.requeue(item)
			break
		}
		job.inflight.Add(1)
		wait.Add(1)
		go func(entry downloadItem) {
			defer wait.Done()
			defer job.inflight.Add(-1)
			defer func() { <-sem }()
			if result := q.transfer(job, ctx, entry); result == transferInterrupted &&
				!job.isCanceled() && !job.isPaused() && !job.aborted.Load() {
				job.requeue(entry)
			}
		}(item)
	}
	wait.Wait()
}

type transferResult int

const (
	transferDone transferResult = iota
	transferSkipped
	transferFailed
	transferInterrupted
)

func (q *downloadQueue) transfer(job *downloadJob, ctx context.Context, item downloadItem) transferResult {
	rel, ok := safeRelPath(item.RelPath)
	if !ok {
		job.addFailed(fmt.Sprintf("非法路径 %q", item.RelPath))
		return transferFailed
	}
	root := filepath.Clean(job.Root)
	target := filepath.Join(root, rel)
	if target != root && !strings.HasPrefix(target, root+string(os.PathSeparator)) {
		job.addFailed(fmt.Sprintf("路径越界 %q", item.RelPath))
		return transferFailed
	}

	if info, err := os.Stat(target); err == nil && !info.IsDir() && !job.Overwrite {
		job.addSkipped()
		return transferSkipped
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		job.addFailed(fmt.Sprintf("创建目录失败: %v", err))
		return transferFailed
	}
	if message := ensureFreeSpace(job.Root, job.MinFreeGB); message != "" {
		job.fail(message)
		return transferFailed
	}
	if ctx.Err() != nil {
		return transferInterrupted
	}

	links, cookies, err := job.Client.Download([]string{item.FID})
	if err != nil {
		job.addFailed(fmt.Sprintf("获取下载地址失败: %v", err))
		q.logf("✗ %s：获取下载地址失败: %v", rel, err)
		return transferFailed
	}
	if len(links) == 0 || links[0].DownloadURL == "" {
		job.addFailed("未返回下载地址")
		return transferFailed
	}

	url := links[0].DownloadURL
	ua := job.Client.UserAgent()
	var fileBytes atomic.Int64
	onProgress := func(delta int64) {
		fileBytes.Add(delta)
		job.addProgress(delta)
	}

	// Quark throttles each connection to roughly 100KB/s, so large files are
	// split into parallel ranges; the total scales with the stream count.
	var size int64
	if item.Size >= segmentThreshold {
		err = q.fetchSegmented(ctx, url, cookies, ua, target, item.Size, onProgress)
		if errors.Is(err, errRangeUnsupported) {
			removePartialFiles(target)
			size, err = q.fetchFile(ctx, url, cookies, ua, target, onProgress)
		} else if err == nil {
			size = item.Size
		}
	} else {
		size, err = q.fetchFile(ctx, url, cookies, ua, target, onProgress)
	}
	if err != nil {
		job.dropProgress(fileBytes.Load())
		if ctx.Err() != nil || job.isPaused() || job.isCanceled() || job.aborted.Load() {
			return transferInterrupted
		}
		job.addFailed(fmt.Sprintf("%s: %v", rel, err))
		q.logf("✗ %s：%v", rel, err)
		return transferFailed
	}
	job.addDone(size, fileBytes.Load())
	q.logf("✓ %s（%s）", rel, formatBytes(size))
	return transferDone
}

// fetchFile downloads into <target>.part with Range resume and renames on
// success. Interrupted transfers keep the .part file for the next attempt.
// onProgress receives the bytes written by this attempt (resumed bytes are
// not counted).
func (q *downloadQueue) fetchFile(ctx context.Context, url, cookie, ua, target string, onProgress func(int64)) (int64, error) {
	part := target + ".part"
	var offset int64
	if info, err := os.Stat(part); err == nil && info.Size() > 0 {
		offset = info.Size()
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, fmt.Errorf("构造请求失败: %w", err)
	}
	if cookie != "" {
		request.Header.Set("Cookie", cookie)
	}
	if ua != "" {
		request.Header.Set("User-Agent", ua)
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
	body := io.Reader(response.Body)
	if onProgress != nil {
		body = &progressReader{reader: response.Body, report: onProgress}
	}
	written, copyErr := io.CopyBuffer(file, body, make([]byte, 256*1024))
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

// progressReader reports bytes as the copy loop consumes them.
type progressReader struct {
	reader io.Reader
	report func(int64)
}

func (p *progressReader) Read(buffer []byte) (int, error) {
	n, err := p.reader.Read(buffer)
	if n > 0 {
		p.report(int64(n))
	}
	return n, err
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
		items = append(items, downloadItem{RelPath: rel, FID: node.Data.FID, Size: node.Data.Size})
	})
	return items
}

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
	var walk func(fid, prefix string, depth int)
	walk = func(fid, prefix string, depth int) {
		if fid == "" || depth > 16 || visited[fid] {
			return
		}
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
	return items
}

// localFileExists mirrors the transfer step: an existing file is skipped
// unless overwrite is on.
func localFileExists(root, rel string) bool {
	clean, ok := safeRelPath(rel)
	if !ok {
		return false
	}
	info, err := os.Stat(filepath.Join(filepath.Clean(root), clean))
	return err == nil && !info.IsDir()
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

func (s *Server) handleDownloads(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"jobs": s.downloads.Snapshot()},
	})
}

func (s *Server) downloadControl(w http.ResponseWriter, r *http.Request, action func(string) bool) {
	id := r.PathValue("id")
	if !action(id) {
		writeError(w, http.StatusNotFound, "下载任务不存在或当前状态不支持该操作")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handlePauseDownload(w http.ResponseWriter, r *http.Request) {
	s.downloadControl(w, r, s.downloads.Pause)
}

func (s *Server) handleResumeDownload(w http.ResponseWriter, r *http.Request) {
	s.downloadControl(w, r, s.downloads.Resume)
}

func (s *Server) handleCancelDownload(w http.ResponseWriter, r *http.Request) {
	s.downloadControl(w, r, s.downloads.Cancel)
}

func (s *Server) handleClearDownloads(w http.ResponseWriter, r *http.Request) {
	removed := s.downloads.ClearFinished()
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"removed": removed},
	})
}
