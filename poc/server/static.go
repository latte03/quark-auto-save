package server

import (
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
)

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
