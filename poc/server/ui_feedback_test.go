package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qas-poc/quark"
)

// failingUpstream turns every quark API call into an error so a write run
// finishes "successfully" while every transfer task inside it fails (the share
// gets marked banned, which counts as a failed task).
func failingUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 1, "message": "boom"})
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

// saveFailingUpstream lets share resolution succeed but fails the transfer, so
// DoSaveTask returns a real error instead of only marking the share banned.
func saveFailingUpstream(t *testing.T) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case strings.HasSuffix(r.URL.Path, "/share/sharepage/token"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": map[string]any{"stoken": "st", "title": "分享"},
			})
		case strings.HasSuffix(r.URL.Path, "/share/sharepage/detail"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"metadata": map[string]any{"_total": 1},
				"data": map[string]any{"list": []map[string]any{
					{"fid": "f1", "share_fid_token": "tk", "file_name": "a.mkv", "size": 1},
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/file/info/path_list"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": []map[string]string{{"file_path": "/tv/a", "fid": "dir-a"}, {"file_path": "/tv/b", "fid": "dir-b"}},
			})
		case strings.HasSuffix(r.URL.Path, "/file/sort"):
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"metadata": map[string]any{"_total": 0},
				"data":     map[string]any{"list": []any{}},
			})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 1, "message": "save boom"})
		}
	}))
	t.Cleanup(upstream.Close)
	return upstream
}

func TestRunReportsFailedTasks(t *testing.T) {
	upstream := failingUpstream(t)
	config := `{
	  "cookie": ["__pus=only"],
	  "webui": {"username": "admin", "password": "test-pass"},
	  "crontab": "",
	  "tasklist": [
	    {"taskname": "会失败的任务", "shareurl": "https://pan.quark.cn/s/abc", "savepath": "/tv/x"}
	  ]
	}`
	instance, ts := newTestServerWithConfig(t, config)
	instance.newClient = func(cookie string) *quark.Client {
		client := quark.New(cookie)
		client.SetBaseURL(upstream.URL, upstream.URL)
		return client
	}
	client := loginClient(t, ts.URL)

	response, body := postJSON(t, client, ts.URL+"/api/run", map[string]any{"write": true})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("run = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(body, &payload)
	waitRun(t, instance, payload.RunID)

	view := instance.runs.Get(payload.RunID).View()
	if !view.Done {
		t.Fatal("run should be done")
	}
	if view.FailedTasks != 1 {
		t.Fatalf("failed_tasks = %d, want 1 (view = %+v)", view.FailedTasks, view)
	}
}

func TestTaskStatsPersistAcrossRestart(t *testing.T) {
	upstream := failingUpstream(t)
	dataDir := t.TempDir()
	// 仅下载任务无论云端结果如何都会写入 stats（本例为 0），适合验证落盘。
	config := `{
	  "cookie": ["__pus=only"],
	  "webui": {"username": "admin", "password": "test-pass"},
	  "crontab": "",
	  "tasklist": [
	    {"taskname": "统计任务", "savepath": "/tv/已有", "download_only": true, "local_download": {"enable": true, "dir": "/vol1/1000/media"}}
	  ]
	}`
	instance, ts := newTestServerWithConfigAndDataDir(t, config, dataDir)
	instance.newClient = func(cookie string) *quark.Client {
		client := quark.New(cookie)
		client.SetBaseURL(upstream.URL, upstream.URL)
		return client
	}
	client := loginClient(t, ts.URL)

	response, body := postJSON(t, client, ts.URL+"/api/run", map[string]any{"write": true})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("run = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(body, &payload)
	waitRun(t, instance, payload.RunID)

	// 新开一个 Server 实例模拟重启：stats 应来自落盘的 task_stats.json。
	restarted, _ := newTestServerWithConfigAndDataDir(t, config, dataDir)
	stats := restarted.taskStats.all()
	stat, ok := stats["统计任务"]
	if !ok {
		t.Fatalf("stats not persisted: %#v", stats)
	}
	if stat.Saved != 0 || stat.At.IsZero() {
		t.Fatalf("stat = %#v", stat)
	}
}

func TestDisabledTaskStaysOutOfScheduleButRunnable(t *testing.T) {
	now := time.Date(2026, 11, 14, 10, 0, 0, 0, time.Local)
	disabled := &quark.Task{
		TaskName: "停用的任务",
		ShareURL: "https://pan.quark.cn/s/abc",
		SavePath: "/tv/paused",
		RunWeek:  []int{1, 2, 3, 4, 5, 6, 7},
		RunTime:  []string{"10:00"},
		Disabled: true,
	}
	if status := statusOf(disabled, now); status != "paused" {
		t.Fatalf("status = %q, want paused", status)
	}
	if indices := dueTaskIndices([]*quark.Task{disabled}, "", now); len(indices) != 0 {
		t.Fatalf("disabled task scheduled: %v", indices)
	}
	if indices := legacyTaskIndices([]*quark.Task{disabled}, "0 10 * * *", now); len(indices) != 0 {
		t.Fatalf("disabled task in legacy schedule: %v", indices)
	}
	if times := nextTimesForTask(disabled, "", now, 3); len(times) != 0 {
		t.Fatalf("disabled task has next times: %v", times)
	}
	if label, _ := nextRunInfo(disabled, "", now); label != "—" {
		t.Fatalf("next_run label = %q", label)
	}

	// 序列化：true 落盘，false 不落盘。
	var task quark.Task
	if err := json.Unmarshal([]byte(`{"taskname":"x","disabled":true}`), &task); err != nil {
		t.Fatal(err)
	}
	if !task.Disabled {
		t.Fatal("disabled should parse")
	}
	encoded, err := json.Marshal(&task)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), `"disabled":true`) {
		t.Fatalf("encoded = %s", encoded)
	}
	var plain quark.Task
	if err := json.Unmarshal([]byte(`{"taskname":"y"}`), &plain); err != nil {
		t.Fatal(err)
	}
	plainEncoded, err := json.Marshal(&plain)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(plainEncoded), "disabled") {
		t.Fatalf("false disabled should not persist: %s", plainEncoded)
	}
}

func TestRunAllSkipsDisabledButManualRunWorks(t *testing.T) {
	upstream := saveFailingUpstream(t)
	config := `{
	  "cookie": ["__pus=only"],
	  "webui": {"username": "admin", "password": "test-pass"},
	  "crontab": "",
	  "tasklist": [
	    {"taskname": "停用", "shareurl": "https://pan.quark.cn/s/abc", "savepath": "/tv/a", "pattern": ".*", "disabled": true},
	    {"taskname": "启用", "shareurl": "https://pan.quark.cn/s/abc", "savepath": "/tv/b", "pattern": ".*"}
	  ]
	}`
	instance, ts := newTestServerWithConfig(t, config)
	instance.newClient = func(cookie string) *quark.Client {
		client := quark.New(cookie)
		client.SetBaseURL(upstream.URL, upstream.URL)
		return client
	}
	client := loginClient(t, ts.URL)

	// 「运行全部」应跳过停用任务：只有启用任务出现在运行里。
	response, body := postJSON(t, client, ts.URL+"/api/run", map[string]any{"write": true})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("run all = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(body, &payload)
	log := strings.Join(waitRun(t, instance, payload.RunID), "\n")
	if strings.Contains(log, "任务名称: 停用") {
		t.Fatalf("disabled task ran with run-all:\n%s", log)
	}
	if !strings.Contains(log, "任务名称: 启用") {
		t.Fatalf("enabled task missing:\n%s", log)
	}

	// 手动指定序号运行停用任务仍然允许。
	response, body = postJSON(t, client, ts.URL+"/api/run", map[string]any{"tasks": []int{0}, "write": true})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("manual run = %d: %s", response.StatusCode, body)
	}
	_ = json.Unmarshal(body, &payload)
	log = strings.Join(waitRun(t, instance, payload.RunID), "\n")
	t.Logf("MANUAL RUN LOG:\n%s", log)
	if !strings.Contains(log, "任务名称: 停用") {
		t.Fatalf("manual run of disabled task blocked:\n%s", log)
	}
	if view := instance.runs.Get(payload.RunID).View(); view.FailedTasks != 1 {
		t.Fatalf("failed_tasks = %d, want 1", view.FailedTasks)
	}
}

func newTestServerWithConfigAndDataDir(t *testing.T, config, dataDir string) (*Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Options{
		ConfigPath: path,
		DataDir:    dataDir,
		Version:    "test",
		Logger:     log.New(io.Discard, "", 0),
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if instance.scheduler != nil && instance.scheduler.current != nil {
			instance.scheduler.current.Stop()
		}
	})
	return instance, httptest.NewServer(instance.Handler())
}
