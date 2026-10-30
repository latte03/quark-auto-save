package server

import (
	"context"
	"net/http"
	"strings"
	"time"

	"qas-poc/notify"
	"qas-poc/source"
)

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
