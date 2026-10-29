package server

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/robfig/cron/v3"

	"qas-poc/plugins"
	"qas-poc/quark"
)

const sessionCookie = "QUARK_AUTO_SAVE_SESSION"

type Options struct {
	ConfigPath string
	DataDir    string
	StaticDir  string
	Version    string
	Logger     *log.Logger
	HTTP       *http.Client
}

type Server struct {
	store     *Store
	runs      *RunManager
	scheduler *Scheduler
	logger    *log.Logger
	http      *http.Client
	opts      Options
	mux       *http.ServeMux
	newClient func(string) *quark.Client
	downloads *downloadQueue
	runLogs   *runLogStore
	qr        *qrLogin
}

func New(opts Options) (*Server, error) {
	store, notice, err := OpenStore(opts.ConfigPath)
	if err != nil {
		return nil, err
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(os.Stderr, "[qas] ", log.LstdFlags)
	}
	client := opts.HTTP
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	server := &Server{
		store:     store,
		runs:      NewRunManager(),
		logger:    logger,
		http:      client,
		opts:      opts,
		newClient: quark.New,
	}
	server.downloads = newDownloadQueue(logger)
	server.qr = newQRLogin(client, logger)
	dataDir := opts.DataDir
	if dataDir == "" {
		dataDir = os.Getenv("TRIM_PKGVAR")
	}
	if dataDir == "" {
		dataDir = filepath.Dir(opts.ConfigPath)
	}
	server.runLogs = newRunLogStore(filepath.Join(dataDir, "runs"))
	server.runs.OnFinish = func(run *Run) {
		if !run.Write {
			return
		}
		if err := server.runLogs.save(run, store.RunLogKeep()); err != nil {
			logger.Printf("保存运行日志失败: %v", err)
		}
	}
	server.scheduler = NewScheduler(server)
	if err := server.scheduler.Reload(); err != nil {
		logger.Printf("调度器加载失败: %v", err)
	}
	if notice.GeneratedPassword != "" {
		logger.Printf("已生成 WebUI 登录密码: %s（用户 %s，可在设置中查看）", notice.GeneratedPassword, store.Snapshot().Username)
	}
	server.routes()
	return server, nil
}

func (s *Server) Handler() http.Handler { return s.mux }

func (s *Server) routes() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/quark/qr", s.auth(s.handleStartQuarkQR))
	mux.HandleFunc("GET /api/quark/qr/{id}", s.auth(s.handleQuarkQRStatus))
	mux.HandleFunc("GET /api/quark/account", s.auth(s.handleQuarkAccount))
	mux.HandleFunc("GET /api/session", s.handleSession)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/version", s.handleVersion)

	mux.HandleFunc("GET /api/config", s.auth(s.handleGetConfig))
	mux.HandleFunc("PUT /api/config", s.auth(s.handleUpdateConfig))
	mux.HandleFunc("POST /api/magic/rename", s.auth(s.handleRenameMagic))
	mux.HandleFunc("GET /api/tasks", s.auth(s.handleTasks))
	mux.HandleFunc("POST /api/tasks", s.auth(s.handleCreateTask))
	mux.HandleFunc("PUT /api/tasks/{index}", s.auth(s.handleUpdateTask))
	mux.HandleFunc("DELETE /api/tasks/{index}", s.auth(s.handleDeleteTask))

	mux.HandleFunc("POST /api/run", s.auth(s.handleRun))
	mux.HandleFunc("POST /api/sign", s.auth(s.handleSign))
	mux.HandleFunc("POST /api/notify/test", s.auth(s.handleNotifyTest))
	mux.HandleFunc("GET /api/runs", s.auth(s.handleRuns))
	mux.HandleFunc("GET /api/runs/latest", s.auth(s.handleLatestRun))
	mux.HandleFunc("GET /api/runs/{id}", s.auth(s.handleGetRun))
	mux.HandleFunc("GET /api/runs/{id}/log", s.auth(s.handleRunLog))
	mux.HandleFunc("GET /api/runs/{id}/stream", s.auth(s.handleRunStream))
	mux.HandleFunc("GET /api/status", s.auth(s.handleStatus))

	mux.HandleFunc("GET /api/browse", s.auth(s.handleBrowse))
	mux.HandleFunc("GET /api/sources/detect", s.auth(s.handleDetectSources))
	mux.HandleFunc("POST /api/local/validate", s.auth(s.handleLocalValidate))
	mux.HandleFunc("GET /api/local/authorized", s.auth(s.handleLocalAuthorized))
	mux.HandleFunc("GET /api/local/browse", s.auth(s.handleLocalBrowse))
	mux.HandleFunc("GET /api/downloads", s.auth(s.handleDownloads))
	mux.HandleFunc("POST /api/downloads/{id}/pause", s.auth(s.handlePauseDownload))
	mux.HandleFunc("POST /api/downloads/{id}/resume", s.auth(s.handleResumeDownload))
	mux.HandleFunc("POST /api/downloads/{id}/cancel", s.auth(s.handleCancelDownload))
	mux.HandleFunc("POST /api/files/delete", s.auth(s.handleDeleteFile))
	mux.HandleFunc("POST /api/files/rename", s.auth(s.handleRenameFile))
	mux.HandleFunc("POST /api/files/mkdir", s.auth(s.handleMkdir))
	mux.HandleFunc("POST /api/share/validate", s.auth(s.handleValidateShare))
	mux.HandleFunc("POST /api/share/detail", s.auth(s.handleShareDetail))
	mux.HandleFunc("POST /api/rename/preview", s.auth(s.handleRenamePreview))
	mux.HandleFunc("GET /api/search", s.auth(s.handleSearch))

	mux.HandleFunc("/", s.handleStatic)
	s.mux = mux
}

// ---------- auth ----------

// APIKey derives the bearer token from the WebUI credentials.
func APIKey(username, password string) string {
	sum := md5.Sum([]byte(fmt.Sprintf("token%s%s+-*/", username, password)))
	return hex.EncodeToString(sum[:])[8:24]
}

func (s *Server) apiToken(username, password string) string {
	return APIKey(username, password)
}

func (s *Server) sessionValue(username, password string) string {
	expiry := time.Now().Add(31 * 24 * time.Hour).Unix()
	payload := strconv.FormatInt(expiry, 10)
	mac := hmac.New(sha256.New, []byte("qas-session-v1|"+username+"|"+password))
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) validSession(value, username, password string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	mac := hmac.New(sha256.New, []byte("qas-session-v1|"+username+"|"+password))
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(parts[1]))
}

func (s *Server) authenticated(r *http.Request) bool {
	username, password := s.store.WebUI()
	if username == "" || password == "" {
		return false
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return subtle.ConstantTimeCompare([]byte(token), []byte(s.apiToken(username, password))) == 1
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, "Bearer ")), []byte(s.apiToken(username, password))) == 1 {
			return true
		}
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.validSession(cookie.Value, username, password)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticated(r) {
			writeError(w, http.StatusUnauthorized, "未登录")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	username, _ := s.store.WebUI()
	auth := s.authenticated(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"authenticated": auth,
		"username":      username,
		"version":       s.opts.Version,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	username, password := s.store.WebUI()
	userOK := subtle.ConstantTimeCompare([]byte(request.Username), []byte(username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(request.Password), []byte(password)) == 1
	if username == "" || password == "" || !userOK || !passOK {
		s.logger.Printf("WebUI 登录失败: 用户=%s 来自=%s", request.Username, r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.sessionValue(username, password),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   31 * 24 * 3600,
	})
	s.logger.Printf("WebUI 登录成功: 用户=%s 来自=%s", username, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"token":    s.apiToken(username, password),
		"username": username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "version": s.opts.Version})
}

// ---------- config ----------

func (s *Server) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	username, password := s.store.WebUI()
	view := s.store.ConfigView(s.opts.Version, s.apiToken(username, password))
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": view})
}

func (s *Server) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	var partial map[string]json.RawMessage
	if err := decodeJSON(r, &partial); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := s.store.Update(partial); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.scheduler.Reload(); err != nil {
		s.logger.Printf("调度器重载失败: %v", err)
	}
	username, password := s.store.WebUI()
	view := s.store.ConfigView(s.opts.Version, s.apiToken(username, password))
	s.logger.Printf("配置已保存")
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": view})
}

func (s *Server) handleRenameMagic(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	name := strings.TrimSpace(request.Name)
	if request.ID == "" || name == "" {
		writeError(w, http.StatusBadRequest, "规则名称不能为空")
		return
	}
	if err := s.store.RenameMagic(request.ID, name); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	s.logger.Printf("重命名魔法规则 %s → %s", request.ID, name)
	s.respondTasks(w)
}

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
	task.RemoveFields("index", "status", "latest", "next_run", "progress")
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
	view["next_run"] = nextRunLabel(task, crontab, now)
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

func nextRunLabel(task *quark.Task, crontab string, now time.Time) string {
	if task.IsOnce() {
		return "—"
	}
	var next time.Time
	if times := nextTimesForTask(task, crontab, now, 1); len(times) > 0 {
		next = times[0]
	} else {
		// A legacy crontab that cannot be expressed as clock times still
		// drives tasks without runtime; keep its preview accurate.
		schedule, err := cron.ParseStandard(strings.TrimSpace(crontab))
		if err != nil {
			return "—"
		}
		cursor := now
		for i := 0; i < 100 && next.IsZero(); i++ {
			cursor = schedule.Next(cursor)
			if cursor.IsZero() {
				return "—"
			}
			if scheduled, err := task.IsScheduled(cursor); err == nil && scheduled {
				next = cursor
			}
		}
		if next.IsZero() {
			return "—"
		}
	}
	label := next.Format("15:04")
	if next.Year() == now.Year() && next.YearDay() == now.YearDay() {
		return "今天 " + label
	}
	return weekdayLabels[int(next.Weekday())] + " " + label
}

// ---------- run ----------

func (s *Server) handleRun(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Tasks []int           `json:"tasks"`
		Write *bool           `json:"write"`
		Task  json.RawMessage `json:"task"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	write := request.Write != nil && *request.Write
	snapshot := s.store.Snapshot()

	var inline *quark.Task
	if raw := strings.TrimSpace(string(request.Task)); raw != "" && raw != "null" {
		if len(request.Tasks) > 0 {
			writeError(w, http.StatusBadRequest, "临时任务与任务序号不能同时指定")
			return
		}
		var task quark.Task
		if err := json.Unmarshal(request.Task, &task); err != nil {
			writeError(w, http.StatusBadRequest, "任务格式错误")
			return
		}
		task.RemoveFields("index", "status", "latest", "next_run", "progress")
		task.SavePath = quark.NormalizePath(task.SavePath)
		task.NormalizeMode()
		if err := validateTask(&task); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		inline = &task
	}

	for _, index := range request.Tasks {
		if index < 0 || index >= len(snapshot.Tasks) {
			writeError(w, http.StatusBadRequest, "任务序号无效")
			return
		}
	}
	label := "全部任务"
	switch {
	case inline != nil:
		label = "临时任务 · " + inline.TaskName
	case len(request.Tasks) > 0:
		var names []string
		for _, index := range request.Tasks {
			names = append(names, snapshot.Tasks[index].TaskName)
		}
		label = strings.Join(names, "、")
	}
	run, err := s.startRun(runSpec{Label: label, Indices: request.Tasks, Write: write, Inline: inline})
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "run_id": run.ID})
}

// runSpec describes one job for the single-run manager. Sign-only jobs share
// the lock so a sign cannot race a transfer on the same account.
type runSpec struct {
	Label    string
	Indices  []int
	Write    bool
	SignOnly bool
	Inline   *quark.Task
}

func (s *Server) startRun(spec runSpec) (*Run, error) {
	return s.runs.Start(spec.Label, spec.Write, func(run *Run) error {
		return s.executeRun(run, spec)
	})
}

func (s *Server) handleRuns(w http.ResponseWriter, r *http.Request) {
	views := s.runLogs.views()
	if active := s.runs.ActiveID(); active != "" {
		if run := s.runs.Get(active); run != nil {
			view := run.View()
			merged := []RunView{view}
			for _, item := range views {
				if item.ID != view.ID {
					merged = append(merged, item)
				}
			}
			views = merged
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": map[string]any{"runs": views}})
}

func (s *Server) handleLatestRun(w http.ResponseWriter, r *http.Request) {
	run := s.runs.Latest()
	if run == nil {
		if metas := s.runLogs.list(); len(metas) > 0 {
			writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": metaView(metas[0])})
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": nil})
		return
	}
	view := run.View()
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": view})
}

func (s *Server) handleGetRun(w http.ResponseWriter, r *http.Request) {
	run := s.runs.Get(r.PathValue("id"))
	if run == nil {
		writeError(w, http.StatusNotFound, "运行记录不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": run.View()})
}

func (s *Server) handleRunLog(w http.ResponseWriter, r *http.Request) {
	run := s.runs.Get(r.PathValue("id"))
	if run == nil {
		meta, lines, ok := s.runLogs.read(r.PathValue("id"))
		if !ok {
			writeError(w, http.StatusNotFound, "运行记录不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data": map[string]any{
				"run":   metaView(meta),
				"lines": lines,
				"done":  true,
			},
		})
		return
	}
	lines, done := run.LinesFrom(0)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"run":   run.View(),
			"lines": lines,
			"done":  done,
		},
	})
}

func (s *Server) handleRunStream(w http.ResponseWriter, r *http.Request) {
	run := s.runs.Get(r.PathValue("id"))
	if run == nil {
		writeError(w, http.StatusNotFound, "运行记录不存在")
		return
	}
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeError(w, http.StatusInternalServerError, "不支持流式输出")
		return
	}
	w.Header().Set("Content-Type", "text/event-stream; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("X-Accel-Buffering", "no")

	// Replay from the first line so late or reconnecting clients see the whole
	// log; the subscription only wakes the loop on new lines.
	_, notify, cancel := run.Subscribe()
	defer cancel()
	offset := 0
	if last := r.Header.Get("Last-Event-ID"); last != "" {
		if seq, err := strconv.Atoi(last); err == nil && seq >= 0 {
			offset = seq + 1
		}
	}
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		lines, done := run.LinesFrom(offset)
		for _, line := range lines {
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", offset, strings.ReplaceAll(line, "\r", ""))
			offset++
		}
		if len(lines) > 0 {
			flusher.Flush()
		}
		if done {
			fmt.Fprint(w, "data: [DONE]\n\n")
			flusher.Flush()
			return
		}
		select {
		case <-r.Context().Done():
			return
		case <-notify:
		case <-ticker.C:
			fmt.Fprint(w, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	snapshot := s.store.Snapshot()
	banned := 0
	for _, task := range snapshot.Tasks {
		if task.ShareURLBan != "" {
			banned++
		}
	}
	var latest any
	if run := s.runs.Latest(); run != nil {
		latest = run.View()
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"task_count":   len(snapshot.Tasks),
			"banned_count": banned,
			"crontab":      snapshot.Crontab,
			"version":      s.opts.Version,
			"active_run":   s.runs.ActiveID(),
			"latest_run":   latest,
		},
	})
}

// ---------- helpers ----------

func decodeJSON(r *http.Request, target any) error {
	return json.NewDecoder(io.LimitReader(r.Body, 2<<20)).Decode(target)
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{"success": false, "message": message})
}

func (s *Server) handleStatic(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		writeError(w, http.StatusNotFound, "接口不存在")
		return
	}
	dir := s.opts.StaticDir
	if dir == "" {
		writeJSON(w, http.StatusOK, map[string]any{
			"name":    "quark-auto-save",
			"version": s.opts.Version,
			"message": "Web UI 未构建；开发时请用 Vite 代理 /api",
		})
		return
	}
	cleaned := path.Clean("/" + r.URL.Path)
	file := filepath.Join(dir, filepath.FromSlash(cleaned))
	if info, err := os.Stat(file); err == nil && !info.IsDir() {
		http.ServeFile(w, r, file)
		return
	}
	if strings.Contains(path.Base(cleaned), ".") && cleaned != "/" {
		http.NotFound(w, r)
		return
	}
	index := filepath.Join(dir, "index.html")
	if _, err := os.Stat(index); err != nil {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, index)
}
