package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

type fakeAPI struct {
	mu        sync.Mutex
	runBodies []map[string]any
	deletes   []map[string]any
	taskPosts int
	patches   []map[string]any
}

func (f *fakeAPI) handler(t *testing.T) http.HandlerFunc {
	t.Helper()
	write := func(w http.ResponseWriter, data any) {
		_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "data": data})
	}
	return func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/api/status":
			write(w, map[string]any{"task_count": 2, "banned_count": 0, "crontab": "0 8 * * *"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/tasks":
			write(w, map[string]any{"tasks": []map[string]any{
				{"index": 0, "taskname": "剧A", "shareurl": "https://pan.quark.cn/s/aaa", "savepath": "/tv/剧A"},
				{"index": 1, "taskname": "剧B", "shareurl": "https://pan.quark.cn/s/bbb", "savepath": "/tv/剧B"},
			}})
		case r.Method == http.MethodGet && r.URL.Path == "/api/config":
			write(w, map[string]any{
				"cookie":    []string{"__pus=SECRET"},
				"api_token": "SECRETTOKEN",
				"crontab":   "0 8 * * *",
				"webui":     map[string]any{"username": "admin", "password": "pw"},
				"plugins": map[string]any{
					"alist": map[string]any{"url": "http://alist.local", "token": "alist-secret", "password": "pw2"},
				},
				"push_config": map[string]any{"CONSOLE": true, "TG_BOT_TOKEN": "tg-secret"},
				"magic_regex": map[string]any{"TV": map[string]any{"pattern": "x", "replace": "y"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/api/run":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.runBodies = append(f.runBodies, body)
			f.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "run_id": "run-1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/sign":
			_ = json.NewEncoder(w).Encode(map[string]any{"success": true, "run_id": "sign-1"})
		case r.Method == http.MethodGet && r.URL.Path == "/api/runs/run-1":
			write(w, map[string]any{"id": "run-1", "done": true, "saved_total": 3})
		case r.Method == http.MethodGet && r.URL.Path == "/api/runs/run-1/log":
			write(w, map[string]any{"run": map[string]any{"id": "run-1", "done": true}, "lines": []string{"第一行", "第二行"}, "done": true})
		case r.Method == http.MethodPost && r.URL.Path == "/api/files/delete":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.deletes = append(f.deletes, body)
			f.mu.Unlock()
			write(w, map[string]any{"fid": "f1", "task_id": "t1"})
		case r.Method == http.MethodPost && r.URL.Path == "/api/tasks":
			f.mu.Lock()
			f.taskPosts++
			f.mu.Unlock()
			write(w, map[string]any{"tasks": []any{}})
		case r.Method == http.MethodPut && r.URL.Path == "/api/config":
			var body map[string]any
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			f.patches = append(f.patches, body)
			f.mu.Unlock()
			write(w, map[string]any{"cookie": []string{"__pus=SECRET"}, "api_token": "SECRETTOKEN", "crontab": body["crontab"]})
		default:
			t.Errorf("unexpected API call %s %s", r.Method, r.URL.Path)
			http.NotFound(w, r)
		}
	}
}

func newTestServer(t *testing.T, readOnly bool) (*Server, *fakeAPI, *httptest.Server) {
	t.Helper()
	fake := &fakeAPI{}
	ts := httptest.NewServer(fake.handler(t))
	t.Cleanup(ts.Close)
	server := New(NewClient(ts.URL, "test-token"), Options{Version: "test", ReadOnly: readOnly})
	return server, fake, ts
}

func callTool(t *testing.T, server *Server, name string, args map[string]any) toolResult {
	t.Helper()
	params, err := json.Marshal(map[string]any{"name": name, "arguments": args})
	if err != nil {
		t.Fatal(err)
	}
	response := server.Handle(context.Background(), rpcRequest{
		ID: json.RawMessage(`1`), Method: "tools/call", Params: params,
	})
	if response.Error != nil {
		t.Fatalf("rpc error = %+v", response.Error)
	}
	result, ok := response.Result.(toolResult)
	if !ok {
		t.Fatalf("result type = %T", response.Result)
	}
	return result
}

func resultText(t *testing.T, result toolResult) string {
	t.Helper()
	if len(result.Content) != 1 {
		t.Fatalf("content = %#v", result.Content)
	}
	return result.Content[0].Text
}

func TestInitializeEchoesProtocol(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	response := server.Handle(context.Background(), rpcRequest{
		ID: json.RawMessage(`"init-1"`), Method: "initialize",
		Params: json.RawMessage(`{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"t","version":"1"}}`),
	})
	if response.Error != nil {
		t.Fatal(response.Error)
	}
	result, _ := response.Result.(map[string]any)
	if result["protocolVersion"] != "2024-11-05" {
		t.Fatalf("protocolVersion = %v", result["protocolVersion"])
	}
	info, _ := result["serverInfo"].(map[string]any)
	if info["name"] != "qas" || info["version"] != "test" {
		t.Fatalf("serverInfo = %#v", info)
	}
	if string(response.ID) != `"init-1"` {
		t.Fatalf("id = %s", response.ID)
	}
}

func TestToolsListAndReadOnly(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	response := server.Handle(context.Background(), rpcRequest{ID: json.RawMessage(`1`), Method: "tools/list"})
	result, _ := response.Result.(map[string]any)
	tools, _ := result["tools"].([]map[string]any)
	names := map[string]bool{}
	for _, tool := range tools {
		names[tool["name"].(string)] = true
	}
	for _, want := range []string{"get_status", "list_tasks", "run_tasks", "delete_file", "sign_in", "add_task"} {
		if !names[want] {
			t.Fatalf("missing tool %s", want)
		}
	}

	readOnlyServer, _, _ := newTestServer(t, true)
	response = readOnlyServer.Handle(context.Background(), rpcRequest{ID: json.RawMessage(`2`), Method: "tools/list"})
	result, _ = response.Result.(map[string]any)
	tools, _ = result["tools"].([]map[string]any)
	for _, tool := range tools {
		if name := tool["name"].(string); name == "run_tasks" || name == "delete_file" || name == "add_task" {
			t.Fatalf("read-only list contains %s", name)
		}
	}
	blocked := callTool(t, readOnlyServer, "run_tasks", map[string]any{})
	if !blocked.IsError || !strings.Contains(resultText(t, blocked), "只读") {
		t.Fatalf("blocked = %#v", blocked)
	}
}

func TestGetStatusAndTaskLookup(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	status := resultText(t, callTool(t, server, "get_status", nil))
	if !strings.Contains(status, "task_count") {
		t.Fatalf("status = %s", status)
	}
	task := resultText(t, callTool(t, server, "get_task", map[string]any{"name": "剧B"}))
	if !strings.Contains(task, "剧B") {
		t.Fatalf("task = %s", task)
	}
	ambiguous := callTool(t, server, "get_task", map[string]any{"name": "剧"})
	if !ambiguous.IsError || !strings.Contains(resultText(t, ambiguous), "多个") {
		t.Fatalf("ambiguous = %#v", ambiguous)
	}
}

func TestGetConfigMasksSecrets(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	text := resultText(t, callTool(t, server, "get_config", nil))
	for _, secret := range []string{"__pus=SECRET", "SECRETTOKEN", "alist-secret", "pw2", "tg-secret"} {
		if strings.Contains(text, secret) {
			t.Fatalf("config leaked %q:\n%s", secret, text)
		}
	}
	for _, kept := range []string{"http://alist.local", "0 8 * * *", "CONSOLE"} {
		if !strings.Contains(text, kept) {
			t.Fatalf("config missing %q:\n%s", kept, text)
		}
	}
	if !strings.Contains(text, "***") {
		t.Fatalf("config has no mask:\n%s", text)
	}
	if strings.Contains(text, "cookie") || strings.Contains(text, "api_token") {
		t.Fatalf("config kept cookie/api_token:\n%s", text)
	}
}

func TestRunTasksRequiresConfirmForWrite(t *testing.T) {
	server, fake, _ := newTestServer(t, false)
	blocked := callTool(t, server, "run_tasks", map[string]any{"tasks": []any{0}, "write": true})
	if !blocked.IsError || !strings.Contains(resultText(t, blocked), "confirm") {
		t.Fatalf("blocked = %#v", blocked)
	}
	if len(fake.runBodies) != 0 {
		t.Fatalf("run was started without confirm: %#v", fake.runBodies)
	}

	dry := resultText(t, callTool(t, server, "run_tasks", map[string]any{"tasks": []any{0}}))
	if !strings.Contains(dry, "run-1") {
		t.Fatalf("dry run = %s", dry)
	}
	if fake.runBodies[0]["write"] != false {
		t.Fatalf("dry body = %#v", fake.runBodies[0])
	}

	confirmed := callTool(t, server, "run_tasks", map[string]any{
		"tasks": []any{1}, "write": true, "confirm": true, "wait_seconds": 1,
	})
	if confirmed.IsError {
		t.Fatal(resultText(t, confirmed))
	}
	if fake.runBodies[1]["write"] != true {
		t.Fatalf("confirmed body = %#v", fake.runBodies[1])
	}
	text := resultText(t, confirmed)
	if !strings.Contains(text, "第二行") || !strings.Contains(text, `"timeout": false`) {
		t.Fatalf("wait result = %s", text)
	}
}

func TestInlineTaskRun(t *testing.T) {
	server, fake, _ := newTestServer(t, false)
	result := callTool(t, server, "run_tasks", map[string]any{
		"task": map[string]any{"taskname": "临时", "shareurl": "https://pan.quark.cn/s/x", "savepath": "/x"},
	})
	if result.IsError {
		t.Fatal(resultText(t, result))
	}
	body := fake.runBodies[0]
	if body["write"] != false {
		t.Fatalf("body = %#v", body)
	}
	if _, ok := body["task"].(map[string]any); !ok {
		t.Fatalf("inline task missing: %#v", body)
	}
}

func TestDeleteFileRequiresConfirm(t *testing.T) {
	server, fake, _ := newTestServer(t, false)
	blocked := callTool(t, server, "delete_file", map[string]any{"path": "/a.mp4"})
	if !blocked.IsError {
		t.Fatal("delete without confirm should fail")
	}
	if len(fake.deletes) != 0 {
		t.Fatalf("delete sent without confirm")
	}
	ok := callTool(t, server, "delete_file", map[string]any{"path": "/a.mp4", "confirm": true})
	if ok.IsError {
		t.Fatal(resultText(t, ok))
	}
	if len(fake.deletes) != 1 || fake.deletes[0]["confirm"] != true {
		t.Fatalf("deletes = %#v", fake.deletes)
	}
}

func TestDeleteTaskRequiresConfirm(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	blocked := callTool(t, server, "delete_task", map[string]any{"index": 0})
	if !blocked.IsError {
		t.Fatal("delete task without confirm should fail")
	}
}

func TestSetCrontabSanitizesResult(t *testing.T) {
	server, fake, _ := newTestServer(t, false)
	result := callTool(t, server, "set_crontab", map[string]any{"crontab": "*/5 * * * *"})
	if result.IsError {
		t.Fatal(resultText(t, result))
	}
	if len(fake.patches) != 1 || fake.patches[0]["crontab"] != "*/5 * * * *" {
		t.Fatalf("patches = %#v", fake.patches)
	}
	text := resultText(t, result)
	if strings.Contains(text, "SECRET") {
		t.Fatalf("result leaked secrets: %s", text)
	}
}

func TestUnknownTool(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	params := json.RawMessage(`{"name":"nope","arguments":{}}`)
	response := server.Handle(context.Background(), rpcRequest{ID: json.RawMessage(`1`), Method: "tools/call", Params: params})
	if response.Error == nil || response.Error.Code != -32602 {
		t.Fatalf("response = %#v", response)
	}
}

func TestServeHandlesNewlineMessages(t *testing.T) {
	server, _, _ := newTestServer(t, false)
	input := strings.NewReader(
		`{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18"}}` + "\n" +
			`{"jsonrpc":"2.0","method":"notifications/initialized"}` + "\n" +
			`{"jsonrpc":"2.0","id":2,"method":"tools/list"}` + "\n")
	var out bytes.Buffer
	if err := server.Serve(context.Background(), input, &out); err != nil {
		t.Fatal(err)
	}
	decoder := json.NewDecoder(bytes.NewReader(out.Bytes()))
	responses := map[string]rpcResponse{}
	for {
		var response rpcResponse
		if err := decoder.Decode(&response); err != nil {
			if err == io.EOF {
				break
			}
			t.Fatal(err)
		}
		responses[string(response.ID)] = response
	}
	if len(responses) != 2 {
		t.Fatalf("responses = %#v", responses)
	}
	if responses["1"].Error != nil || responses["2"].Error != nil {
		t.Fatalf("responses = %#v", responses)
	}
}
