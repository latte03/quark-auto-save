package server

import (
	"fmt"
	"net/http"
	"strings"

	"qas-poc/quark"
)

// ---------- rename preview ----------

type renamePreviewItem struct {
	Name    string `json:"name"`
	Matched bool   `json:"matched"`
	Renamed string `json:"renamed"`
}

func (s *Server) handleRenamePreview(w http.ResponseWriter, r *http.Request) {
	var request struct {
		TaskName string   `json:"taskname"`
		MagicID  string   `json:"magic_id"`
		Pattern  string   `json:"pattern"`
		Replace  string   `json:"replace"`
		Files    []string `json:"files"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if len(request.Files) == 0 || len(request.Files) > 50 {
		writeError(w, http.StatusBadRequest, "一次可预览 1 到 50 个文件名")
		return
	}
	snapshot := s.store.Snapshot()
	magic := quark.NewMagic(snapshot.Magic)
	magic.SetTaskName(request.TaskName)
	pattern, replace := magic.ResolveTask(request.MagicID, request.Pattern, request.Replace)

	items := make([]renamePreviewItem, len(request.Files))
	var planned []*quark.ShareFile
	var plannedIndex []int
	for index, name := range request.Files {
		item := renamePreviewItem{Name: name, Renamed: name}
		if pattern != "" {
			matched, err := magic.Match(pattern, name)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("正则格式错误: %v", err))
				return
			}
			item.Matched = matched
			if matched {
				renamed, err := magic.Sub(pattern, replace, name)
				if err != nil {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("正则替换失败: %v", err))
					return
				}
				item.Renamed = renamed
				planned = append(planned, &quark.ShareFile{FileName: name, FileNameRe: renamed})
				plannedIndex = append(plannedIndex, index)
			}
		} else {
			item.Matched = true
		}
		items[index] = item
	}
	if len(planned) > 0 && strings.Contains(replace, "{I") {
		if err := magic.SetDirFileList(nil, replace); err == nil {
			magic.SortFileList(planned)
		}
		for position, file := range planned {
			items[plannedIndex[position]].Renamed = file.FileNameRe
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"pattern": pattern,
			"replace": replace,
			"files":   items,
		},
	})
}
