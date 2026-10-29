package server

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

var ErrRunBusy = errors.New("已有任务在运行")

type TaskStats struct {
	Saved int
	At    time.Time
}

type RunView struct {
	ID         string         `json:"id"`
	Label      string         `json:"label"`
	StartedAt  time.Time      `json:"started_at"`
	FinishedAt *time.Time     `json:"finished_at,omitempty"`
	Done       bool           `json:"done"`
	Error      string         `json:"error,omitempty"`
	SavedTotal int            `json:"saved_total"`
	Tasks      map[string]int `json:"tasks,omitempty"`
	Dry        bool           `json:"dry,omitempty"`
}

type Run struct {
	ID        string
	Label     string
	Write     bool
	StartedAt time.Time

	mu         sync.Mutex
	lines      []string
	done       bool
	errMsg     string
	finishedAt time.Time
	saved      int
	stats      map[string]int
	subs       map[chan struct{}]struct{}

	writer *runWriter
}

type runWriter struct {
	run  *Run
	mu   sync.Mutex
	tail string
}

func (w *runWriter) Write(data []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	text := w.tail + string(data)
	parts := strings.Split(text, "\n")
	w.tail = parts[len(parts)-1]
	for _, line := range parts[:len(parts)-1] {
		w.run.appendLine(strings.TrimSuffix(line, "\r"))
	}
	return len(data), nil
}

func (w *runWriter) flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.tail != "" {
		w.run.appendLine(w.tail)
		w.tail = ""
	}
}

func (r *Run) Writer() io.Writer { return r.writer }

func (r *Run) Logf(format string, args ...any) {
	r.appendLine(fmt.Sprintf(format, args...))
}

func (r *Run) appendLine(line string) {
	r.mu.Lock()
	r.lines = append(r.lines, line)
	for ch := range r.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
	r.mu.Unlock()
}

func (r *Run) finish(err error) {
	r.finalize(err)
	r.publish()
}

// finalize records the outcome but keeps the run open so finish hooks (log
// persistence) complete before observers see done.
func (r *Run) finalize(err error) {
	if r.writer != nil {
		r.writer.flush()
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if err != nil {
		r.errMsg = err.Error()
		r.lines = append(r.lines, "❌ 运行失败: "+err.Error())
	}
	r.finishedAt = time.Now()
}

func (r *Run) publish() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.done = true
	for ch := range r.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

// AllLines returns a copy of every line collected so far.
func (r *Run) AllLines() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.lines...)
}

func (r *Run) LinesFrom(offset int) ([]string, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if offset >= len(r.lines) {
		return nil, r.done
	}
	out := append([]string(nil), r.lines[offset:]...)
	return out, r.done
}

func (r *Run) Subscribe() (int, <-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	r.mu.Lock()
	if r.subs == nil {
		r.subs = map[chan struct{}]struct{}{}
	}
	r.subs[ch] = struct{}{}
	offset := len(r.lines)
	r.mu.Unlock()
	cancel := func() {
		r.mu.Lock()
		delete(r.subs, ch)
		r.mu.Unlock()
	}
	return offset, ch, cancel
}

func (r *Run) View() RunView {
	r.mu.Lock()
	defer r.mu.Unlock()
	view := RunView{
		ID:         r.ID,
		Label:      r.Label,
		StartedAt:  r.StartedAt,
		Done:       r.done,
		Error:      r.errMsg,
		SavedTotal: r.saved,
		Dry:        !r.Write,
	}
	if !r.finishedAt.IsZero() {
		finished := r.finishedAt
		view.FinishedAt = &finished
	}
	if len(r.stats) > 0 {
		view.Tasks = make(map[string]int, len(r.stats))
		for key, value := range r.stats {
			view.Tasks[key] = value
		}
	}
	return view
}

type RunManager struct {
	mu       sync.Mutex
	runs     map[string]*Run
	order    []string
	active   *Run
	stats    map[string]TaskStats
	OnFinish func(*Run)
}

func NewRunManager() *RunManager {
	return &RunManager{
		runs:  map[string]*Run{},
		stats: map[string]TaskStats{},
	}
}

func newRunID() string {
	buffer := make([]byte, 3)
	_, _ = rand.Read(buffer)
	return time.Now().Format("20060102-150405") + "-" + hex.EncodeToString(buffer)
}

// Start launches fn in a goroutine; only one run may be active at a time.
func (m *RunManager) Start(label string, write bool, fn func(*Run) error) (*Run, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil {
		m.active.mu.Lock()
		active := !m.active.done
		m.active.mu.Unlock()
		if active {
			return nil, ErrRunBusy
		}
	}
	run := &Run{
		ID:        newRunID(),
		Label:     label,
		Write:     write,
		StartedAt: time.Now(),
		stats:     map[string]int{},
	}
	run.writer = &runWriter{run: run}
	m.runs[run.ID] = run
	m.order = append(m.order, run.ID)
	m.active = run
	for len(m.order) > 10 {
		delete(m.runs, m.order[0])
		m.order = m.order[1:]
	}
	go func() {
		err := fn(run)
		run.mu.Lock()
		run.saved = sumStats(run.stats)
		run.mu.Unlock()
		run.finalize(err)
		m.mu.Lock()
		hook := m.OnFinish
		for name, saved := range run.stats {
			m.stats[name] = TaskStats{Saved: saved, At: time.Now()}
		}
		m.mu.Unlock()
		if hook != nil {
			hook(run)
		}
		run.publish()
	}()
	return run, nil
}

func sumStats(stats map[string]int) int {
	total := 0
	for _, value := range stats {
		total += value
	}
	return total
}

func (m *RunManager) Get(id string) *Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.runs[id]
}

// Latest returns the active run (dry or not) so a live dry run is still
// watchable; once it finishes the history falls back to the last real run.
// All returns the in-memory runs, newest first.
func (m *RunManager) All() []*Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*Run, 0, len(m.order))
	for index := len(m.order) - 1; index >= 0; index-- {
		if run := m.runs[m.order[index]]; run != nil {
			out = append(out, run)
		}
	}
	return out
}

func (m *RunManager) Latest() *Run {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active != nil {
		m.active.mu.Lock()
		running := !m.active.done
		m.active.mu.Unlock()
		if running {
			return m.active
		}
	}
	for index := len(m.order) - 1; index >= 0; index-- {
		run := m.runs[m.order[index]]
		if run != nil && run.Write {
			return run
		}
	}
	return nil
}

func (m *RunManager) ActiveID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.active == nil {
		return ""
	}
	m.active.mu.Lock()
	defer m.active.mu.Unlock()
	if m.active.done {
		return ""
	}
	return m.active.ID
}

func (m *RunManager) TaskStats() map[string]TaskStats {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make(map[string]TaskStats, len(m.stats))
	for key, value := range m.stats {
		out[key] = value
	}
	return out
}

func (r *Run) setStats(stats map[string]int) {
	r.mu.Lock()
	r.stats = stats
	r.mu.Unlock()
}

// Scheduler registers one cron entry per distinct task clock time. A fire
// only starts the tasks whose time, weekday and end date all match.
type Scheduler struct {
	mu      sync.Mutex
	current *cron.Cron
	server  *Server
}

func NewScheduler(server *Server) *Scheduler { return &Scheduler{server: server} }

func ValidCrontab(spec string) bool {
	_, err := cron.ParseStandard(spec)
	return err == nil
}

func (s *Scheduler) Reload() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.current != nil {
		s.current.Stop()
		s.current = nil
	}
	snapshot := s.server.store.Snapshot()
	if len(snapshot.Cookies) == 0 {
		s.server.logger.Printf("定时任务未启用：cookie 未配置")
		return nil
	}
	clocks := map[string]bool{}
	legacy := false
	for _, task := range snapshot.Tasks {
		if task.IsOnce() {
			continue
		}
		times := taskTimes(task, snapshot.Crontab)
		if len(times) == 0 {
			legacy = true
			continue
		}
		for _, clock := range times {
			clocks[clock] = true
		}
	}
	legacySpec := ""
	if legacy && ValidCrontab(snapshot.Crontab) {
		legacySpec = snapshot.Crontab
	}
	if len(clocks) == 0 && legacySpec == "" {
		s.server.logger.Printf("调度器未启用：没有任务设置运行时间")
		return nil
	}

	runner := cron.New(cron.WithChain(cron.SkipIfStillRunning(cron.DiscardLogger)))
	for clock := range clocks {
		at := strings.Split(clock, ":")
		spec := fmt.Sprintf("%d %d * * *", atoi(at[1]), atoi(at[0]))
		if _, err := runner.AddFunc(spec, func() {
			current := s.server.store.Snapshot()
			indices := dueTaskIndices(current.Tasks, current.Crontab, time.Now())
			if len(indices) == 0 {
				return
			}
			if _, err := s.server.startRun(runSpec{Label: "定时任务", Indices: indices, Write: true}); err != nil {
				s.server.logger.Printf("定时运行跳过: %v", err)
			}
		}); err != nil {
			return err
		}
	}
	if legacySpec != "" {
		if _, err := runner.AddFunc(legacySpec, func() {
			current := s.server.store.Snapshot()
			indices := legacyTaskIndices(current.Tasks, current.Crontab, time.Now())
			if len(indices) == 0 {
				return
			}
			if _, err := s.server.startRun(runSpec{Label: "定时任务", Indices: indices, Write: true}); err != nil {
				s.server.logger.Printf("定时运行跳过: %v", err)
			}
		}); err != nil {
			return err
		}
	}
	runner.Start()
	s.current = runner
	s.server.logger.Printf("调度器已加载: %d 个时刻", len(clocks))
	if legacySpec != "" {
		s.server.logger.Printf("旧 crontab 作为兜底: %s", legacySpec)
	}
	return nil
}

func atoi(value string) int {
	out, _ := strconv.Atoi(value)
	return out
}
