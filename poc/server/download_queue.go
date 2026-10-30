package server

import (
	"context"
	"fmt"
	"log"
	"net/http"
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

	// processing is guarded by mu; it is true while the queue worker owns the
	// job, so Resume can tell "still winding down" from "queued for a new run".
	processing bool

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
	if job == nil || !job.isPaused() || job.isCanceled() {
		return false
	}
	// The worker may still be winding down after the pause; it observes the
	// cleared flag itself (see settle) and continues the same run, so only
	// enqueue when no worker owns the job.
	if job.clearPause() {
		return true
	}
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

// clearPause lifts the paused flag and reports whether the queue worker still
// owns the job; when it does, the worker continues instead of a second run
// being queued.
func (j *downloadJob) clearPause() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.paused.Store(false)
	j.status = "queued"
	j.finished = time.Time{}
	return j.processing
}

// settle records how one execute pass ended. It reports "resume" when a pause
// was lifted before the pass wound down, so the same worker keeps going.
func (j *downloadJob) settle() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	status := "done"
	switch {
	case j.canceled.Load():
		status = "canceled"
	case j.paused.Load():
		status = "paused"
	case j.aborted.Load():
		status = "failed"
	case len(j.pendingItems) > 0:
		status = "resume"
	}
	if status != "resume" {
		j.status = status
		if status != "paused" {
			j.finished = time.Now()
		}
		j.processing = false
	}
	return status
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
	j.processing = false
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

// claimStart marks the job as owned by the worker and reports whether it must
// bail out because it was paused while still queued. The pause check and the
// processing handoff share one critical section: a concurrent Resume either
// sees processing == false (and enqueues) or observes the cleared pause.
func (j *downloadJob) claimStart() bool {
	j.mu.Lock()
	defer j.mu.Unlock()
	j.processing = true
	if j.started.IsZero() {
		j.started = time.Now()
	}
	// Canceled jobs fall through so run() can record that terminal status.
	if !j.paused.Load() || j.canceled.Load() {
		return false
	}
	j.status = "paused"
	j.processing = false
	return true
}

func (q *downloadQueue) run(job *downloadJob) {
	if job.claimStart() {
		return
	}
	if job.isCanceled() {
		job.markFinished("canceled")
		job.notifyFinish("canceled")
		return
	}

	job.mu.Lock()
	job.status = "running"
	job.mu.Unlock()

	if message := q.prepare(job); message != "" {
		job.fail(message)
		q.logf("✗ 下载 %s：%s", job.TaskName, message)
		job.markFinished("failed")
		job.notifyFinish("failed")
		return
	}

	q.logf("⬇ 开始下载 %s：%d 个文件 → %s", job.TaskName, len(job.Items), job.Root)
	for {
		ctx, cancel := context.WithCancel(context.Background())
		job.attachContext(ctx, cancel)
		// A resume during the previous pass left the status at "queued"; the
		// job is executing again, so flip it back for the UI.
		job.mu.Lock()
		job.status = "running"
		job.mu.Unlock()
		q.execute(job, ctx)
		cancel()

		switch outcome := job.settle(); outcome {
		case "resume":
			continue
		case "paused":
			q.logf("⏸ 已暂停 %s（进度已保存，可继续）", job.TaskName)
			return
		case "done":
			view := job.view()
			q.logf("⬇ 下载结束 %s：完成 %d，跳过 %d，失败 %d，%s",
				job.TaskName, view.Done, view.Skipped, view.Failed, formatBytes(view.Bytes))
			job.notifyFinish("done")
			return
		default:
			job.notifyFinish(outcome)
			return
		}
	}
}

// prepare validates the job's target before any transfer starts.
func (q *downloadQueue) prepare(job *downloadJob) string {
	if message := validateLocalRoot(job.Root); message != "" {
		return message
	}
	if check := checkLocalDir(job.Root); !check.OK {
		return check.Message
	}
	if message := ensureFreeSpace(job.Root, job.MinFreeGB); message != "" {
		return message
	}
	return ""
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
				!job.isCanceled() && !job.aborted.Load() {
				// Paused transfers go back to the queue too: Resume continues
				// them from the .part file instead of dropping the file.
				job.requeue(entry)
			}
		}(item)
	}
	wait.Wait()
}
