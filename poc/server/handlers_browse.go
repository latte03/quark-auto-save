package server

import (
	"fmt"
	"net/http"

	"qas-poc/quark"
)

// ---------- browse ----------

type browseEntry struct {
	Name     string `json:"name"`
	FID      string `json:"fid"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Category string `json:"category,omitempty"`
	Count    int    `json:"count,omitempty"`
}

func (s *Server) handleBrowse(w http.ResponseWriter, r *http.Request) {
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	dirPath := quark.NormalizePath(r.URL.Query().Get("path"))
	if dirPath == "" {
		dirPath = "/"
	}
	fid := "0"
	if dirPath != "/" {
		fids, err := client.GetFids([]string{dirPath})
		if err != nil {
			writeError(w, http.StatusBadGateway, apiMessage(err))
			return
		}
		if len(fids) == 0 {
			writeError(w, http.StatusNotFound, "目录不存在: "+dirPath)
			return
		}
		fid = fids[0].FID
	}
	files, err := client.LsDir(fid)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	entries := make([]browseEntry, 0, len(files))
	for _, file := range files {
		entries = append(entries, browseEntry{
			Name:     file.FileName,
			FID:      file.FID,
			IsDir:    file.Dir,
			Size:     file.Size,
			Category: file.ObjCategory,
			Count:    file.IncludeItems,
		})
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data":    map[string]any{"path": dirPath, "fid": fid, "entries": entries},
	})
}

// ---------- share ----------

func (s *Server) handleValidateShare(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ShareURL string `json:"shareurl"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	ref, err := quark.ExtractShareURL(request.ShareURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "分享链接格式无效")
		return
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"ok": false, "message": apiMessage(err)},
		})
		return
	}
	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, ref.PdirFID, 0, 0)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"ok": false, "message": apiMessage(err)},
		})
		return
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"ok":    true,
			"title": title,
			"count": len(detail.List),
			"path":  len(ref.Paths),
		},
	})
}

type shareFileView struct {
	Name     string `json:"name"`
	FID      string `json:"fid"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Category string `json:"category,omitempty"`
	Matched  bool   `json:"matched"`
	Saved    bool   `json:"saved"`
	Renamed  string `json:"renamed,omitempty"`
}

const sharePreviewLimit = 500

func validFID(fid string) bool {
	if len(fid) == 0 || len(fid) > 64 {
		return false
	}
	for _, char := range fid {
		if (char < '0' || char > '9') && (char < 'a' || char > 'z') && (char < 'A' || char > 'Z') {
			return false
		}
	}
	return true
}

func (s *Server) handleShareDetail(w http.ResponseWriter, r *http.Request) {
	var request struct {
		ShareURL        string `json:"shareurl"`
		PdirFID         string `json:"pdir_fid"`
		TaskName        string `json:"taskname"`
		MagicID         string `json:"magic_id"`
		Pattern         string `json:"pattern"`
		Replace         string `json:"replace"`
		SavePath        string `json:"savepath"`
		UpdateSubdir    string `json:"update_subdir"`
		IgnoreExtension bool   `json:"ignore_extension"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	ref, err := quark.ExtractShareURL(request.ShareURL)
	if err != nil {
		writeError(w, http.StatusBadRequest, "分享链接格式无效")
		return
	}
	pdirFID := ref.PdirFID
	if request.PdirFID != "" {
		if !validFID(request.PdirFID) {
			writeError(w, http.StatusBadRequest, "目录标识格式无效")
			return
		}
		pdirFID = request.PdirFID
	}
	client, err := s.client()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	detail, err := client.GetDetail(ref.PwdID, stoken.Stoken, pdirFID, 0, 0)
	if err != nil {
		writeError(w, http.StatusBadGateway, apiMessage(err))
		return
	}
	title := detail.Title
	if title == "" {
		title = stoken.Title
	}
	files := detail.List
	if len(files) == 1 && files[0].Dir && pdirFID == "0" {
		nested, err := client.GetDetail(ref.PwdID, stoken.Stoken, files[0].FID, 0, 0)
		if err == nil && len(nested.List) > 0 {
			files = nested.List
		}
	}

	var dirFiles []quark.ShareFile
	savepath := quark.NormalizePath(request.SavePath)
	if savepath != "/" {
		if fids, err := client.GetFids([]string{savepath}); err == nil && len(fids) > 0 {
			dirFiles, _ = client.LsDir(fids[0].FID)
		}
	}
	dirNames := make([]string, len(dirFiles))
	for i, file := range dirFiles {
		dirNames[i] = file.FileName
	}

	snapshot := s.store.Snapshot()
	magic := quark.NewMagic(snapshot.Magic)
	magic.SetTaskName(request.TaskName)
	pattern, replace := magic.ResolveTask(request.MagicID, request.Pattern, request.Replace)

	var views []shareFileView
	var pending []*quark.ShareFile
	indexOf := map[string]int{}
	for _, item := range files {
		file := item
		search := pattern
		if file.Dir && request.UpdateSubdir != "" {
			search = request.UpdateSubdir
		}
		matched := false
		if search != "" {
			matched, err = magic.Match(search, file.FileName)
			if err != nil {
				writeError(w, http.StatusBadRequest, fmt.Sprintf("正则格式错误: %v", err))
				return
			}
		}
		view := shareFileView{
			Name:     file.FileName,
			FID:      file.FID,
			IsDir:    file.Dir,
			Size:     file.Size,
			Category: file.ObjCategory,
			Matched:  matched,
		}
		if matched {
			if file.Dir {
				view.Saved = magic.Exists(file.FileName, dirNames, false) != ""
			} else {
				nameRe, err := magic.Sub(pattern, replace, file.FileName)
				if err != nil {
					writeError(w, http.StatusBadRequest, fmt.Sprintf("正则替换失败: %v", err))
					return
				}
				view.Renamed = nameRe
				view.Saved = magic.Exists(nameRe, dirNames, request.IgnoreExtension) != ""
				if !view.Saved {
					file.FileNameRe = nameRe
					pending = append(pending, &file)
					indexOf[file.FID] = len(views)
				}
			}
		}
		views = append(views, view)
	}

	if len(pending) > 0 {
		if err := magic.SetDirFileList(dirFiles, replace); err == nil {
			magic.SortFileList(pending)
		}
		for _, file := range pending {
			if index, ok := indexOf[file.FID]; ok {
				views[index].Renamed = file.FileNameRe
			}
		}
	}

	total := len(views)
	if total > sharePreviewLimit {
		views = views[:sharePreviewLimit]
	}
	if views == nil {
		views = []shareFileView{}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"title":    title,
			"savepath": savepath,
			"pattern":  pattern,
			"replace":  replace,
			"total":    total,
			"files":    views,
		},
	})
}
