package server

import (
	"errors"
	"fmt"
	"net/http"
	"strings"

	"qas-poc/quark"
)

// ---------- files ----------

// resolveFID maps a cloud path (or an explicit fid) to a file id. The root is
// never a valid target for rename/delete.
//
// Quark's path_list endpoint resolves directories only; files must be looked
// up by listing the parent directory and matching the name, which is also how
// the upstream Python client did it.
func resolveFID(client *quark.Client, path, fid string) (string, error) {
	if value := strings.TrimSpace(fid); value != "" {
		if value == "0" {
			return "", errors.New("不能操作根目录")
		}
		return value, nil
	}
	path = quark.NormalizePath(path)
	if len(path) > 1 {
		path = strings.TrimRight(path, "/")
	}
	switch path {
	case "":
		return "", errors.New("缺少 path 或 fid")
	case "/":
		return "", errors.New("不能操作根目录")
	}

	parent, name := splitCloudPath(path)
	parentFID := "0"
	if parent != "/" {
		items, err := client.GetFids([]string{parent})
		if err != nil {
			return "", err
		}
		for _, item := range items {
			if item.FilePath == parent && item.FID != "" {
				parentFID = item.FID
				break
			}
		}
		if parentFID == "0" {
			return "", fmt.Errorf("路径不存在：%s", parent)
		}
	}

	entries, err := client.LsDir(parentFID)
	if err != nil {
		return "", err
	}
	for _, entry := range entries {
		if entry.FileName == name && entry.FID != "" {
			return entry.FID, nil
		}
	}
	return "", fmt.Errorf("路径不存在：%s", path)
}

// splitCloudPath splits a normalized path into its parent directory and the
// final name ("/a/b/c" -> "/a/b", "c"; "/c" -> "/", "c").
func splitCloudPath(path string) (string, string) {
	index := strings.LastIndex(path, "/")
	parent := path[:index]
	if parent == "" {
		parent = "/"
	}
	return parent, path[index+1:]
}

func (s *Server) guardIdle(w http.ResponseWriter) bool {
	if s.runs.ActiveID() != "" {
		writeError(w, http.StatusConflict, "任务运行中，请稍后再操作文件")
		return false
	}
	return true
}

func (s *Server) handleDeleteFile(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path    string `json:"path"`
		FID     string `json:"fid"`
		Confirm bool   `json:"confirm"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if !request.Confirm {
		writeError(w, http.StatusBadRequest, "删除需要显式确认")
		return
	}
	if !s.guardIdle(w) {
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fid, err := resolveFID(client, request.Path, request.FID)
	if err != nil {
		writeError(w, http.StatusBadRequest, apiMessage(err))
		return
	}
	taskID, err := client.Delete([]string{fid})
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	s.logger.Printf("删除文件: %s (fid=%s)", quark.NormalizePath(request.Path), fid)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"fid": fid, "task_id": taskID},
	})
}

func (s *Server) handleRenameFile(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
		FID  string `json:"fid"`
		Name string `json:"name"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	name := strings.TrimSpace(request.Name)
	switch {
	case name == "":
		writeError(w, http.StatusBadRequest, "新名称不能为空")
		return
	case name == "." || name == "..":
		writeError(w, http.StatusBadRequest, "新名称无效")
		return
	case strings.ContainsAny(name, "/\\"):
		writeError(w, http.StatusBadRequest, "新名称不能包含路径分隔符")
		return
	case len([]byte(name)) > 255:
		writeError(w, http.StatusBadRequest, "新名称过长")
		return
	}
	if !s.guardIdle(w) {
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fid, err := resolveFID(client, request.Path, request.FID)
	if err != nil {
		writeError(w, http.StatusBadRequest, apiMessage(err))
		return
	}
	if err := client.Rename(fid, name); err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	s.logger.Printf("重命名文件: %s → %s", quark.NormalizePath(request.Path), name)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"fid": fid, "name": name},
	})
}

func (s *Server) handleMkdir(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Path string `json:"path"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	dirPath := quark.NormalizePath(request.Path)
	if len(dirPath) > 1 {
		dirPath = strings.TrimRight(dirPath, "/")
	}
	if dirPath == "" || dirPath == "/" {
		writeError(w, http.StatusBadRequest, "目录路径无效")
		return
	}
	if !s.guardIdle(w) {
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	fid, err := client.Mkdir(dirPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	s.logger.Printf("新建目录: %s", dirPath)
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"path": dirPath, "fid": fid},
	})
}
