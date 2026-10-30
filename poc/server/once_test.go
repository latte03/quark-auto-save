package server

import (
	"bytes"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"qas-poc/quark"
)

func TestOnceTaskRoundTrip(t *testing.T) {
	task := mustTask(t, `{"taskname":"a","mode":"once","runtime":["08:00"]}`)
	if !task.IsOnce() || task.OnceFinished() {
		t.Fatalf("mode = %q done = %q", task.Mode, task.OnceDoneAt)
	}
	task.OnceDoneAt = "2026-10-29T15:00:00+08:00"
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Mode       string `json:"mode"`
		OnceDoneAt string `json:"once_done_at"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if out.Mode != "once" || out.OnceDoneAt != task.OnceDoneAt {
		t.Fatalf("round trip = %+v", out)
	}

	// Switching back to the scheduled mode drops both keys.
	task.Mode = ""
	task.OnceDoneAt = ""
	raw, err = json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	if _, ok := fields["mode"]; ok {
		t.Fatal("empty mode must not be written")
	}
	if _, ok := fields["once_done_at"]; ok {
		t.Fatal("empty once_done_at must not be written")
	}
}

func TestOnceTaskSkipsSchedules(t *testing.T) {
	monday := time.Date(2026, 10, 26, 8, 0, 0, 0, time.Local)
	once := mustTask(t, `{"taskname":"once","mode":"once","runtime":["08:00"]}`)
	scheduled := mustTask(t, `{"taskname":"daily","runtime":["08:00"]}`)

	got := dueTaskIndices([]*quark.Task{once, scheduled}, "0 8,20 * * *", monday)
	if !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("due indices = %v", got)
	}
	got = legacyTaskIndices([]*quark.Task{once}, "*/5 * * * *", monday)
	if len(got) != 0 {
		t.Fatalf("legacy indices = %v", got)
	}
	if times := nextTimesForTask(once, "", monday, 3); len(times) != 0 {
		t.Fatalf("next times = %v", times)
	}
	if label := nextRunLabel(once, "", monday); label != "—" {
		t.Fatalf("next label = %q", label)
	}
	if status := statusOf(once, monday); status != "once" {
		t.Fatalf("status = %q", status)
	}

	done := mustTask(t, `{"taskname":"once","mode":"once","once_done_at":"2026-10-26T07:00:00+08:00"}`)
	if status := statusOf(done, monday); status != "once_done" {
		t.Fatalf("done status = %q", status)
	}

	banned := mustTask(t, `{"taskname":"broken","mode":"once","shareurl_ban":"分享失效"}`)
	if status := statusOf(banned, monday); status != "banned" {
		t.Fatalf("banned status = %q", status)
	}
}

func TestOnceTaskAPI(t *testing.T) {
	instance, ts := newTestServer(t)
	client := loginClient(t, ts.URL)

	task := map[string]any{
		"taskname": "一次性",
		"shareurl": "https://pan.quark.cn/s/abcdef",
		"savepath": "video/once",
		"runweek":  []int{1, 2, 3, 4, 5, 6, 7},
		"runtime":  []string{"08:00"},
		"mode":     "once",
		"addition": map[string]any{},
	}
	send := func(method, path string, payload any) *http.Response {
		body, _ := json.Marshal(map[string]any{"task": payload})
		request, _ := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
		request.Header.Set("Content-Type", "application/json")
		response, err := client.Do(request)
		if err != nil {
			t.Fatal(err)
		}
		return response
	}
	read := func() map[string]any {
		response, err := client.Get(ts.URL + "/api/tasks")
		if err != nil {
			t.Fatal(err)
		}
		defer response.Body.Close()
		var payload struct {
			Data struct {
				Tasks []map[string]any `json:"tasks"`
			} `json:"data"`
		}
		if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		if len(payload.Data.Tasks) != 1 {
			t.Fatalf("tasks = %v", payload.Data.Tasks)
		}
		return payload.Data.Tasks[0]
	}

	created := send(http.MethodPost, "/api/tasks", task)
	if created.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(created.Body)
		created.Body.Close()
		t.Fatalf("create = %d: %s", created.StatusCode, body)
	}
	created.Body.Close()
	view := read()
	if view["status"] != "once" || view["next_run"] != "—" {
		t.Fatalf("view = %v / %v", view["status"], view["next_run"])
	}

	if err := instance.store.ApplyRunResults(nil, nil, map[int]bool{0: true}); err != nil {
		t.Fatal(err)
	}
	done := read()
	if done["status"] != "once_done" || done["once_done_at"] == "" {
		t.Fatalf("done view = %v / %v", done["status"], done["once_done_at"])
	}

	// Saving the unchanged task keeps the completion mark.
	unchanged := send(http.MethodPut, "/api/tasks/0", done)
	if unchanged.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(unchanged.Body)
		unchanged.Body.Close()
		t.Fatalf("update = %d: %s", unchanged.StatusCode, body)
	}
	unchanged.Body.Close()
	if view := read(); view["once_done_at"] == "" {
		t.Fatal("unchanged save cleared the completion mark")
	}

	// Editing it turns the task back into a pending one-shot.
	edited := map[string]any{}
	for key, value := range done {
		edited[key] = value
	}
	edited["taskname"] = "一次性（重跑）"
	response := send(http.MethodPut, "/api/tasks/0", edited)
	response.Body.Close()
	if code := response.StatusCode; code != http.StatusOK {
		t.Fatalf("edit = %d", code)
	}
	if view := read(); view["once_done_at"] != nil || view["status"] != "once" {
		t.Fatalf("edited view = %v / %v", view["status"], view["once_done_at"])
	}
}

func TestApplyRunResultsMarksOnceDone(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := `{"tasklist":[
		{"taskname":"once","mode":"once"},
		{"taskname":"daily"},
		{"taskname":"finished","mode":"once","once_done_at":"2026-10-28T09:00:00+08:00"}
	]}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ApplyRunResults(nil, nil, map[int]bool{0: true, 1: true, 2: true}); err != nil {
		t.Fatal(err)
	}
	tasks := store.Tasks()
	if tasks[0].OnceDoneAt == "" {
		t.Fatal("once task was not marked done")
	}
	if tasks[1].OnceDoneAt != "" {
		t.Fatal("scheduled task must not be marked done")
	}
	if tasks[2].OnceDoneAt != "2026-10-28T09:00:00+08:00" {
		t.Fatalf("completed task was rewritten: %q", tasks[2].OnceDoneAt)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc struct {
		TaskList []struct {
			Mode       string `json:"mode"`
			OnceDoneAt string `json:"once_done_at"`
		} `json:"tasklist"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	if doc.TaskList[0].Mode != "once" || doc.TaskList[0].OnceDoneAt == "" {
		t.Fatalf("persisted task = %+v", doc.TaskList[0])
	}
}

func TestRunnableTaskIgnoresScheduleForOnce(t *testing.T) {
	thursday := time.Date(2026, 10, 29, 12, 0, 0, 0, time.Local)
	once := mustTask(t, `{"taskname":"once","mode":"once","runweek":[1],"enddate":"2020-01-01"}`)
	if ok, err := runnableTask(once, thursday); err != nil || !ok {
		t.Fatalf("once runnable = %v, %v", ok, err)
	}
	scheduled := mustTask(t, `{"taskname":"daily","runweek":[1],"enddate":"2020-01-01"}`)
	if ok, err := runnableTask(scheduled, thursday); err != nil || ok {
		t.Fatalf("scheduled runnable = %v, %v", ok, err)
	}
}

func TestMarkOnceDoneByName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := `{"tasklist":[
		{"taskname":"once","mode":"once"},
		{"taskname":"daily"},
		{"taskname":"done","mode":"once","once_done_at":"2026-10-28T09:00:00+08:00"}
	]}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOnceDone("missing"); err != nil {
		t.Fatal(err)
	}
	if err := store.MarkOnceDone("daily"); err != nil {
		t.Fatal(err)
	}
	if tasks := store.Tasks(); tasks[1].OnceDoneAt != "" {
		t.Fatalf("scheduled task was marked done: %q", tasks[1].OnceDoneAt)
	}

	before := store.Tasks()[2].OnceDoneAt
	if err := store.MarkOnceDone("done"); err != nil {
		t.Fatal(err)
	}
	if after := store.Tasks()[2].OnceDoneAt; after != before {
		t.Fatalf("completed task was rewritten: %q", after)
	}

	if err := store.MarkOnceDone("once"); err != nil {
		t.Fatal(err)
	}
	tasks := store.Tasks()
	if tasks[0].OnceDoneAt == "" {
		t.Fatal("one-shot task was not marked done")
	}
}

func TestOnceDownloadDoneRequiresSuccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	config := `{"tasklist":[{"taskname":"once","mode":"once"}]}`
	if err := os.WriteFile(path, []byte(config), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _, err := OpenStore(path)
	if err != nil {
		t.Fatal(err)
	}
	server := &Server{store: store, logger: log.New(io.Discard, "", 0)}
	done := server.onceDoneCallback(store.Tasks()[0])

	for _, status := range []string{"failed", "canceled", "paused"} {
		done(status)
		if store.Tasks()[0].OnceDoneAt != "" {
			t.Fatalf("%s must keep the one-shot task runnable", status)
		}
	}
	done("done")
	if store.Tasks()[0].OnceDoneAt == "" {
		t.Fatal("a finished download should complete the one-shot task")
	}
}
