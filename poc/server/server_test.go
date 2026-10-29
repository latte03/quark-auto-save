package server

import (
	"bufio"
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"qas-poc/quark"
)

const testConfig = `{
  "cookie": ["kps=placeholder; sign=placeholder; vcode=placeholder"],
  "webui": {"username": "admin", "password": "test-pass"},
  "crontab": "0 8,18,20 * * *",
  "tasklist": []
}`

func newTestServer(t *testing.T) (*Server, *httptest.Server) {
	return newTestServerWithConfig(t, testConfig)
}

func newTestServerWithConfig(t *testing.T, config string) (*Server, *httptest.Server) {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	instance, err := New(Options{
		ConfigPath: path,
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

func loginClient(t *testing.T, base string) *http.Client {
	t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		t.Fatal(err)
	}
	client := &http.Client{Jar: jar, Timeout: 10 * time.Second}
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "test-pass"})
	response, err := client.Post(base+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d", response.StatusCode)
	}
	return client
}

func TestLoginAndSession(t *testing.T) {
	_, ts := newTestServer(t)
	client := &http.Client{Timeout: 5 * time.Second}

	response, err := client.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous /api/config = %d, want 401", response.StatusCode)
	}

	bad, _ := json.Marshal(map[string]string{"username": "admin", "password": "wrong"})
	response, err = client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("bad login = %d, want 401", response.StatusCode)
	}

	authed := loginClient(t, ts.URL)
	response, err = authed.Get(ts.URL + "/api/session")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var session struct {
		Authenticated bool   `json:"authenticated"`
		Version       string `json:"version"`
	}
	if err := json.NewDecoder(response.Body).Decode(&session); err != nil {
		t.Fatal(err)
	}
	if !session.Authenticated || session.Version != "test" {
		t.Fatalf("session = %+v", session)
	}
}

func TestConfigUpdateValidatesCrontab(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	put := func(payload map[string]any) int {
		body, _ := json.Marshal(payload)
		request, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		return response.StatusCode
	}

	if code := put(map[string]any{"crontab": "not a cron"}); code != http.StatusBadRequest {
		t.Fatalf("invalid crontab = %d, want 400", code)
	}
	if code := put(map[string]any{"crontab": "*/5 * * * *", "push_config": map[string]any{"CONSOLE": true}}); code != http.StatusOK {
		t.Fatalf("valid config = %d, want 200", code)
	}

	response, err := client.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Data struct {
			Crontab  string         `json:"crontab"`
			NextRuns []string       `json:"next_runs"`
			Plugins  map[string]any `json:"plugins"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Crontab != "*/5 * * * *" {
		t.Fatalf("crontab = %q", payload.Data.Crontab)
	}
	if len(payload.Data.NextRuns) != 3 {
		t.Fatalf("next_runs = %v", payload.Data.NextRuns)
	}
	if _, ok := payload.Data.Plugins["alist"]; !ok {
		t.Fatalf("plugins missing defaults: %v", payload.Data.Plugins)
	}
}

func TestConfigViewDefaultsForFreshInstall(t *testing.T) {
	_, ts := newTestServerWithConfig(t, `{"webui":{"username":"admin","password":"test-pass"}}`)
	client := loginClient(t, ts.URL)
	response, err := client.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Data struct {
			Cookie   []string                  `json:"cookie"`
			NextRuns []string                  `json:"next_runs"`
			Push     map[string]any            `json:"push_config"`
			Magic    []map[string]any          `json:"magic_regex"`
			Source   map[string]map[string]any `json:"source"`
			Plugins  map[string]any            `json:"plugins"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data.Cookie == nil || payload.Data.NextRuns == nil {
		t.Fatalf("arrays decoded as null: cookie=%v next_runs=%v", payload.Data.Cookie, payload.Data.NextRuns)
	}
	if payload.Data.Push == nil || payload.Data.Magic == nil || payload.Data.Plugins == nil {
		t.Fatalf("objects decoded as null: push=%v magic=%v plugins=%v", payload.Data.Push, payload.Data.Magic, payload.Data.Plugins)
	}
	if _, ok := payload.Data.Source["cloudsaver"]["enable"]; !ok {
		t.Fatalf("source.cloudsaver missing defaults: %v", payload.Data.Source)
	}
	if _, ok := payload.Data.Source["pansou"]["enable"]; !ok {
		t.Fatalf("source.pansou missing defaults: %v", payload.Data.Source)
	}
}

func TestTaskCRUD(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	post := func(payload map[string]any) *http.Response {
		body, _ := json.Marshal(payload)
		response, err := client.Post(ts.URL+"/api/tasks", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		return response
	}

	invalid := post(map[string]any{"task": map[string]any{"taskname": "坏任务", "shareurl": "https://example.com/x", "savepath": "/tv"}})
	invalid.Body.Close()
	if invalid.StatusCode != http.StatusBadRequest {
		t.Fatalf("invalid task = %d, want 400", invalid.StatusCode)
	}

	task := map[string]any{
		"taskname": "黑镜",
		"shareurl": "https://pan.quark.cn/s/abc#/list/share/" + strings.Repeat("a", 32) + "-Black%20Mirror",
		"savepath": "video/tv/Black Mirror",
		"pattern":  "$TV",
		"runweek":  []int{1, 2, 3},
		"status":   "tracking",
		"addition": map[string]any{"emby": map[string]any{"try_match": true}},
	}
	created := post(map[string]any{"task": task})
	if created.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(created.Body)
		created.Body.Close()
		t.Fatalf("create task = %d: %s", created.StatusCode, body)
	}
	created.Body.Close()

	response, err := client.Get(ts.URL + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var list struct {
		Data struct {
			Tasks   []map[string]any `json:"tasks"`
			Crontab string           `json:"crontab"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Data.Tasks) != 1 {
		t.Fatalf("task count = %d", len(list.Data.Tasks))
	}
	stored := list.Data.Tasks[0]
	if stored["savepath"] != "/video/tv/Black Mirror" {
		t.Fatalf("savepath = %v", stored["savepath"])
	}
	if _, leaked := stored["status"]; !leaked {
		t.Fatalf("view missing status: %v", stored)
	}
	if _, leaked := stored["index"]; !leaked {
		t.Fatalf("view missing index: %v", stored)
	}
	if _, ok := stored["addition"]; !ok {
		t.Fatalf("view missing addition: %v", stored)
	}
	if _, ok := stored["runweek"]; !ok {
		t.Fatalf("view missing runweek: %v", stored)
	}

	// Updating must not persist UI-only keys back into the config.
	update := map[string]any{}
	for key, value := range stored {
		update[key] = value
	}
	update["taskname"] = "黑镜 第七季"
	body, _ := json.Marshal(map[string]any{"task": update})
	request, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/tasks/0", bytes.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	updated, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	updated.Body.Close()
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("update = %d", updated.StatusCode)
	}

	deleteRequest, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/tasks/0", nil)
	deleted, err := client.Do(deleteRequest)
	if err != nil {
		t.Fatal(err)
	}
	deleted.Body.Close()
	if deleted.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d", deleted.StatusCode)
	}
}

func TestRunStreamReplaysLog(t *testing.T) {
	instance, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	run, err := instance.runs.Start("测试运行", true, func(run *Run) error {
		run.Logf("第一行")
		run.Logf("第二行")
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !run.View().Done {
		time.Sleep(10 * time.Millisecond)
	}

	response, err := client.Get(ts.URL + "/api/runs/" + run.ID + "/stream")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()

	var lines []string
	scanner := bufio.NewScanner(response.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "data: ") {
			line = strings.TrimPrefix(line, "data: ")
			if line == "[DONE]" {
				break
			}
			lines = append(lines, line)
		}
	}
	if len(lines) != 2 || lines[0] != "第一行" || lines[1] != "第二行" {
		t.Fatalf("stream lines = %v", lines)
	}

	latest, err := client.Get(ts.URL + "/api/runs/latest")
	if err != nil {
		t.Fatal(err)
	}
	defer latest.Body.Close()
	var payload struct {
		Data *RunView `json:"data"`
	}
	if err := json.NewDecoder(latest.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data == nil || !payload.Data.Done || payload.Data.Label != "测试运行" {
		t.Fatalf("latest run = %+v", payload.Data)
	}
}

func TestPluginUpdatePreservesUnknownSections(t *testing.T) {
	instance, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	put := func(payload map[string]any) {
		body, _ := json.Marshal(payload)
		request, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/config", bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		response.Body.Close()
		if response.StatusCode != http.StatusOK {
			t.Fatalf("put config = %d", response.StatusCode)
		}
	}

	put(map[string]any{"plugins": map[string]any{"emby": map[string]any{"url": "http://emby"}}})
	put(map[string]any{"plugins": map[string]any{"alist": map[string]any{"url": "http://alist", "token": "t"}}})

	raw, err := os.ReadFile(instance.store.path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		Plugins map[string]map[string]any `json:"plugins"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.Plugins["emby"]["url"] != "http://emby" {
		t.Fatalf("emby section lost: %v", doc.Plugins)
	}
	if doc.Plugins["alist"]["token"] != "t" {
		t.Fatalf("alist section missing: %v", doc.Plugins)
	}

	response, err := client.Get(ts.URL + "/api/config")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var view struct {
		Data struct {
			Plugins map[string]any `json:"plugins"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&view); err != nil {
		t.Fatal(err)
	}
	if _, ok := view.Data.Plugins["emby"]; ok {
		t.Fatalf("unported plugin leaked into settings: %v", view.Data.Plugins)
	}
	if _, ok := view.Data.Plugins["alist"]; !ok {
		t.Fatalf("ported plugin missing from settings: %v", view.Data.Plugins)
	}
}

func TestTaskWithoutRunweekDefaultsToAllDays(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	body, _ := json.Marshal(map[string]any{"task": map[string]any{
		"taskname": "每天运行",
		"shareurl": "https://pan.quark.cn/s/abc",
		"savepath": "/tv/daily",
	}})
	response, err := client.Post(ts.URL+"/api/tasks", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("create = %d", response.StatusCode)
	}

	list, err := client.Get(ts.URL + "/api/tasks")
	if err != nil {
		t.Fatal(err)
	}
	defer list.Body.Close()
	var payload struct {
		Data struct {
			Tasks []struct {
				RunWeek []int `json:"runweek"`
			} `json:"tasks"`
		} `json:"data"`
	}
	if err := json.NewDecoder(list.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Tasks) != 1 {
		t.Fatalf("tasks = %d", len(payload.Data.Tasks))
	}
	if days := payload.Data.Tasks[0].RunWeek; len(days) != 7 {
		t.Fatalf("runweek = %v, want every day", days)
	}
}

func TestRenamePreview(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	body, _ := json.Marshal(map[string]any{
		"taskname": "黑镜",
		"pattern":  "$TV",
		"replace":  "",
		"files":    []string{"Show.Name.S01E03.mkv", "readme.txt"},
	})
	response, err := client.Post(ts.URL+"/api/rename/preview", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		payload, _ := io.ReadAll(response.Body)
		t.Fatalf("preview = %d: %s", response.StatusCode, payload)
	}
	var payload struct {
		Data struct {
			Files []struct {
				Name    string `json:"name"`
				Matched bool   `json:"matched"`
				Renamed string `json:"renamed"`
			} `json:"files"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	files := payload.Data.Files
	if len(files) != 2 {
		t.Fatalf("files = %v", files)
	}
	if !files[0].Matched || files[0].Renamed != "S01E03.mkv" {
		t.Fatalf("episode preview = %+v", files[0])
	}
	if files[1].Matched {
		t.Fatalf("readme should not match: %+v", files[1])
	}
}

func TestBrowseRequiresCookie(t *testing.T) {
	instance, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	if err := instance.store.Update(map[string]json.RawMessage{"cookie": json.RawMessage(`[]`)}); err != nil {
		t.Fatal(err)
	}
	response, err := client.Get(ts.URL + "/api/browse?path=/")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("browse without cookie = %d, want 400", response.StatusCode)
	}
}

func TestSearchWithoutConfiguredSources(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)
	response, err := client.Get(ts.URL + "/api/search?keyword=test")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Message, "未配置搜索源") {
		t.Fatalf("message = %q", payload.Message)
	}
}

func TestSearchSkipsIncompleteCloudSaver(t *testing.T) {
	config := `{
  "webui": {"username": "admin", "password": "test-pass"},
  "crontab": "",
  "tasklist": [],
  "source": {
    "cloudsaver": {"enable": true, "server": "http://127.0.0.1:9", "username": "", "password": "", "token": ""},
    "pansou": {"enable": false, "server": ""}
  }
}`
	_, ts := newTestServerWithConfig(t, config)
	client := loginClient(t, ts.URL)
	response, err := client.Get(ts.URL + "/api/search?keyword=test")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Message, "缺少账号密码或 Token") {
		t.Fatalf("message = %q", payload.Message)
	}
}

func TestSearchRequiresKeyword(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)
	response, err := client.Get(ts.URL + "/api/search?keyword=")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("empty keyword = %d, want 400", response.StatusCode)
	}
}

func TestShareDetailRejectsInvalidPdirFID(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)
	body := strings.NewReader(`{"shareurl":"https://pan.quark.cn/s/abcdef","pdir_fid":"../bad"}`)
	response, err := client.Post(ts.URL+"/api/share/detail", "application/json", body)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
	var payload struct {
		Message string `json:"message"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(payload.Message, "目录标识") {
		t.Fatalf("message = %q", payload.Message)
	}
}

func TestSignEndpointRunsSignOnlyJob(t *testing.T) {
	config := `{
  "cookie": ["__pus=only"],
  "webui": {"username": "admin", "password": "test-pass"},
  "crontab": "",
  "tasklist": []
}`
	instance, ts := newTestServerWithConfig(t, config)
	client := loginClient(t, ts.URL)

	response, err := client.Post(ts.URL+"/api/sign", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("sign status = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		run := instance.runs.Get(payload.RunID)
		if run == nil {
			t.Fatal("run not found")
		}
		if view := run.View(); view.Done {
			if view.Error != "" {
				t.Fatalf("run error = %s", view.Error)
			}
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("sign run did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}

	lines, _ := instance.runs.Get(payload.RunID).LinesFrom(0)
	log := strings.Join(lines, "\n")
	for _, want := range []string{"签到任务", "判断为仅签到", "签到账号 1 个，新签 0 个", "程序结束"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log missing %q:\n%s", want, log)
		}
	}
}

func TestNotifyTestEndpoint(t *testing.T) {
	config := `{
  "cookie": ["__pus=only"],
  "webui": {"username": "admin", "password": "test-pass"},
  "crontab": "",
  "push_config": {"CONSOLE": true},
  "tasklist": []
}`
	_, ts := newTestServerWithConfig(t, config)
	client := loginClient(t, ts.URL)

	response, err := client.Post(ts.URL+"/api/notify/test", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(response.Body)
		t.Fatalf("notify status = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		Data struct {
			Channels []struct {
				Channel string `json:"channel"`
				OK      bool   `json:"ok"`
			} `json:"channels"`
			Log string `json:"log"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Data.Channels) != 1 || payload.Data.Channels[0].Channel != "Console" || !payload.Data.Channels[0].OK {
		t.Fatalf("channels = %#v", payload.Data.Channels)
	}
	if !strings.Contains(payload.Data.Log, "通知测试") {
		t.Fatalf("log = %q", payload.Data.Log)
	}
}

func TestNotifyTestWithoutChannels(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)
	response, err := client.Post(ts.URL+"/api/notify/test", "application/json", strings.NewReader(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", response.StatusCode)
	}
}

// ---------- file management ----------

type fileAPIRecorder struct {
	deleted []string
	renamed string
	mkdir   string
}

func newFileTestServer(t *testing.T) (*Server, *httptest.Server, *fileAPIRecorder) {
	t.Helper()
	recorder := &fileAPIRecorder{}
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body map[string]any
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/info/path_list":
			paths, _ := body["file_path"].([]any)
			items := make([]map[string]string, 0, len(paths))
			for _, item := range paths {
				items = append(items, map[string]string{
					"file_path": item.(string),
					"fid":       "fid-" + strings.TrimPrefix(item.(string), "/"),
				})
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": items})
		case "/1/clouddrive/file/delete":
			raw, _ := body["filelist"].([]any)
			for _, item := range raw {
				recorder.deleted = append(recorder.deleted, item.(string))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": map[string]any{"task_id": "t-1"}})
		case "/1/clouddrive/file/rename":
			recorder.renamed = body["file_name"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": map[string]any{}})
		case "/1/clouddrive/file":
			recorder.mkdir = body["dir_path"].(string)
			_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 0, "data": map[string]any{"fid": "dir-1"}})
		default:
			t.Errorf("unexpected upstream path %s", r.URL.Path)
		}
	}))
	t.Cleanup(upstream.Close)

	instance, ts := newTestServer(t)
	instance.newClient = func(cookie string) *quark.Client {
		client := quark.New(cookie)
		client.SetBaseURL(upstream.URL, upstream.URL)
		return client
	}
	return instance, ts, recorder
}

func postJSON(t *testing.T, client *http.Client, url string, payload any) (*http.Response, []byte) {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	response, err := client.Post(url, "application/json", bytes.NewReader(raw))
	if err != nil {
		t.Fatal(err)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	return response, body
}

func TestDeleteFileRequiresConfirmAndRejectsRoot(t *testing.T) {
	_, ts, recorder := newFileTestServer(t)
	client := loginClient(t, ts.URL)

	response, _ := postJSON(t, client, ts.URL+"/api/files/delete", map[string]any{"path": "/video/a.mp4"})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("without confirm = %d, want 400", response.StatusCode)
	}

	response, _ = postJSON(t, client, ts.URL+"/api/files/delete", map[string]any{"path": "/", "confirm": true})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("root delete = %d, want 400", response.StatusCode)
	}

	response, body := postJSON(t, client, ts.URL+"/api/files/delete", map[string]any{"path": "/video/a.mp4", "confirm": true})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("delete = %d: %s", response.StatusCode, body)
	}
	if len(recorder.deleted) != 1 || recorder.deleted[0] != "fid-video/a.mp4" {
		t.Fatalf("deleted = %#v", recorder.deleted)
	}
}

func TestRenameFile(t *testing.T) {
	_, ts, recorder := newFileTestServer(t)
	client := loginClient(t, ts.URL)

	for _, name := range []string{"", "a/b", ".."} {
		response, _ := postJSON(t, client, ts.URL+"/api/files/rename", map[string]any{"path": "/video/a.mp4", "name": name})
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("rename %q = %d, want 400", name, response.StatusCode)
		}
	}

	response, body := postJSON(t, client, ts.URL+"/api/files/rename", map[string]any{"path": "/video/a.mp4", "name": "b.mp4"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("rename = %d: %s", response.StatusCode, body)
	}
	if recorder.renamed != "b.mp4" {
		t.Fatalf("renamed = %q", recorder.renamed)
	}
}

func TestMkdir(t *testing.T) {
	_, ts, recorder := newFileTestServer(t)
	client := loginClient(t, ts.URL)

	response, _ := postJSON(t, client, ts.URL+"/api/files/mkdir", map[string]any{"path": "/"})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("root mkdir = %d, want 400", response.StatusCode)
	}

	response, body := postJSON(t, client, ts.URL+"/api/files/mkdir", map[string]any{"path": "/video/新目录/"})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("mkdir = %d: %s", response.StatusCode, body)
	}
	if recorder.mkdir != "/video/新目录" {
		t.Fatalf("mkdir path = %q", recorder.mkdir)
	}
}

func TestFileOperationsBlockedWhileRunning(t *testing.T) {
	instance, ts, _ := newFileTestServer(t)
	client := loginClient(t, ts.URL)

	started := make(chan struct{})
	release := make(chan struct{})
	run, err := instance.runs.Start("占位", true, func(*Run) error {
		close(started)
		<-release
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	<-started

	response, _ := postJSON(t, client, ts.URL+"/api/files/delete", map[string]any{"path": "/video/a.mp4", "confirm": true})
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("delete while running = %d, want 409", response.StatusCode)
	}
	close(release)
	waitRun(t, instance, run.ID)
}

// ---------- inline task ----------

func waitRun(t *testing.T, instance *Server, id string) []string {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		run := instance.runs.Get(id)
		if run == nil {
			t.Fatal("run not found")
		}
		if run.View().Done {
			lines, _ := run.LinesFrom(0)
			return lines
		}
		if time.Now().After(deadline) {
			t.Fatal("run did not finish")
		}
		time.Sleep(20 * time.Millisecond)
	}
}

func TestInlineTaskRunDoesNotPersist(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"status": 200, "code": 1, "message": "boom"})
	}))
	defer upstream.Close()

	instance, ts := newTestServer(t)
	instance.newClient = func(cookie string) *quark.Client {
		client := quark.New(cookie)
		client.SetBaseURL(upstream.URL, upstream.URL)
		return client
	}
	client := loginClient(t, ts.URL)

	response, body := postJSON(t, client, ts.URL+"/api/run", map[string]any{
		"write": false,
		"task": map[string]any{
			"taskname": "内联测试",
			"shareurl": "https://pan.quark.cn/s/abc123",
			"savepath": "/tmp-inline",
			"pattern":  ".*",
		},
	})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("inline run = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	log := strings.Join(waitRun(t, instance, payload.RunID), "\n")
	for _, want := range []string{"临时任务：执行后不写入任务列表", "任务名称: 内联测试", "❌《内联测试》"} {
		if !strings.Contains(log, want) {
			t.Fatalf("log missing %q:\n%s", want, log)
		}
	}
	if tasks := instance.store.Snapshot().Tasks; len(tasks) != 0 {
		t.Fatalf("inline run persisted %d tasks", len(tasks))
	}
}

func TestInlineTaskValidation(t *testing.T) {
	_, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	response, _ := postJSON(t, client, ts.URL+"/api/run", map[string]any{
		"task": map[string]any{"taskname": "坏任务", "shareurl": "https://pan.quark.cn/s/abc123"},
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("missing savepath = %d, want 400", response.StatusCode)
	}

	response, _ = postJSON(t, client, ts.URL+"/api/run", map[string]any{
		"tasks": []int{0},
		"task":  map[string]any{"taskname": "x", "shareurl": "https://pan.quark.cn/s/abc123", "savepath": "/x"},
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("task+tasks = %d, want 400", response.StatusCode)
	}
}

func TestRunLogEndpoint(t *testing.T) {
	config := `{
  "cookie": ["__pus=only"],
  "webui": {"username": "admin", "password": "test-pass"},
  "crontab": "",
  "tasklist": []
}`
	instance, ts := newTestServerWithConfig(t, config)
	client := loginClient(t, ts.URL)

	response, body := postJSON(t, client, ts.URL+"/api/sign", map[string]any{})
	if response.StatusCode != http.StatusOK {
		t.Fatalf("sign = %d: %s", response.StatusCode, body)
	}
	var payload struct {
		RunID string `json:"run_id"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatal(err)
	}
	waitRun(t, instance, payload.RunID)

	response, getErr := client.Get(ts.URL + "/api/runs/" + payload.RunID + "/log")
	if getErr != nil {
		t.Fatal(getErr)
	}
	body, _ = io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("log = %d: %s", response.StatusCode, body)
	}
	var logPayload struct {
		Data struct {
			Done  bool     `json:"done"`
			Lines []string `json:"lines"`
		} `json:"data"`
	}
	if err := json.Unmarshal(body, &logPayload); err != nil {
		t.Fatal(err)
	}
	if !logPayload.Data.Done || len(logPayload.Data.Lines) == 0 {
		t.Fatalf("log payload = %#v", logPayload.Data)
	}
	if !strings.Contains(strings.Join(logPayload.Data.Lines, "\n"), "签到任务") {
		t.Fatalf("lines = %#v", logPayload.Data.Lines)
	}
}

func TestAPIKeyMatchesLoginToken(t *testing.T) {
	instance, ts := newTestServer(t)
	client := &http.Client{Timeout: 5 * time.Second}
	body, _ := json.Marshal(map[string]string{"username": "admin", "password": "test-pass"})
	response, err := client.Post(ts.URL+"/api/login", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var payload struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	username, password := instance.store.WebUI()
	if got := APIKey(username, password); got != payload.Token {
		t.Fatalf("APIKey = %q, login token = %q", got, payload.Token)
	}
}
