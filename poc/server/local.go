package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"qas-poc/quark"
)

type localDownloadSettings struct {
	Dir         string
	Concurrency int
	MinFreeGB   int64
}

// LocalDownload reads the global download defaults from the config.
func (s *Store) LocalDownload() localDownloadSettings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return localDownloadFromConfig(s.cfg)
}

func localDownloadFromConfig(cfg *quark.Config) localDownloadSettings {
	settings := localDownloadSettings{Concurrency: 2, MinFreeGB: 5}
	raw := cfg.Raw("local_download")
	if len(raw) == 0 {
		return settings
	}
	var stored struct {
		Dir         string `json:"dir"`
		Concurrency int    `json:"concurrency"`
		MinFreeGB   *int64 `json:"min_free_gb"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil {
		return settings
	}
	settings.Dir = strings.TrimSpace(stored.Dir)
	if stored.Concurrency > 0 {
		settings.Concurrency = stored.Concurrency
	}
	// 缺省时沿用默认阈值；显式写 0 才表示不检查剩余空间。
	if stored.MinFreeGB != nil && *stored.MinFreeGB >= 0 {
		settings.MinFreeGB = *stored.MinFreeGB
	}
	return settings
}

func freeBytes(path string) uint64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return uint64(stat.Bavail) * uint64(stat.Bsize)
}

type localDirCheck struct {
	OK        bool   `json:"ok"`
	Writable  bool   `json:"writable"`
	FreeBytes uint64 `json:"free_bytes"`
	Message   string `json:"message"`
}

// handleLocalValidate checks a NAS directory for the built-in downloader:
// it must live under /vol and the QAS process must be able to create files.
func (s *Server) handleLocalValidate(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	path := filepath.Clean(strings.TrimSpace(request.Path))
	check := localDirCheck{}
	if message := validateLocalRoot(path); message != "" {
		check.Message = message
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": check})
		return
	}
	check = checkLocalDir(path)
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": check})
}

type localDirEntry struct {
	Name string `json:"name"`
	Path string `json:"path"`
}

// handleLocalBrowse lists subdirectories below the administrator-approved
// roots so the UI can pick a child folder without re-authorising it.
func (s *Server) handleLocalBrowse(w http.ResponseWriter, r *http.Request) {
	data, err := trimCall(r.Context(), "trim.file.getSharedAccessibleFolders", map[string]any{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	var authorized struct {
		Paths []string `json:"paths"`
	}
	if err := json.Unmarshal(data, &authorized); err != nil {
		writeError(w, http.StatusBadGateway, "解析授权目录失败")
		return
	}
	roots := make([]string, 0, len(authorized.Paths))
	for _, item := range authorized.Paths {
		cleaned := filepath.Clean(strings.TrimSpace(item))
		if cleaned == "" || cleaned == "." {
			continue
		}
		// 授权根目录自身也可能是指向别处的软链，先解析成真实路径再比对。
		if resolved, err := filepath.EvalSymlinks(cleaned); err == nil {
			cleaned = resolved
		}
		roots = append(roots, cleaned)
	}
	sort.Strings(roots)

	response := map[string]any{
		"roots":  roots,
		"path":   "",
		"parent": "",
		"dirs":   []localDirEntry{},
	}
	requested := strings.TrimSpace(r.URL.Query().Get("path"))
	if requested == "" {
		writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": response})
		return
	}
	clean := filepath.Clean(requested)
	// 按真实路径判定归属：授权目录内的软链不能把浏览带出授权范围。
	resolved, root, ok := resolveAuthorizedPath(clean, roots)
	if !ok {
		writeError(w, http.StatusBadRequest, "只能浏览已授权目录")
		return
	}
	entries, err := os.ReadDir(resolved)
	if err != nil {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("读取目录失败: %v", err))
		return
	}
	dirs := make([]localDirEntry, 0, len(entries))
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		dirs = append(dirs, localDirEntry{Name: entry.Name(), Path: filepath.Join(resolved, entry.Name())})
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].Name < dirs[j].Name })
	parent := ""
	if resolved != root {
		parent = filepath.Dir(resolved)
	}
	response["path"] = resolved
	response["parent"] = parent
	response["dirs"] = dirs
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": response})
}

func matchAuthorizedRoot(path string, roots []string) string {
	for _, root := range roots {
		if path == root || strings.HasPrefix(path, root+string(os.PathSeparator)) {
			return root
		}
	}
	return ""
}

// resolveAuthorizedPath resolves path through any symlinks and reports the
// real path plus the authorized root it lands in.
func resolveAuthorizedPath(path string, roots []string) (string, string, bool) {
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return "", "", false
	}
	root := matchAuthorizedRoot(resolved, roots)
	if root == "" {
		return "", "", false
	}
	return resolved, root, true
}

func validateLocalRoot(path string) string {
	if path == "" || path == "." {
		return "请选择或填写下载目录"
	}
	if !filepath.IsAbs(path) {
		return "请填写绝对路径，例如 /vol1/1000/Media"
	}
	if path == "/" {
		return "不能使用根目录"
	}
	if !strings.HasPrefix(path, "/vol") {
		return "目录必须位于 /vol1、/vol2 等存储空间下"
	}
	lower := strings.ToLower(path)
	for _, blocked := range []string{"/@appcenter", "/@appconf", "/@appdata", "/@apphome", "/@apptemp"} {
		if strings.Contains(lower, blocked) {
			return "不能选择应用系统目录"
		}
	}
	return ""
}

func checkLocalDir(path string) localDirCheck {
	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return localDirCheck{Message: "目录不存在"}
		}
		return localDirCheck{Message: fmt.Sprintf("读取目录失败: %v", err)}
	}
	if !info.IsDir() {
		return localDirCheck{Message: "目标不是目录"}
	}
	probe := filepath.Join(path, fmt.Sprintf(".qas-write-%d", time.Now().UnixNano()))
	file, err := os.OpenFile(probe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return localDirCheck{Message: fmt.Sprintf("目录不可写: %v", err)}
	}
	_, writeErr := file.WriteString("ok")
	closeErr := file.Close()
	_ = os.Remove(probe)
	if writeErr != nil || closeErr != nil {
		return localDirCheck{Message: "目录不可写"}
	}

	check := localDirCheck{OK: true, Writable: true}
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err == nil {
		check.FreeBytes = uint64(stat.Bavail) * uint64(stat.Bsize)
	}
	return check
}
