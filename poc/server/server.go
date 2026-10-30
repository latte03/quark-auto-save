package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"time"

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
	mux.HandleFunc("POST /api/downloads/clear", s.auth(s.handleClearDownloads))
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
