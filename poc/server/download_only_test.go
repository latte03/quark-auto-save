package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qas-poc/quark"
)

func TestValidateTaskDownloadOnly(t *testing.T) {
	// 仅下载任务：shareurl 可以为空，但必须开启本地下载。
	task := &quark.Task{
		TaskName:      "同步已有",
		SavePath:      "/tv/已有",
		DownloadOnly:  true,
		LocalDownload: &quark.TaskDownload{Enable: true, Dir: "/vol1/1000/media"},
	}
	if err := validateTask(task); err != nil {
		t.Fatalf("valid download-only task rejected: %v", err)
	}

	noDownload := &quark.Task{TaskName: "缺下载配置", SavePath: "/tv/已有", DownloadOnly: true}
	if err := validateTask(noDownload); err == nil || !strings.Contains(err.Error(), "下载到 NAS") {
		t.Fatalf("missing local_download error = %v", err)
	}

	// 普通任务仍要求分享链接。
	if err := validateTask(&quark.Task{TaskName: "普通", SavePath: "/tv"}); err == nil || !strings.Contains(err.Error(), "分享链接") {
		t.Fatalf("shareurl requirement error = %v", err)
	}
}

// newDownloadOnlyServer spins up a console whose cloud client is a fake quark
// API serving a source directory with two files; a.mkv already exists locally,
// b.mkv is missing so exactly one file should ever be queued.
func newDownloadOnlyServer(t *testing.T) (*Server, *httptest.Server, string) {
	t.Helper()
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.mkv"), []byte("existing"), 0o644); err != nil {
		t.Fatal(err)
	}
	mkdirs := 0
	var upstream *httptest.Server
	upstream = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/1/clouddrive/file/info/path_list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": []map[string]string{{"file_path": "/tv/源目录", "fid": "fid-src"}},
			})
		case r.Method == http.MethodGet && r.URL.Path == "/1/clouddrive/file/sort":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"metadata": map[string]any{"_total": 2},
				"data": map[string]any{"list": []map[string]any{
					{"fid": "fid-a", "file_name": "a.mkv", "size": 8},
					{"fid": "fid-b", "file_name": "b.mkv", "size": 15},
				}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/1/clouddrive/file/download":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 0,
				"data": []map[string]string{{"download_url": upstream.URL + "/files/b.mkv"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/1/clouddrive/file":
			mkdirs++
			t.Errorf("download-only task must not create cloud directories")
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": map[string]any{}})
		case r.Method == http.MethodGet && r.URL.Path == "/files/b.mkv":
			_, _ = io.WriteString(w, "fresh-episode")
		default:
			t.Errorf("unexpected upstream call %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(upstream.Close)

	config := fmt.Sprintf(`{
	  "cookie": ["__pus=only"],
	  "webui": {"username": "admin", "password": "test-pass"},
	  "crontab": "",
	  "local_download": {"dir": "/vol1/1000/media"},
	  "tasklist": [{
	    "taskname": "同步已有",
	    "savepath": "/tv/源目录",
	    "download_only": true,
	    "runweek": [1, 2, 3, 4, 5, 6, 7],
	    "local_download": {"enable": true, "dir": %q}
	  }]
	}`, root)

	instance, ts := newTestServerWithConfig(t, config)
	instance.newClient = func(cookie string) *quark.Client {
		client := quark.New(cookie)
		client.SetBaseURL(upstream.URL, upstream.URL)
		return client
	}
	// 下载队列拿到的 client 也要指向假上游：队列在入队时捕获 job.Client，
	// executeRun 传入的就是 instance.newClient 产出的实例，这里无需额外处理。
	originalValidator := localRootValidator
	localRootValidator = func(string) string { return "" }
	t.Cleanup(func() { localRootValidator = originalValidator })

	if mkdirs != 0 {
		t.Fatalf("unexpected mkdir calls: %d", mkdirs)
	}
	return instance, ts, root
}

func TestRunDownloadOnlyTask(t *testing.T) {
	instance, ts, root := newDownloadOnlyServer(t)
	client := loginClient(t, ts.URL)

	// 演练：只列出将下载的文件，不产生下载任务。
	response, body := postJSON(t, client, ts.URL+"/api/run", map[string]any{"tasks": []int{0}, "write": false})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("dry run = %d: %s", response.StatusCode, body)
	}
	var dry struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(body, &dry)
	dryLog := strings.Join(waitRun(t, instance, dry.RunID), "\n")
	if !strings.Contains(dryLog, "任务类型: 仅下载") || !strings.Contains(dryLog, "（演练）将下载 1 个文件") {
		t.Fatalf("dry log missing download plan:\n%s", dryLog)
	}
	if strings.Contains(dryLog, "已加入本地下载队列") {
		t.Fatalf("dry run must not enqueue downloads:\n%s", dryLog)
	}

	// 正式运行：云端有本地缺的一个文件被下载落地。
	response, body = postJSON(t, client, ts.URL+"/api/run", map[string]any{"tasks": []int{0}, "write": true})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("real run = %d: %s", response.StatusCode, body)
	}
	var real struct {
		RunID string `json:"run_id"`
	}
	_ = json.Unmarshal(body, &real)
	realLog := strings.Join(waitRun(t, instance, real.RunID), "\n")
	if !strings.Contains(realLog, "已加入本地下载队列：1 个文件") {
		t.Fatalf("run log missing enqueue line:\n%s", realLog)
	}

	// 下载是异步的，等文件落盘。
	for i := 0; i < 50; i++ {
		if _, err := os.Stat(filepath.Join(root, "b.mkv")); err == nil {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	content, err := os.ReadFile(filepath.Join(root, "b.mkv"))
	if err != nil {
		queueState, _ := client.Get(ts.URL + "/api/downloads")
		queueBody, _ := io.ReadAll(queueState.Body)
		queueState.Body.Close()
		t.Fatalf("downloaded file missing: %v\nlog:\n%s\nqueue:\n%s", err, realLog, queueBody)
	}
	if string(content) != "fresh-episode" {
		t.Fatalf("content = %q", content)
	}
	if _, err := os.Stat(filepath.Join(root, "a.mkv")); err != nil {
		t.Fatalf("existing file disturbed: %v", err)
	}

	// 运行统计应计入下载文件数。
	view, err := client.Get(ts.URL + "/api/runs/" + real.RunID)
	if err != nil {
		t.Fatal(err)
	}
	defer view.Body.Close()
	var payload struct {
		Data RunView `json:"data"`
	}
	if err := json.NewDecoder(view.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if got := payload.Data.Tasks["同步已有"]; got != 1 {
		t.Fatalf("stats = %d, want 1 (view = %+v)", got, payload.Data)
	}
}
