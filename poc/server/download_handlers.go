package server

import (
	"net/http"
)

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
