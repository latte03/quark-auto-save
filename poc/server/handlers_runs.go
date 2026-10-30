package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"qas-poc/quark"
)

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
