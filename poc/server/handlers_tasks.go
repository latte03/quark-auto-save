package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"qas-poc/plugins"
	"qas-poc/quark"
)

// ---------- tasks ----------

func (s *Server) pluginTaskDefaults() map[string]map[string]any {
	out := map[string]map[string]any{}
	for _, plugin := range plugins.Build(nil, plugins.Deps{}) {
		if defaults := plugin.TaskDefaults(); defaults != nil {
			out[plugin.Name()] = defaults
		}
	}
	return out
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	snapshot := s.store.Snapshot()
	now := time.Now()
	stats := s.runs.TaskStats()
	views := make([]map[string]any, 0, len(snapshot.Tasks))
	for index, task := range snapshot.Tasks {
		views = append(views, taskView(index, task, now, snapshot.Crontab, stats, snapshot.Magic))
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"tasks":                views,
			"task_plugins_default": s.pluginTaskDefaults(),
			"plugin_status":        plugins.Configured(snapshot.Plugins),
			"version":              s.opts.Version,
			"crontab":              snapshot.Crontab,
			"running":              s.runs.ActiveID(),
		},
	})
}

func (s *Server) handleCreateTask(w http.ResponseWriter, r *http.Request) {
	task, err := readTask(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateTask(task); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	task.OnceDoneAt = ""
	if err := s.store.MutateTasks(func(tasks []*quark.Task) ([]*quark.Task, error) {
		return append(tasks, task), nil
	}); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.logger.Printf("新增任务: %s", task.TaskName)
	s.respondTasks(w)
}

func (s *Server) handleUpdateTask(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "任务序号无效")
		return
	}
	task, err := readTask(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := validateTask(task); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	err = s.store.MutateTasks(func(tasks []*quark.Task) ([]*quark.Task, error) {
		if index < 0 || index >= len(tasks) {
			return nil, errors.New("任务不存在")
		}
		previous := tasks[index]
		// Editing a finished one-shot task gives it a new run, so the
		// completion mark only survives an unchanged save.
		if task.IsOnce() && sameTaskContent(previous, task) {
			task.OnceDoneAt = previous.OnceDoneAt
		} else {
			task.OnceDoneAt = ""
		}
		tasks[index] = task
		return tasks, nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Printf("更新任务: %s", task.TaskName)
	s.respondTasks(w)
}

func (s *Server) handleDeleteTask(w http.ResponseWriter, r *http.Request) {
	index, err := strconv.Atoi(r.PathValue("index"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "任务序号无效")
		return
	}
	err = s.store.MutateTasks(func(tasks []*quark.Task) ([]*quark.Task, error) {
		if index < 0 || index >= len(tasks) {
			return nil, errors.New("任务不存在")
		}
		name := tasks[index].TaskName
		s.logger.Printf("删除任务: %s", name)
		return append(tasks[:index], tasks[index+1:]...), nil
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.respondTasks(w)
}

func (s *Server) respondTasks(w http.ResponseWriter) {
	if err := s.scheduler.Reload(); err != nil {
		s.logger.Printf("调度器重载失败: %v", err)
	}
	snapshot := s.store.Snapshot()
	now := time.Now()
	stats := s.runs.TaskStats()
	views := make([]map[string]any, 0, len(snapshot.Tasks))
	for index, task := range snapshot.Tasks {
		views = append(views, taskView(index, task, now, snapshot.Crontab, stats, snapshot.Magic))
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": map[string]any{"tasks": views}})
}

// sameTaskContent compares every field except the one-shot completion mark.
func sameTaskContent(a, b *quark.Task) bool {
	left, right := cloneTask(a), cloneTask(b)
	left.OnceDoneAt = ""
	right.OnceDoneAt = ""
	return reflect.DeepEqual(taskFields(left), taskFields(right))
}

func taskFields(task *quark.Task) any {
	raw, err := json.Marshal(task)
	if err != nil {
		return nil
	}
	var out any
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil
	}
	return out
}

func readTask(r *http.Request) (*quark.Task, error) {
	body, err := io.ReadAll(io.LimitReader(r.Body, 2<<20))
	if err != nil {
		return nil, err
	}
	var envelope struct {
		Task json.RawMessage `json:"task"`
	}
	raw := body
	if err := json.Unmarshal(body, &envelope); err == nil && len(envelope.Task) > 0 {
		raw = envelope.Task
	}
	var task quark.Task
	if err := json.Unmarshal(raw, &task); err != nil {
		return nil, fmt.Errorf("任务格式错误: %w", err)
	}
	task.RemoveFields("index", "status", "latest", "next_run", "next_run_at", "progress")
	task.SavePath = quark.NormalizePath(task.SavePath)
	task.NormalizeMode()
	return &task, nil
}

func validateTask(task *quark.Task) error {
	if strings.TrimSpace(task.TaskName) == "" {
		return errors.New("任务名称不能为空")
	}
	if task.ShareURL == "" {
		return errors.New("分享链接不能为空")
	}
	if _, err := quark.ExtractShareURL(task.ShareURL); err != nil {
		return errors.New("分享链接格式无效")
	}
	if strings.TrimSpace(task.SavePath) == "" || task.SavePath == "/" {
		return errors.New("保存路径不能为空")
	}
	for _, value := range task.RunTime {
		if _, ok := parseClock(value); !ok {
			return fmt.Errorf("运行时间格式无效: %s（应为 HH:MM）", value)
		}
	}
	return nil
}

func taskView(index int, task *quark.Task, now time.Time, crontab string, stats map[string]TaskStats, rules []quark.Rule) map[string]any {
	raw, _ := json.Marshal(task)
	var view map[string]any
	_ = json.Unmarshal(raw, &view)
	// The editor expects these keys even when the config omits them. A task
	// without runweek runs every day; show that explicitly so saving the form
	// does not turn it into a task with an empty (never-run) schedule.
	if _, ok := view["addition"]; !ok {
		view["addition"] = map[string]any{}
	}
	if _, ok := view["runweek"]; !ok {
		view["runweek"] = []int{1, 2, 3, 4, 5, 6, 7}
	}
	// Same idea for scheduling: show the effective times so editing a task
	// saved under the old global crontab keeps its cadence.
	if times := taskTimes(task, crontab); len(times) > 0 {
		view["runtime"] = times
	}
	if task.MagicID != "" {
		for _, rule := range rules {
			if rule.ID == task.MagicID {
				view["magic_name"] = rule.Name
				break
			}
		}
	}
	view["index"] = index
	view["status"] = statusOf(task, now)
	nextLabel, nextAt := nextRunInfo(task, crontab, now)
	view["next_run"] = nextLabel
	if !nextAt.IsZero() {
		view["next_run_at"] = nextAt.Format(time.RFC3339)
	}
	saved := 0
	latest := "—"
	if stat, ok := stats[task.TaskName]; ok && stat.Saved > 0 {
		saved = stat.Saved
		latest = fmt.Sprintf("+%d", stat.Saved)
	}
	view["latest"] = latest
	view["progress"] = map[string]any{
		"saved":    saved,
		"expected": 0,
		"days":     runweekDays(task),
	}
	return view
}

func statusOf(task *quark.Task, now time.Time) string {
	if task.OnceFinished() {
		return "once_done"
	}
	if task.ShareURLBan != "" {
		return "banned"
	}
	if task.IsOnce() {
		return "once"
	}
	if task.EndDate != "" {
		end, err := time.ParseInLocation("2006-01-02", task.EndDate, now.Location())
		if err == nil {
			today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
			if today.After(end) {
				return "done"
			}
		}
	}
	if task.HasRunWeek() {
		weekday := int(now.Weekday())
		if weekday == 0 {
			weekday = 7
		}
		found := false
		for _, day := range task.RunWeek {
			if day == weekday {
				found = true
			}
		}
		if !found {
			return "waiting"
		}
	}
	return "tracking"
}

func runweekDays(task *quark.Task) []int {
	days := make([]int, 7)
	for _, day := range task.RunWeek {
		if day >= 1 && day <= 7 {
			days[day-1] = 1
		}
	}
	return days
}

var weekdayLabels = []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}

// nextRunInfo renders the human label and the instant behind it, so the console
// can sort by absolute time instead of re-parsing localized weekday text.
func nextRunInfo(task *quark.Task, crontab string, now time.Time) (string, time.Time) {
	if task.IsOnce() {
		return "—", time.Time{}
	}
	var next time.Time
	if times := nextTimesForTask(task, crontab, now, 1); len(times) > 0 {
		next = times[0]
	} else {
		// A legacy crontab that cannot be expressed as clock times still
		// drives tasks without runtime; keep its preview accurate.
		schedule, err := cron.ParseStandard(strings.TrimSpace(crontab))
		if err != nil {
			return "—", time.Time{}
		}
		cursor := now
		for i := 0; i < 100 && next.IsZero(); i++ {
			cursor = schedule.Next(cursor)
			if cursor.IsZero() {
				return "—", time.Time{}
			}
			if scheduled, err := task.IsScheduled(cursor); err == nil && scheduled {
				next = cursor
			}
		}
		if next.IsZero() {
			return "—", time.Time{}
		}
	}
	label := next.Format("15:04")
	if next.Year() == now.Year() && next.YearDay() == now.YearDay() {
		return "今天 " + label, next
	}
	return weekdayLabels[int(next.Weekday())] + " " + label, next
}

func nextRunLabel(task *quark.Task, crontab string, now time.Time) string {
	label, _ := nextRunInfo(task, crontab, now)
	return label
}
