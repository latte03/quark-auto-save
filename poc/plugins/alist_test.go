package plugins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qas-poc/quark"
)

func pluginTask(t *testing.T, savepath, addition string) *quark.Task {
	t.Helper()
	raw := fmt.Sprintf(`{"taskname":"测试任务","shareurl":"https://pan.quark.cn/s/abc","savepath":%q,"addition":%s}`, savepath, addition)
	var task quark.Task
	if err := task.UnmarshalJSON([]byte(raw)); err != nil {
		t.Fatal(err)
	}
	return &task
}

func testDeps(t *testing.T) (Deps, *[]string) {
	t.Helper()
	var logs []string
	deps := Deps{
		Log: func(format string, args ...any) {
			logs = append(logs, fmt.Sprintf(format, args...))
		},
	}
	return deps, &logs
}

func hasLog(logs []string, substring string) bool {
	for _, line := range logs {
		if strings.Contains(line, substring) {
			return true
		}
	}
	return false
}

type fsCall struct {
	Path    string
	Refresh bool
}

func writeAlist(w http.ResponseWriter, code int, message string, content []alistFile) {
	payload := map[string]any{
		"code":    code,
		"message": message,
		"data":    map[string]any{"content": content},
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func TestAlistRefreshesTaskPath(t *testing.T) {
	var calls []fsCall
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/admin/setting/list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 200,
				"data": []map[string]any{{"value": "QuarkSave"}, {"value": "v3.0.0"}},
			})
		case "/api/fs/list":
			var body struct {
				Path    string `json:"path"`
				Refresh bool   `json:"refresh"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			calls = append(calls, fsCall{Path: body.Path, Refresh: body.Refresh})
			writeAlist(w, 200, "success", nil)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps, logs := testDeps(t)
	section := []byte(fmt.Sprintf(`{"url":%q,"token":"token","storage_id":"/mnt:/"}`, server.URL))
	plugin := NewAlist(section, deps)
	if !plugin.Active() {
		t.Fatalf("plugin not active, logs: %v", *logs)
	}
	task := pluginTask(t, "/tv/剧集", "null")
	if err := plugin.Run(task, nil); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 2 || calls[0] != (fsCall{"/mnt", false}) || calls[1] != (fsCall{"/mnt/tv/剧集", true}) {
		t.Fatalf("fs calls = %#v", calls)
	}
	if !hasLog(*logs, "目录[/mnt/tv/剧集] 成功✅") {
		t.Fatalf("logs = %v", *logs)
	}
}

func TestAlistRefreshWalksToParent(t *testing.T) {
	var calls []fsCall
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/admin/setting/list":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": []map[string]any{{"value": "x"}, {"value": "y"}}})
		case "/api/fs/list":
			var body struct {
				Path    string `json:"path"`
				Refresh bool   `json:"refresh"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			calls = append(calls, fsCall{Path: body.Path, Refresh: body.Refresh})
			if body.Refresh && body.Path == "/mnt/tv/show" {
				writeAlist(w, 500, "object not found", nil)
				return
			}
			writeAlist(w, 200, "success", nil)
		}
	}))
	defer server.Close()

	deps, logs := testDeps(t)
	section := []byte(fmt.Sprintf(`{"url":%q,"token":"token","storage_id":"/mnt:/"}`, server.URL))
	plugin := NewAlist(section, deps)
	if !plugin.Active() {
		t.Fatalf("plugin not active: %v", *logs)
	}
	if err := plugin.Run(pluginTask(t, "/tv/show", "null"), nil); err != nil {
		t.Fatal(err)
	}
	if !hasLog(*logs, "转父目录 [/mnt/tv]") || !hasLog(*logs, "目录[/mnt/tv] 成功✅") {
		t.Fatalf("logs = %v", *logs)
	}
	found := false
	for _, call := range calls {
		if call == (fsCall{"/mnt/tv", true}) {
			found = true
		}
	}
	if !found {
		t.Fatalf("parent refresh not requested: %#v", calls)
	}
}

func TestAlistStrmRunsSelectedConfigs(t *testing.T) {
	var submitted map[string][]string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/configs":
			if cookie := r.Header.Get("Cookie"); cookie != "session=abc" {
				t.Errorf("cookie = %q", cookie)
			}
			fmt.Fprint(w, `<option value="1"> <strong>名称:</strong>剧集任务</option>`+
				`<option value="2"> <strong>名称:</strong>电影</option>`)
		case "/run_selected_configs":
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			submitted = r.PostForm
			fmt.Fprint(w, `<div role="alert"> 已开始运行 <button>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps, logs := testDeps(t)
	section := []byte(fmt.Sprintf(`{"url":%q,"cookie":"session=abc","config_id":"1,2"}`, server.URL))
	plugin := NewAlistStrm(section, deps)
	if !plugin.Active() {
		t.Fatalf("not active: %v", *logs)
	}
	if err := plugin.Run(pluginTask(t, "/tv", "null"), nil); err != nil {
		t.Fatal(err)
	}
	if got := submitted["selected_configs"]; len(got) != 2 || got[0] != "1" || got[1] != "2" {
		t.Fatalf("selected_configs = %v", got)
	}
	if got := submitted["action"]; len(got) != 1 || got[0] != "run_selected" {
		t.Fatalf("action = %v", got)
	}
	if !hasLog(*logs, "已开始运行✅") {
		t.Fatalf("logs = %v", *logs)
	}
}

func TestAlistStrmGenWritesStrmFiles(t *testing.T) {
	files := map[string][]alistFile{
		"/mnt":        {},
		"/mnt/tv":     {{Name: "video.mkv", Sign: "abc"}, {Name: "sub", IsDir: true}, {Name: "note.txt"}},
		"/mnt/tv/sub": {{Name: "inside.mp4"}},
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/fs/list" {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		writeAlist(w, 200, "success", files[body.Path])
	}))
	defer server.Close()

	deps, logs := testDeps(t)
	tempDir := t.TempDir()
	section := []byte(fmt.Sprintf(`{"url":%q,"token":"token","storage_id":"/mnt:/","strm_save_dir":%q}`, server.URL, tempDir))
	plugin := NewAlistStrmGen(section, deps)
	if !plugin.Active() {
		t.Fatalf("not active: %v", *logs)
	}
	task := pluginTask(t, "/tv", `{"alist_strm_gen":{"auto_gen":true}}`)
	if err := plugin.Run(task, nil); err != nil {
		t.Fatal(err)
	}
	video := filepath.Join(tempDir, "mnt/tv/video.strm")
	content, err := os.ReadFile(video)
	if err != nil {
		t.Fatalf("video strm: %v (logs %v)", err, *logs)
	}
	if want := server.URL + "/d/mnt/tv/video.mkv?sign=abc"; string(content) != want {
		t.Errorf("video strm = %q, want %q", content, want)
	}
	inside, err := os.ReadFile(filepath.Join(tempDir, "mnt/tv/sub/inside.strm"))
	if err != nil {
		t.Fatalf("inside strm: %v", err)
	}
	if want := server.URL + "/d/mnt/tv/sub/inside.mp4"; string(inside) != want {
		t.Errorf("inside strm = %q, want %q", inside, want)
	}
	if _, err := os.Stat(filepath.Join(tempDir, "mnt/tv/note.strm")); !os.IsNotExist(err) {
		t.Errorf("non-video produced a strm file")
	}

	// auto_gen=false must not touch the filesystem.
	offDir := t.TempDir()
	offSection := []byte(fmt.Sprintf(`{"url":%q,"token":"token","storage_id":"/mnt:/","strm_save_dir":%q}`, server.URL, offDir))
	off := NewAlistStrmGen(offSection, deps)
	if err := off.Run(pluginTask(t, "/tv", `{"alist_strm_gen":{"auto_gen":false}}`), nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(offDir, "mnt/tv/video.strm")); !os.IsNotExist(err) {
		t.Fatalf("auto_gen=false still wrote files")
	}
}

func TestAlistSyncCopiesMissingFiles(t *testing.T) {
	listCalls := 0
	var copyPayload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/me":
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": map[string]any{"username": "admin"}})
		case "/api/admin/storage/get":
			id := r.URL.Query().Get("id")
			data := map[string]any{"driver": "Quark", "mount_path": "/q", "addition": `{"root_folder_id":"0"}`}
			if id == "2" {
				data = map[string]any{"driver": "Local", "mount_path": "/target", "addition": "{}"}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "data": data})
		case "/api/fs/list":
			var body struct {
				Path  string `json:"path"`
				Force bool   `json:"force_root"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			listCalls++
			content := []alistFile{}
			if body.Path == "/q/tv" {
				content = []alistFile{{Name: "Show.S01E01.mkv"}, {Name: "Show.S01E02.mkv"}}
			} else if body.Path == "/target/tv" {
				content = []alistFile{{Name: "Show.S01E01.mkv"}}
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":    200,
				"message": "success",
				"data":    map[string]any{"content": content},
			})
		case "/api/fs/copy":
			_ = json.NewDecoder(r.Body).Decode(&copyPayload)
			_ = json.NewEncoder(w).Encode(map[string]any{"code": 200, "message": "success"})
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	deps, logs := testDeps(t)
	section := []byte(fmt.Sprintf(`{"url":%q,"token":"token","quark_storage_id":"1","save_storage_id":"2"}`, server.URL))
	plugin := NewAlistSync(section, deps)
	if !plugin.Active() {
		t.Fatalf("not active: %v", *logs)
	}
	task := pluginTask(t, "/tv", `{"alist_sync":{"enable":true}}`)
	if err := plugin.Run(task, nil); err != nil {
		t.Fatal(err)
	}
	names, _ := copyPayload["names"].([]any)
	if len(names) != 1 || names[0] != "Show.S01E02.mkv" {
		t.Fatalf("copy payload = %#v", copyPayload)
	}
	if copyPayload["src_dir"] != "/q/tv" || copyPayload["dst_dir"] != "/target/tv" {
		t.Fatalf("copy dirs = %v -> %v", copyPayload["src_dir"], copyPayload["dst_dir"])
	}
	if listCalls < 3 {
		t.Fatalf("expected list calls, got %d", listCalls)
	}
	if !hasLog(*logs, "同步的文件列表") {
		t.Fatalf("logs = %v", *logs)
	}
}
