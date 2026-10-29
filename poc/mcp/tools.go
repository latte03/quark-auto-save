package mcp

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (s *Server) buildTools() []*Tool {
	return []*Tool{
		{
			Name:        "get_status",
			Description: "查看 QAS 控制台状态：任务数、失效链接数、crontab、正在运行的任务和最近一次运行。",
			Schema:      objectSchema(nil),
			Handler:     s.toolGetStatus,
		},
		{
			Name:        "list_tasks",
			Description: "列出全部转存任务（含序号、状态、下次运行、失效原因），序号可直接用于 update_task / delete_task / run_tasks。",
			Schema:      objectSchema(nil),
			Handler:     s.toolListTasks,
		},
		{
			Name:        "get_task",
			Description: "按序号或名称查看单个任务，名称为精确匹配优先，其次唯一包含匹配。",
			Schema: objectSchema(map[string]any{
				"index": integerSchema("任务序号，从 0 开始"),
				"name":  stringSchema("任务名称或名称片段"),
			}),
			Handler: s.toolGetTask,
		},
		{
			Name:        "get_config",
			Description: "查看全局配置（crontab、插件、通知渠道、搜索源等）。Cookie、API token、密码类字段会被自动替换为 ***。",
			Schema:      objectSchema(nil),
			Handler:     s.toolGetConfig,
		},
		{
			Name:        "browse_files",
			Description: "浏览夸克网盘目录，返回当前目录下的文件与文件夹。",
			Schema: objectSchema(map[string]any{
				"path": stringSchema("绝对路径，例如 /video/tv；默认 /"),
			}),
			Handler: s.toolBrowse,
		},
		{
			Name:        "validate_share",
			Description: "校验夸克分享链接是否有效，返回标题与文件数量；不会转存。",
			Schema: objectSchema(map[string]any{
				"shareurl": stringSchema("https://pan.quark.cn/s/xxx，可带 #/list/share/... 子路径"),
			}, "shareurl"),
			Handler: s.toolValidateShare,
		},
		{
			Name:        "share_detail",
			Description: "查看分享内容，并按给定正则计算重命名结果与是否已存在；用于转存前确认。",
			Schema: objectSchema(map[string]any{
				"shareurl":         stringSchema("分享链接"),
				"taskname":         stringSchema("任务名，用于 {TASKNAME} 等魔法变量"),
				"pattern":          stringSchema("正则匹配式，支持 $TV_MAGIC 等魔法键"),
				"replace":          stringSchema("正则替换式，支持 {TASKNAME}/{SXX}/{E} 等变量"),
				"savepath":         stringSchema("目标目录，用于判断文件是否已存在"),
				"update_subdir":    stringSchema("追更子目录匹配式"),
				"ignore_extension": booleanSchema("比较已存在文件时忽略后缀"),
			}, "shareurl"),
			Handler: s.toolShareDetail,
		},
		{
			Name:        "preview_rename",
			Description: "用 QAS 的魔法正则引擎预览文件名替换结果（最多 50 个文件名）。",
			Schema: objectSchema(map[string]any{
				"taskname": stringSchema("任务名"),
				"pattern":  stringSchema("匹配表达式"),
				"replace":  stringSchema("替换表达式"),
				"files":    arraySchema(stringSchema("文件名"), "待预览的文件名"),
			}, "files"),
			Handler: s.toolPreviewRename,
		},
		{
			Name:        "search_shares",
			Description: "用已配置的 CloudSaver / PanSou 搜索夸克分享资源，返回候选链接。",
			Schema: objectSchema(map[string]any{
				"keyword": stringSchema("搜索关键词"),
				"deep":    booleanSchema("深度搜索"),
			}, "keyword"),
			Handler: s.toolSearchShares,
		},
		{
			Name:        "get_run",
			Description: "查看一次运行的概况；不传 id 时返回最近一次运行。",
			Schema: objectSchema(map[string]any{
				"id": stringSchema("运行 ID，缺省为最近一次"),
			}),
			Handler: s.toolGetRun,
		},
		{
			Name:        "get_run_log",
			Description: "读取运行的日志尾部；不传 id 时读取最近一次运行。",
			Schema: objectSchema(map[string]any{
				"id":   stringSchema("运行 ID，缺省为最近一次"),
				"tail": integerSchema("返回最后多少行，默认 200，最多 2000"),
			}),
			Handler: s.toolGetRunLog,
		},
		{
			Name:        "wait_run",
			Description: "等待指定的运行结束并返回尾部日志；超时后返回当前状态。",
			Schema: objectSchema(map[string]any{
				"id":              stringSchema("运行 ID"),
				"timeout_seconds": integerSchema("最长等待秒数，默认 60，最多 600"),
				"tail":            integerSchema("日志最后多少行，默认 100"),
			}, "id"),
			Handler: s.toolWaitRun,
		},
		{
			Name:        "add_task",
			Description: "新增一个持久化转存任务。必填 taskname、shareurl、savepath；pattern/replace 决定命名。新增后不会自动运行。",
			Schema: objectSchema(map[string]any{
				"task": taskSchema("任务对象"),
			}, "task"),
			Write:   true,
			Handler: s.toolAddTask,
		},
		{
			Name:        "update_task",
			Description: "按序号整体更新任务。请先 get_task 取回对象，修改后再提交；未知字段会保留。",
			Schema: objectSchema(map[string]any{
				"index": integerSchema("任务序号，从 0 开始"),
				"task":  taskSchema("完整任务对象"),
			}, "index", "task"),
			Write:   true,
			Handler: s.toolUpdateTask,
		},
		{
			Name:        "delete_task",
			Description: "删除任务（不会删除已转存的文件）。必须传 confirm=true。",
			Schema: objectSchema(map[string]any{
				"index":   integerSchema("任务序号，从 0 开始"),
				"confirm": booleanSchema("确认删除"),
			}, "index", "confirm"),
			Write:       true,
			Destructive: true,
			Handler:     s.toolDeleteTask,
		},
		{
			Name:        "set_crontab",
			Description: "修改定时运行表达式（5 段 cron），保存后立即重载调度器。传空字符串表示停用定时运行。",
			Schema: objectSchema(map[string]any{
				"crontab": stringSchema("cron 表达式，例如 0 8,18,20 * * *"),
			}, "crontab"),
			Write:   true,
			Handler: s.toolSetCrontab,
		},
		{
			Name: "run_tasks",
			Description: "立即运行任务。可传 tasks（任务序号）或 task（临时任务，不保存到任务列表，二者互斥），都不传则运行全部。" +
				"默认 write=false 仅演练；write=true 会真实转存，必须同时传 confirm=true。",
			Schema: objectSchema(map[string]any{
				"tasks":        arraySchema(integerSchema("任务序号"), "要运行的任务序号；空为全部"),
				"task":         taskSchema("临时任务（执行后不写入配置）"),
				"write":        booleanSchema("true 会真正转存与改名；默认 false 仅演练"),
				"confirm":      booleanSchema("write=true 时必须为 true，表示确认真实写入网盘"),
				"wait_seconds": integerSchema("等待运行结束的秒数，默认 0 立即返回"),
				"tail":         integerSchema("等待时返回的日志行数，默认 100"),
			}),
			Write:       true,
			Destructive: true,
			Handler:     s.toolRunTasks,
		},
		{
			Name:        "sign_in",
			Description: "立即执行多账号签到（需要带 kps/sign/vcode 的移动端 Cookie），可等待结果。",
			Schema: objectSchema(map[string]any{
				"wait_seconds": integerSchema("等待签到结束的秒数，默认 0 立即返回"),
				"tail":         integerSchema("等待时返回的日志行数，默认 50"),
			}),
			Write:   true,
			Handler: s.toolSignIn,
		},
		{
			Name:        "rename_file",
			Description: "重命名网盘文件或文件夹，可用 path 或 fid 指定目标。",
			Schema: objectSchema(map[string]any{
				"path": stringSchema("完整路径"),
				"fid":  stringSchema("文件 ID，与 path 二选一"),
				"name": stringSchema("新名称，不能包含 /"),
			}, "name"),
			Write:   true,
			Handler: s.toolRenameFile,
		},
		{
			Name:        "delete_file",
			Description: "删除网盘文件或文件夹（移入回收站）。必须传 confirm=true；任务运行中会被拒绝。",
			Schema: objectSchema(map[string]any{
				"path":    stringSchema("完整路径"),
				"fid":     stringSchema("文件 ID，与 path 二选一"),
				"confirm": booleanSchema("确认删除"),
			}, "confirm"),
			Write:       true,
			Destructive: true,
			Handler:     s.toolDeleteFile,
		},
		{
			Name:        "mkdir",
			Description: "在网盘创建文件夹（自动创建缺失的上级目录）。",
			Schema: objectSchema(map[string]any{
				"path": stringSchema("完整目录路径"),
			}, "path"),
			Write:   true,
			Handler: s.toolMkdir,
		},
		{
			Name:        "notify_test",
			Description: "用当前推送配置发送一条测试通知，返回每个渠道的成功/失败。",
			Schema: objectSchema(map[string]any{
				"title":   stringSchema("标题，默认【夸克自动转存】"),
				"content": stringSchema("内容，默认带时间戳"),
			}),
			Write:   true,
			Handler: s.toolNotifyTest,
		},
	}
}

// ---------- read tools ----------

func (s *Server) toolGetStatus(ctx context.Context, _ map[string]any) (any, error) {
	var data map[string]any
	if err := s.api.get(ctx, "/api/status", nil, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolListTasks(ctx context.Context, _ map[string]any) (any, error) {
	var data map[string]any
	if err := s.api.get(ctx, "/api/tasks", nil, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolGetTask(ctx context.Context, args map[string]any) (any, error) {
	tasks, err := s.fetchTasks(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := args["index"]; ok {
		index, err := intArg(args, "index", -1)
		if err != nil {
			return nil, err
		}
		if index < 0 || index >= len(tasks) {
			return nil, fmt.Errorf("任务序号 %d 超出范围 0..%d", index, len(tasks)-1)
		}
		return tasks[index], nil
	}
	name := stringArg(args, "name")
	if name == "" {
		return nil, errors.New("需要 index 或 name")
	}
	var partial []map[string]any
	for _, task := range tasks {
		title, _ := task["taskname"].(string)
		if title == name {
			return task, nil
		}
		if strings.Contains(title, name) {
			partial = append(partial, task)
		}
	}
	if len(partial) == 1 {
		return partial[0], nil
	}
	if len(partial) == 0 {
		return nil, fmt.Errorf("没有名称包含 %q 的任务", name)
	}
	var names []string
	for _, task := range partial {
		names = append(names, fmt.Sprint(task["taskname"]))
	}
	return nil, fmt.Errorf("匹配到多个任务，请用 index：%s", strings.Join(names, "、"))
}

func (s *Server) toolGetConfig(ctx context.Context, _ map[string]any) (any, error) {
	var data map[string]any
	if err := s.api.get(ctx, "/api/config", nil, &data); err != nil {
		return nil, err
	}
	return sanitizeConfig(data), nil
}

func (s *Server) toolBrowse(ctx context.Context, args map[string]any) (any, error) {
	path := stringArg(args, "path")
	if path == "" {
		path = "/"
	}
	var data map[string]any
	if err := s.api.get(ctx, "/api/browse", url.Values{"path": {path}}, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolValidateShare(ctx context.Context, args map[string]any) (any, error) {
	shareurl := stringArg(args, "shareurl")
	if shareurl == "" {
		return nil, errors.New("shareurl 不能为空")
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/share/validate", map[string]any{"shareurl": shareurl}, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolShareDetail(ctx context.Context, args map[string]any) (any, error) {
	shareurl := stringArg(args, "shareurl")
	if shareurl == "" {
		return nil, errors.New("shareurl 不能为空")
	}
	body := map[string]any{
		"shareurl":         shareurl,
		"taskname":         stringArg(args, "taskname"),
		"pattern":          stringArg(args, "pattern"),
		"replace":          stringArg(args, "replace"),
		"savepath":         stringArg(args, "savepath"),
		"update_subdir":    stringArg(args, "update_subdir"),
		"ignore_extension": boolArg(args, "ignore_extension"),
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/share/detail", body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolPreviewRename(ctx context.Context, args map[string]any) (any, error) {
	rawFiles, ok := args["files"].([]any)
	if !ok || len(rawFiles) == 0 {
		return nil, errors.New("files 需要 1 到 50 个文件名")
	}
	if len(rawFiles) > 50 {
		return nil, errors.New("一次最多预览 50 个文件名")
	}
	files := make([]string, 0, len(rawFiles))
	for _, item := range rawFiles {
		files = append(files, fmt.Sprint(item))
	}
	body := map[string]any{
		"taskname": stringArg(args, "taskname"),
		"pattern":  stringArg(args, "pattern"),
		"replace":  stringArg(args, "replace"),
		"files":    files,
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/rename/preview", body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolSearchShares(ctx context.Context, args map[string]any) (any, error) {
	keyword := stringArg(args, "keyword")
	if keyword == "" {
		return nil, errors.New("keyword 不能为空")
	}
	query := url.Values{"keyword": {keyword}}
	if boolArg(args, "deep") {
		query.Set("deep", "1")
	}
	var data map[string]any
	if err := s.api.get(ctx, "/api/search", query, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolGetRun(ctx context.Context, args map[string]any) (any, error) {
	id := stringArg(args, "id")
	if id == "" {
		var latest map[string]any
		if err := s.api.get(ctx, "/api/runs/latest", nil, &latest); err != nil {
			return nil, err
		}
		if latest == nil {
			return nil, errors.New("还没有运行记录")
		}
		return latest, nil
	}
	var view map[string]any
	if err := s.api.get(ctx, "/api/runs/"+url.PathEscape(id), nil, &view); err != nil {
		return nil, err
	}
	return view, nil
}

func (s *Server) toolGetRunLog(ctx context.Context, args map[string]any) (any, error) {
	tail, err := intArg(args, "tail", 200)
	if err != nil {
		return nil, err
	}
	id := stringArg(args, "id")
	if id == "" {
		id, err = s.latestRunID(ctx)
		if err != nil {
			return nil, err
		}
	}
	return s.runLog(ctx, id, tail)
}

func (s *Server) toolWaitRun(ctx context.Context, args map[string]any) (any, error) {
	id := stringArg(args, "id")
	if id == "" {
		return nil, errors.New("id 不能为空")
	}
	timeout, err := intArg(args, "timeout_seconds", 60)
	if err != nil {
		return nil, err
	}
	tail, err := intArg(args, "tail", 100)
	if err != nil {
		return nil, err
	}
	return s.waitRun(ctx, id, boundedSeconds(timeout, 600), tail)
}

// ---------- write tools ----------

func (s *Server) toolAddTask(ctx context.Context, args map[string]any) (any, error) {
	task, err := objectArg(args, "task")
	if err != nil {
		return nil, err
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/tasks", map[string]any{"task": task}, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolUpdateTask(ctx context.Context, args map[string]any) (any, error) {
	index, err := intArg(args, "index", -1)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, errors.New("index 不能为空")
	}
	task, err := objectArg(args, "task")
	if err != nil {
		return nil, err
	}
	var data map[string]any
	path := fmt.Sprintf("/api/tasks/%d", index)
	if err := s.api.put(ctx, path, map[string]any{"task": task}, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolDeleteTask(ctx context.Context, args map[string]any) (any, error) {
	if !boolArg(args, "confirm") {
		return nil, errors.New("删除任务需要 confirm=true")
	}
	index, err := intArg(args, "index", -1)
	if err != nil {
		return nil, err
	}
	if index < 0 {
		return nil, errors.New("index 不能为空")
	}
	var data map[string]any
	if err := s.api.delete(ctx, fmt.Sprintf("/api/tasks/%d", index), &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolSetCrontab(ctx context.Context, args map[string]any) (any, error) {
	raw, ok := args["crontab"]
	if !ok {
		return nil, errors.New("crontab 不能为空（传空字符串表示停用）")
	}
	crontab, ok := raw.(string)
	if !ok {
		return nil, errors.New("crontab 必须是字符串")
	}
	var data map[string]any
	if err := s.api.put(ctx, "/api/config", map[string]any{"crontab": strings.TrimSpace(crontab)}, &data); err != nil {
		return nil, err
	}
	return sanitizeConfig(data), nil
}

func (s *Server) toolRunTasks(ctx context.Context, args map[string]any) (any, error) {
	write := boolArg(args, "write")
	if write && !boolArg(args, "confirm") {
		return nil, errors.New("write=true 会真实转存与改名，必须先取得用户同意并传 confirm=true")
	}
	var indices []int
	if raw, ok := args["tasks"]; ok && raw != nil {
		items, ok := raw.([]any)
		if !ok {
			return nil, errors.New("tasks 必须是任务序号数组")
		}
		for _, item := range items {
			index, err := toInt(item)
			if err != nil {
				return nil, fmt.Errorf("tasks 含无效序号: %v", item)
			}
			indices = append(indices, index)
		}
	}
	task, hasTask := args["task"].(map[string]any)
	if hasTask && len(indices) > 0 {
		return nil, errors.New("task 与 tasks 不能同时指定")
	}
	body := map[string]any{"tasks": indices, "write": write}
	if hasTask {
		body["task"] = task
	}
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := s.api.post(ctx, "/api/run", body, &created); err != nil {
		return nil, err
	}
	result := map[string]any{"run_id": created.RunID, "write": write}
	if wait, err := intArg(args, "wait_seconds", 0); err != nil {
		return nil, err
	} else if wait > 0 {
		tail, err := intArg(args, "tail", 100)
		if err != nil {
			return nil, err
		}
		waited, err := s.waitRun(ctx, created.RunID, boundedSeconds(wait, 600), tail)
		if err != nil {
			return nil, err
		}
		result["wait"] = waited
	}
	return result, nil
}

func (s *Server) toolSignIn(ctx context.Context, args map[string]any) (any, error) {
	var created struct {
		RunID string `json:"run_id"`
	}
	if err := s.api.post(ctx, "/api/sign", map[string]any{}, &created); err != nil {
		return nil, err
	}
	result := map[string]any{"run_id": created.RunID}
	if wait, err := intArg(args, "wait_seconds", 0); err != nil {
		return nil, err
	} else if wait > 0 {
		tail, err := intArg(args, "tail", 50)
		if err != nil {
			return nil, err
		}
		waited, err := s.waitRun(ctx, created.RunID, boundedSeconds(wait, 600), tail)
		if err != nil {
			return nil, err
		}
		result["wait"] = waited
	}
	return result, nil
}

func (s *Server) toolRenameFile(ctx context.Context, args map[string]any) (any, error) {
	name := stringArg(args, "name")
	if name == "" {
		return nil, errors.New("name 不能为空")
	}
	body := map[string]any{"path": stringArg(args, "path"), "fid": stringArg(args, "fid"), "name": name}
	var data map[string]any
	if err := s.api.post(ctx, "/api/files/rename", body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolDeleteFile(ctx context.Context, args map[string]any) (any, error) {
	if !boolArg(args, "confirm") {
		return nil, errors.New("删除文件需要 confirm=true")
	}
	path := stringArg(args, "path")
	fid := stringArg(args, "fid")
	if path == "" && fid == "" {
		return nil, errors.New("需要 path 或 fid")
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/files/delete", map[string]any{
		"path": path, "fid": fid, "confirm": true,
	}, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolMkdir(ctx context.Context, args map[string]any) (any, error) {
	path := stringArg(args, "path")
	if path == "" {
		return nil, errors.New("path 不能为空")
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/files/mkdir", map[string]any{"path": path}, &data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *Server) toolNotifyTest(ctx context.Context, args map[string]any) (any, error) {
	body := map[string]any{
		"title":   stringArg(args, "title"),
		"content": stringArg(args, "content"),
	}
	var data map[string]any
	if err := s.api.post(ctx, "/api/notify/test", body, &data); err != nil {
		return nil, err
	}
	return data, nil
}

// ---------- helpers ----------

func (s *Server) fetchTasks(ctx context.Context) ([]map[string]any, error) {
	var payload struct {
		Tasks []map[string]any `json:"tasks"`
	}
	if err := s.api.get(ctx, "/api/tasks", nil, &payload); err != nil {
		return nil, err
	}
	return payload.Tasks, nil
}

func (s *Server) latestRunID(ctx context.Context) (string, error) {
	var latest map[string]any
	if err := s.api.get(ctx, "/api/runs/latest", nil, &latest); err != nil {
		return "", err
	}
	id, _ := latest["id"].(string)
	if id == "" {
		return "", errors.New("还没有运行记录")
	}
	return id, nil
}

func (s *Server) runLog(ctx context.Context, id string, tail int) (any, error) {
	if tail <= 0 {
		tail = 200
	}
	if tail > 2000 {
		tail = 2000
	}
	var payload struct {
		Run   map[string]any `json:"run"`
		Lines []string       `json:"lines"`
		Done  bool           `json:"done"`
	}
	if err := s.api.get(ctx, "/api/runs/"+url.PathEscape(id)+"/log", nil, &payload); err != nil {
		return nil, err
	}
	lines := payload.Lines
	if tail > 0 && len(lines) > tail {
		lines = lines[len(lines)-tail:]
	}
	return map[string]any{
		"run":   payload.Run,
		"done":  payload.Done,
		"lines": lines,
		"log":   strings.Join(lines, "\n"),
	}, nil
}

func (s *Server) waitRun(ctx context.Context, id string, timeoutSeconds, tail int) (any, error) {
	deadline := time.Now().Add(time.Duration(timeoutSeconds) * time.Second)
	for {
		var view map[string]any
		if err := s.api.get(ctx, "/api/runs/"+url.PathEscape(id), nil, &view); err != nil {
			return nil, err
		}
		done, _ := view["done"].(bool)
		if done || time.Now().After(deadline) {
			logData, err := s.runLog(ctx, id, tail)
			if err != nil {
				return map[string]any{"run": view, "timeout": !done}, nil
			}
			result, _ := logData.(map[string]any)
			result["timeout"] = !done
			return result, nil
		}
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}
	}
}

func boundedSeconds(value, max int) int {
	if value <= 0 {
		return 60
	}
	if value > max {
		return max
	}
	return value
}

func intArg(args map[string]any, key string, fallback int) (int, error) {
	raw, ok := args[key]
	if !ok || raw == nil {
		return fallback, nil
	}
	return toInt(raw)
}

func toInt(value any) (int, error) {
	switch typed := value.(type) {
	case float64:
		return int(typed), nil
	case int:
		return typed, nil
	case int64:
		return int(typed), nil
	case string:
		return strconv.Atoi(strings.TrimSpace(typed))
	}
	return 0, fmt.Errorf("需要整数，收到 %T", value)
}

func stringArg(args map[string]any, key string) string {
	value, _ := args[key].(string)
	return strings.TrimSpace(value)
}

func boolArg(args map[string]any, key string) bool {
	switch typed := args[key].(type) {
	case bool:
		return typed
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "true", "1", "yes", "on":
			return true
		}
	}
	return false
}

func objectArg(args map[string]any, key string) (map[string]any, error) {
	value, ok := args[key].(map[string]any)
	if !ok || value == nil {
		return nil, fmt.Errorf("%s 必须是对象", key)
	}
	return value, nil
}
