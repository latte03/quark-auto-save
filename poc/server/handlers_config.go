package server

import (
	"encoding/json"
	"net/http"
	"strings"
)

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
