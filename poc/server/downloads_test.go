package server

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

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
	size, err := queue.fetchFile(server.URL, "", target)
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
	if _, err := queue.fetchFile(server.URL, "", target); err != nil {
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
	queue.transfer(job, downloadItem{RelPath: "剧集/E01.mkv", FID: "fid-1"})

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
	queue.transfer(job, downloadItem{RelPath: "剧集/E01.mkv", FID: "fid-1"})
	if skipped := job.view().Skipped; skipped != 1 {
		t.Fatalf("skipped = %d, want 1", skipped)
	}

	// Path traversal is rejected.
	job.Overwrite = true
	queue.transfer(job, downloadItem{RelPath: "../escape.mkv", FID: "fid-1"})
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
