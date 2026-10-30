package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"qas-poc/quark"
)

func TestSafeRelPath(t *testing.T) {
	if got, ok := safeRelPath("/剧集/S01E01.mkv"); !ok || got != filepath.Join("剧集", "S01E01.mkv") {
		t.Fatalf("safeRelPath = %q, %v", got, ok)
	}
	for _, bad := range []string{"", "..", "a/../../b", "../x", "//"} {
		if _, ok := safeRelPath(bad); ok {
			t.Fatalf("safeRelPath(%q) should fail", bad)
		}
	}
}

func TestFetchFileResumes(t *testing.T) {
	payload := "hello world"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rangeHeader := r.Header.Get("Range")
		if strings.HasPrefix(rangeHeader, "bytes=") {
			start, _ := strconv.Atoi(strings.TrimSuffix(strings.TrimPrefix(rangeHeader, "bytes="), "-"))
			w.WriteHeader(http.StatusPartialContent)
			_, _ = io.WriteString(w, payload[start:])
			return
		}
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "episode.mkv")
	if err := os.WriteFile(target+".part", []byte(payload[:5]), 0o644); err != nil {
		t.Fatal(err)
	}

	queue := newDownloadQueue(nil)
	size, err := queue.fetchFile(context.Background(), server.URL, "", "", target, nil)
	if err != nil {
		t.Fatalf("fetchFile: %v", err)
	}
	if size != int64(len(payload)) {
		t.Fatalf("size = %d, want %d", size, len(payload))
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != payload {
		t.Fatalf("content = %q, want %q", content, payload)
	}
	if _, err := os.Stat(target + ".part"); !os.IsNotExist(err) {
		t.Fatal("part file should be renamed away")
	}
}

func TestFetchFileFallsBackWhenRangeIgnored(t *testing.T) {
	payload := "abcdefgh"
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
	defer server.Close()

	dir := t.TempDir()
	target := filepath.Join(dir, "file.bin")
	if err := os.WriteFile(target+".part", []byte("abc"), 0o644); err != nil {
		t.Fatal(err)
	}
	queue := newDownloadQueue(nil)
	if _, err := queue.fetchFile(context.Background(), server.URL, "", "", target, nil); err != nil {
		t.Fatalf("fetchFile: %v", err)
	}
	content, _ := os.ReadFile(target)
	if string(content) != payload {
		t.Fatalf("content = %q, want %q", content, payload)
	}
}

func TestCollectDownloadItems(t *testing.T) {
	tree := quark.NewTree()
	root := tree.Root()
	tree.Add(root, "S01E01.mkv", "fid-1", quark.NodeData{Path: "/影视/黑镜/S01E01.mkv", FileNameRe: "S01E01.mkv", FID: "fid-1"})
	tree.Add(root, "📁SP", "fid-dir", quark.NodeData{IsDir: true, Path: "/影视/黑镜/SP"})
	tree.Add(root.Children[1], "S00E01.mkv", "fid-2", quark.NodeData{Path: "/影视/黑镜/SP/S00E01.mkv", FileNameRe: "S00E01.mkv", FID: "fid-2"})
	tree.Add(root, "planned.mkv", "fid-3", quark.NodeData{Path: "/影视/黑镜/planned.mkv", FileNameRe: "planned.mkv", Planned: true})

	task := &quark.Task{SavePath: "/影视/黑镜"}
	items := collectDownloadItems(tree, task)
	if len(items) != 2 {
		t.Fatalf("items = %d, want 2: %+v", len(items), items)
	}
	if items[0].RelPath != "S01E01.mkv" {
		t.Fatalf("first rel = %q", items[0].RelPath)
	}
	if items[1].RelPath != filepath.Join("SP", "S00E01.mkv") {
		t.Fatalf("second rel = %q", items[1].RelPath)
	}
}

func TestQueueDownloadsFiles(t *testing.T) {
	payload := "0123456789"
	fileServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, payload)
	}))
	defer fileServer.Close()

	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"code":0,"status":200,"data":[{"download_url":"`+fileServer.URL+`/f"}]}`)
	}))
	defer api.Close()

	client := quark.New("")
	client.SetBaseURL(api.URL, api.URL)

	dir := t.TempDir()
	queue := newDownloadQueue(nil)
	job := &downloadJob{TaskName: "测试任务", Root: dir, Client: client}
	queue.transfer(job, context.Background(), downloadItem{RelPath: "剧集/E01.mkv", FID: "fid-1", Size: int64(len(payload))})

	view := job.view()
	if view.Done != 1 || view.Failed != 0 {
		t.Fatalf("unexpected job result: %+v", view)
	}
	content, err := os.ReadFile(filepath.Join(dir, "剧集", "E01.mkv"))
	if err != nil {
		t.Fatal(err)
	}
	if string(content) != payload {
		t.Fatalf("content = %q, want %q", content, payload)
	}

	// Existing files are skipped unless overwrite is on.
	job.Overwrite = false
	queue.transfer(job, context.Background(), downloadItem{RelPath: "剧集/E01.mkv", FID: "fid-1", Size: int64(len(payload))})
	if skipped := job.view().Skipped; skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}

	// Path traversal is rejected.
	job.Overwrite = true
	queue.transfer(job, context.Background(), downloadItem{RelPath: "../escape.mkv", FID: "fid-1"})
	if failed := job.view().Failed; failed != 1 {
		t.Fatalf("failed = %d, want 1 for traversal path", failed)
	}
}

func TestEnsureFreeSpace(t *testing.T) {
	if message := ensureFreeSpace(t.TempDir(), 0); message != "" {
		t.Fatalf("zero threshold should pass: %s", message)
	}
	if message := ensureFreeSpace(t.TempDir(), 1000000); message == "" {
		t.Fatal("huge threshold should fail")
	}
}

func TestCollectMissingDownloads(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/1/clouddrive/file/info/path_list":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code": 0,
				"data": []map[string]string{{"file_path": "/tv", "fid": "dir-tv"}},
			})
		case "/1/clouddrive/file/sort":
			lists := map[string][]map[string]any{
				"dir-tv": {
					{"fid": "f1", "file_name": "S01.mkv"},
					{"fid": "sub", "file_name": "剧集", "dir": true},
				},
				"sub": {
					{"fid": "f2", "file_name": "S02.mkv"},
				},
			}
			list := lists[r.URL.Query().Get("pdir_fid")]
			_ = json.NewEncoder(w).Encode(map[string]any{
				"code":     0,
				"metadata": map[string]any{"_total": len(list)},
				"data":     map[string]any{"list": list},
			})
		default:
			t.Errorf("unexpected upstream path %s", r.URL.Path)
		}
	}))
	defer upstream.Close()

	client := quark.New("")
	client.SetBaseURL(upstream.URL, upstream.URL)

	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "S01.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := &Server{}
	logf := func(string, ...any) {}
	task := &quark.Task{TaskName: "测试", SavePath: "/tv"}

	items := server.collectMissingDownloads(client, task, root, false, nil, logf)
	if len(items) != 1 || items[0].RelPath != "剧集/S02.mkv" || items[0].FID != "f2" {
		t.Fatalf("items = %#v", items)
	}

	// Files already queued by this run's transfer are not repeated.
	if got := server.collectMissingDownloads(client, task, root, false, map[string]bool{"f2": true}, logf); len(got) != 0 {
		t.Fatalf("skip set ignored: %#v", got)
	}

	// Overwrite re-downloads files that already exist locally.
	all := server.collectMissingDownloads(client, task, root, true, nil, logf)
	if len(all) != 2 || all[0].RelPath != "S01.mkv" || all[1].RelPath != "剧集/S02.mkv" {
		t.Fatalf("overwrite items = %#v", all)
	}
}

func TestLocalFileExists(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "sub", "a.mkv"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !localFileExists(root, "sub/a.mkv") {
		t.Fatal("existing file not detected")
	}
	if localFileExists(root, "sub") {
		t.Fatal("directories must not count as files")
	}
	if localFileExists(root, "missing.mkv") {
		t.Fatal("missing file reported as present")
	}
}

func TestDownloadJobTracksProgress(t *testing.T) {
	job := &downloadJob{Root: t.TempDir()}
	job.addProgress(120)
	if view := job.view(); view.Bytes != 120 {
		t.Fatalf("in-flight bytes = %d", view.Bytes)
	}
	job.addDone(500, 120)
	view := job.view()
	if view.Bytes != 500 || view.Done != 1 {
		t.Fatalf("view = %+v", view)
	}
	// A failed transfer removes its partial bytes.
	job.addProgress(80)
	job.dropProgress(80)
	if view := job.view(); view.Bytes != 500 {
		t.Fatalf("bytes after drop = %d", view.Bytes)
	}
}

func TestClearFinishedKeepsActiveJobs(t *testing.T) {
	queue := &downloadQueue{jobs: map[string]*downloadJob{}}
	add := func(id, status string) {
		job := &downloadJob{ID: id, status: status}
		queue.jobs[id] = job
		queue.order = append(queue.order, id)
	}
	add("done", "done")
	add("running", "running")
	add("failed", "failed")
	add("paused", "paused")

	if removed := queue.ClearFinished(); removed != 2 {
		t.Fatalf("removed = %d, want 2", removed)
	}
	views := queue.Snapshot()
	if len(views) != 2 || views[0].ID != "paused" || views[1].ID != "running" {
		t.Fatalf("remaining jobs = %+v", views)
	}
}

func TestDownloadJobFinishCallback(t *testing.T) {
	queue := newDownloadQueue(nil)
	done := make(chan string, 1)
	job := &downloadJob{
		TaskName: "测试",
		Root:     t.TempDir(), // not under /vol, so the job fails fast
		OnFinish: func(status string) { done <- status },
	}
	queue.Enqueue(job)

	select {
	case status := <-done:
		if status != "failed" {
			t.Fatalf("status = %q", status)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("OnFinish was not called")
	}
}

func TestSplitCloudPath(t *testing.T) {
	if parent, name := splitCloudPath("foo"); parent != "/" || name != "foo" {
		t.Fatalf("bare name = %q, %q", parent, name)
	}
	if parent, name := splitCloudPath("/tv/Show"); parent != "/tv" || name != "Show" {
		t.Fatalf("nested = %q, %q", parent, name)
	}
	if parent, name := splitCloudPath("/tv"); parent != "/" || name != "tv" {
		t.Fatalf("top level = %q, %q", parent, name)
	}
}
