package plugins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"qas-poc/quark"
)

func rpcServer(t *testing.T, calls *[]map[string]any) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request map[string]any
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Error(err)
			return
		}
		if calls != nil {
			*calls = append(*calls, request)
		}
		switch request["method"] {
		case "aria2.getVersion":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "quark-auto-save", "result": map[string]any{"version": "1.36.0"}})
		case "aria2.addUri":
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "quark-auto-save", "result": "gid-1"})
		default:
			_ = json.NewEncoder(w).Encode(map[string]any{"jsonrpc": "2.0", "id": "quark-auto-save", "error": map[string]any{"message": "unknown method"}})
		}
	}))
}

func TestAria2AddsDownloadTasks(t *testing.T) {
	var calls []map[string]any
	server := rpcServer(t, &calls)
	defer server.Close()

	deps, logs := testDeps(t)
	section := []byte(fmt.Sprintf(`{"host_port":%q,"secret":"s3cret","dir":"/dl"}`, strings.TrimPrefix(server.URL, "http://")))
	plugin := NewAria2(section, deps)
	if !plugin.Active() {
		t.Fatalf("not active: %v", *logs)
	}

	tree := quark.NewTree()
	tree.SetRoot(&quark.TreeNode{ID: "target", Name: "/tv", Data: quark.NodeData{IsDir: true}})
	tree.Add(tree.Root(), "🎞️Show.E01.mkv", "f1", quark.NodeData{
		FileName: "Show.E01.mkv", FileNameRe: "Show.E01.mkv", FID: "f1", Path: "/tv/Show.E01.mkv", ObjCategory: "video",
	})
	tree.Add(tree.Root(), "📁sub", "d1", quark.NodeData{IsDir: true, FID: "d1", Path: "/tv/sub"})

	account := &fakeAccount{
		dirs:      map[string][]quark.ShareFile{"d1": {{FID: "f2", FileName: "x.mkv"}}},
		downloads: []quark.DownloadItem{{DownloadURL: "http://dl/1"}, {DownloadURL: "http://dl/2"}},
		cookie:    "c=1",
	}
	task := pluginTask(t, "/tv", `{"aria2":{"auto_download":true,"download_subdir":true,"save_path":"tv","pause":true}}`)
	env := &quark.PluginEnv{Account: account, Tree: tree, Log: deps.Log}
	if err := plugin.Run(task, env); err != nil {
		t.Fatal(err)
	}

	var addURI []map[string]any
	for _, call := range calls {
		if call["method"] == "aria2.addUri" {
			addURI = append(addURI, call)
		} else if params, _ := call["params"].([]any); len(params) == 0 || params[0] != "token:s3cret" {
			t.Errorf("getVersion params = %v", call["params"])
		}
	}
	if len(addURI) != 2 {
		t.Fatalf("addUri calls = %d, logs: %v", len(addURI), *logs)
	}
	first := addURI[0]["params"].([]any)
	if first[0] != "token:s3cret" {
		t.Errorf("secret token missing: %v", first)
	}
	uris := first[1].([]any)
	if len(uris) != 1 || uris[0] != "http://dl/1" {
		t.Errorf("uris = %v", uris)
	}
	options := first[2].(map[string]any)
	if options["out"] != "Show.E01.mkv" || options["dir"] != "/dl/tv" || options["pause"] != "true" {
		t.Errorf("options = %v", options)
	}
	headers := options["header"].([]any)
	joined := fmt.Sprint(headers)
	if !strings.Contains(joined, "Cookie: c=1") || !strings.Contains(joined, "User-Agent: test-ua") {
		t.Errorf("headers = %v", headers)
	}
	if len(account.downloaded) != 2 || account.downloaded[1] != "f2" {
		t.Errorf("downloaded fids = %v", account.downloaded)
	}
}

func TestAria2SkipsWhenDisabled(t *testing.T) {
	var calls []map[string]any
	server := rpcServer(t, &calls)
	defer server.Close()
	deps, _ := testDeps(t)
	section := []byte(fmt.Sprintf(`{"host_port":%q,"secret":"s"}`, strings.TrimPrefix(server.URL, "http://")))
	plugin := NewAria2(section, deps)
	env := &quark.PluginEnv{Account: &fakeAccount{}, Tree: quark.NewTree(), Log: deps.Log}
	if err := plugin.Run(pluginTask(t, "/tv", `{"aria2":{"auto_download":false}}`), env); err != nil {
		t.Fatal(err)
	}
	for _, call := range calls {
		if call["method"] == "aria2.addUri" {
			t.Fatalf("addUri called while disabled")
		}
	}
}

func archiveTree() *quark.Tree {
	tree := quark.NewTree()
	tree.SetRoot(&quark.TreeNode{ID: "target", Name: "/tv", Data: quark.NodeData{IsDir: true}})
	tree.Add(tree.Root(), "📦show.zip", "zip1", quark.NodeData{
		FileName: "show.zip", FileNameRe: "show.zip", FID: "zip1", Path: "/tv/show.zip", ObjCategory: "archive",
	})
	return tree
}

func unarchiveAccount() *fakeAccount {
	return &fakeAccount{
		unarchiveTaskID: "task-9",
		unarchiveResult: quark.TaskResult{UnarchiveList: []quark.ShareFile{{FID: "sub1", FileName: "show"}}},
		dirs:            map[string][]quark.ShareFile{"sub1": {{FID: "x1", FileName: "video.mkv"}}},
	}
}

func TestAutoUnarchiveMovesAndCleans(t *testing.T) {
	deps, logs := testDeps(t)
	plugin := NewAutoUnarchive(nil, deps)
	account := unarchiveAccount()
	env := &quark.PluginEnv{Account: account, Tree: archiveTree(), SavepathFID: map[string]string{"/tv": "target"}, Log: deps.Log}
	task := pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true}}`)
	if err := plugin.Run(task, env); err != nil {
		t.Fatal(err)
	}
	if len(account.renames) != 2 || account.renames[0] != (renameCall{"sub1", "show.zip"}) || account.renames[1] != (renameCall{"x1", "show.mkv"}) {
		t.Fatalf("renames = %#v", account.renames)
	}
	if len(account.moves) != 1 || len(account.moves[0].FIDs) != 1 || account.moves[0].FIDs[0] != "x1" || account.moves[0].To != "target" {
		t.Fatalf("moves = %#v", account.moves)
	}
	if len(account.deletes) != 1 || len(account.deletes[0]) != 1 || account.deletes[0][0] != "zip1" {
		t.Fatalf("deletes = %#v", account.deletes)
	}
	if !hasLog(*logs, "批量清理完成") || !hasLog(*logs, "重命名: show.mkv") {
		t.Fatalf("logs = %v", *logs)
	}
}

func TestAutoUnarchiveCleanModes(t *testing.T) {
	deps, _ := testDeps(t)
	plugin := NewAutoUnarchive(nil, deps)

	account := unarchiveAccount()
	env := &quark.PluginEnv{Account: account, Tree: archiveTree(), SavepathFID: map[string]string{"/tv": "target"}, Log: deps.Log}
	task := pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true,"auto_clean":false}}`)
	if err := plugin.Run(task, env); err != nil {
		t.Fatal(err)
	}
	if len(account.deletes) != 1 || len(account.deletes[0]) != 1 || account.deletes[0][0] != "sub1" {
		t.Fatalf("auto_clean=false deletes = %#v", account.deletes)
	}

	account = unarchiveAccount()
	env = &quark.PluginEnv{Account: account, Tree: archiveTree(), SavepathFID: map[string]string{"/tv": "target"}, Log: deps.Log}
	task = pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true,"auto_clean_zipdir":true}}`)
	if err := plugin.Run(task, env); err != nil {
		t.Fatal(err)
	}
	if len(account.renames) != 1 || account.renames[0].FID != "x1" {
		t.Fatalf("zipdir mode renames = %#v", account.renames)
	}
	if len(account.deletes) != 1 || len(account.deletes[0]) != 2 || account.deletes[0][0] != "zip1" || account.deletes[0][1] != "sub1" {
		t.Fatalf("zipdir mode deletes = %#v", account.deletes)
	}
}

func TestAutoUnarchiveTaskGate(t *testing.T) {
	deps, _ := testDeps(t)
	disabled := &fakeAccount{}
	env := &quark.PluginEnv{Account: disabled, Tree: archiveTree(), SavepathFID: map[string]string{"/tv": "target"}, Log: deps.Log}
	if err := NewAutoUnarchive(nil, deps).Run(pluginTask(t, "/tv", `{"auto_unarchive":{"enable":false}}`), env); err != nil {
		t.Fatal(err)
	}
	if len(disabled.renames) != 0 || len(disabled.moves) != 0 {
		t.Fatalf("disabled plugin acted: %#v", disabled)
	}

	global := NewAutoUnarchive([]byte(`{"global_enable":true}`), deps)
	account := unarchiveAccount()
	env = &quark.PluginEnv{Account: account, Tree: archiveTree(), SavepathFID: map[string]string{"/tv": "target"}, Log: deps.Log}
	if err := global.Run(pluginTask(t, "/tv", `{"auto_unarchive":{"enable":false}}`), env); err != nil {
		t.Fatal(err)
	}
	if len(account.moves) != 1 {
		t.Fatalf("global_enable did not run: %#v", account)
	}
}

func TestSmartStrmWebhook(t *testing.T) {
	var posted map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "version": "1.2.3"})
		case http.MethodPost:
			_ = json.NewDecoder(r.Body).Decode(&posted)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"task":    map[string]any{"name": "剧集", "storage_path": "/media/tv"},
			})
		}
	}))
	defer server.Close()

	deps, logs := testDeps(t)
	section := []byte(fmt.Sprintf(`{"webhook":%q,"strmtask":"tv","xlist_path_fix":"/quark:/"}`, server.URL))
	plugin := NewSmartStrm(section, deps)
	if !plugin.Active() {
		t.Fatalf("not active: %v", *logs)
	}
	if err := plugin.Run(pluginTask(t, "/tv", "null"), nil); err != nil {
		t.Fatal(err)
	}
	data, _ := posted["data"].(map[string]any)
	if posted["event"] != "qas_strm" || data["strmtask"] != "tv" || data["savepath"] != "/tv" || data["xlist_path_fix"] != "/quark:/" {
		t.Fatalf("payload = %#v", posted)
	}
	if !hasLog(*logs, "[剧集] /media/tv 成功✅") {
		t.Fatalf("logs = %v", *logs)
	}
}

func TestRegistryOrderAndDefaults(t *testing.T) {
	deps, _ := testDeps(t)
	list := Build(nil, deps)
	var names []string
	active := 0
	for _, plugin := range list {
		names = append(names, plugin.Name())
		if plugin.Active() {
			active++
		}
	}
	want := []string{"auto_unarchive", "smartstrm", "alist", "alist_strm", "alist_strm_gen", "alist_sync", "aria2"}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Fatalf("order = %v", names)
	}
	if active != 1 {
		t.Fatalf("active without config = %d, want only auto_unarchive", active)
	}
	if defaults := list[0].TaskDefaults(); defaults["enable"] != false || defaults["auto_clean"] != true {
		t.Fatalf("auto_unarchive defaults = %v", defaults)
	}
}
