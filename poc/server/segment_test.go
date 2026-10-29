package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"qas-poc/quark"
)

// rangeServer serves payload with byte-range support. When delay is set it
// trickles 64KB chunks so a pause can land mid-transfer.
func rangeServer(t *testing.T, payload []byte, delay time.Duration) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, end := int64(0), int64(len(payload)-1)
		rangeHeader := r.Header.Get("Range")
		if strings.HasPrefix(rangeHeader, "bytes=") {
			parts := strings.SplitN(strings.TrimPrefix(rangeHeader, "bytes="), "-", 2)
			if value, err := strconv.ParseInt(parts[0], 10, 64); err == nil {
				start = value
			}
			if len(parts) == 2 && parts[1] != "" {
				if value, err := strconv.ParseInt(parts[1], 10, 64); err == nil {
					end = value
				}
			}
			if start >= int64(len(payload)) {
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				return
			}
			if end >= int64(len(payload)) {
				end = int64(len(payload) - 1)
			}
			w.Header().Set("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, len(payload)))
			w.WriteHeader(http.StatusPartialContent)
		}
		body := payload[start : end+1]
		if delay <= 0 {
			_, _ = w.Write(body)
			return
		}
		for len(body) > 0 {
			chunk := 64 * 1024
			if len(body) < chunk {
				chunk = len(body)
			}
			if _, err := w.Write(body[:chunk]); err != nil {
				return
			}
			w.(http.Flusher).Flush()
			body = body[chunk:]
			time.Sleep(delay)
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func downloadClient(t *testing.T, url string) *quark.Client {
	t.Helper()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 200,
			"code":   0,
			"data":   []map[string]string{{"download_url": url}},
		})
	}))
	t.Cleanup(api.Close)
	client := quark.New("")
	client.SetBaseURL(api.URL, api.URL)
	return client
}

func TestSegmentPlan(t *testing.T) {
	if plan := segmentPlan(32 << 20); plan != nil {
		t.Fatalf("small files must not be segmented: %+v", plan)
	}
	for _, size := range []int64{64 << 20, 512 << 20, 2 << 30} {
		plan := segmentPlan(size)
		if len(plan) == 0 {
			t.Fatalf("size %d got no plan", size)
		}
		var expect int64
		for _, segment := range plan {
			if segment.Start != expect {
				t.Fatalf("size %d plan not contiguous: %+v", size, plan)
			}
			expect = segment.End + 1
		}
		if expect != size {
			t.Fatalf("size %d plan covers %d", size, expect)
		}
	}
	if plan := segmentPlan(2 << 30); len(plan) != segmentCountLarge {
		t.Fatalf("large plan = %d segments", len(plan))
	}
}

func TestFetchSegmentedResumesSidecar(t *testing.T) {
	payload := make([]byte, 1<<20)
	for index := range payload {
		payload[index] = byte(index % 251)
	}
	server := rangeServer(t, payload, 0)

	target := filepath.Join(t.TempDir(), "file.bin")
	size := int64(len(payload))
	// A pre-seeded sidecar doubles as the plan, so the 64MB threshold does
	// not matter here.
	state := segmentState{Size: size, Segments: []segmentProgress{
		{Start: 0, End: size/4 - 1},
		{Start: size / 4, End: size/2 - 1},
		{Start: size / 2, End: size*3/4 - 1},
		{Start: size * 3 / 4, End: size - 1},
	}}
	encoded, _ := json.Marshal(state)
	if err := os.WriteFile(target+".part.json", encoded, 0o644); err != nil {
		t.Fatal(err)
	}

	var progress atomic.Int64
	queue := newDownloadQueue(nil)
	if err := queue.fetchSegmented(context.Background(), server.URL, "", "", target, size, func(delta int64) {
		progress.Add(delta)
	}); err != nil {
		t.Fatalf("fetchSegmented: %v", err)
	}
	content, err := os.ReadFile(target)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, payload) {
		t.Fatal("content mismatch")
	}
	if progress.Load() != size {
		t.Fatalf("progress = %d, want %d", progress.Load(), size)
	}
	if _, err := os.Stat(target + ".part.json"); !os.IsNotExist(err) {
		t.Fatal("sidecar should be removed on success")
	}
}

func TestTransferPauseThenResume(t *testing.T) {
	payload := make([]byte, 512*1024)
	for index := range payload {
		payload[index] = byte(index % 253)
	}
	server := rangeServer(t, payload, 20*time.Millisecond)
	client := downloadClient(t, server.URL)

	root := t.TempDir()
	job := &downloadJob{TaskName: "暂停测试", Root: root, Overwrite: true, Client: client}
	queue := newDownloadQueue(nil)
	item := downloadItem{RelPath: "a.bin", FID: "f1", Size: int64(len(payload))}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan transferResult, 1)
	go func() { result <- queue.transfer(job, ctx, item) }()
	time.Sleep(120 * time.Millisecond)
	job.paused.Store(true)
	cancel()
	select {
	case got := <-result:
		if got != transferInterrupted {
			t.Fatalf("paused transfer = %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("pause did not interrupt the transfer")
	}
	if _, err := os.Stat(filepath.Join(root, "a.bin")); !os.IsNotExist(err) {
		t.Fatal("incomplete download must not become the final file")
	}

	job.paused.Store(false)
	if got := queue.transfer(job, context.Background(), item); got != transferDone {
		t.Fatalf("resumed transfer = %v", got)
	}
	content, err := os.ReadFile(filepath.Join(root, "a.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(content, payload) {
		t.Fatal("resumed content mismatch")
	}
}

func TestTransferCancelStopsDownload(t *testing.T) {
	payload := make([]byte, 512*1024)
	server := rangeServer(t, payload, 20*time.Millisecond)
	client := downloadClient(t, server.URL)

	root := t.TempDir()
	job := &downloadJob{TaskName: "取消测试", Root: root, Overwrite: true, Client: client}
	queue := newDownloadQueue(nil)
	item := downloadItem{RelPath: "b.bin", FID: "f1", Size: int64(len(payload))}

	ctx, cancel := context.WithCancel(context.Background())
	result := make(chan transferResult, 1)
	go func() { result <- queue.transfer(job, ctx, item) }()
	time.Sleep(100 * time.Millisecond)
	job.canceled.Store(true)
	cancel()
	select {
	case got := <-result:
		if got != transferInterrupted {
			t.Fatalf("canceled transfer = %v", got)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancel did not interrupt the transfer")
	}
	if _, err := os.Stat(filepath.Join(root, "b.bin")); !os.IsNotExist(err) {
		t.Fatal("canceled download must not produce a final file")
	}
}

func TestPauseResumeCancelStates(t *testing.T) {
	job := &downloadJob{Root: t.TempDir()}
	queue := newDownloadQueue(nil)
	queue.jobs["dl-test"] = job
	job.status = "running"
	job.pendingItems = []downloadItem{{RelPath: "x"}}

	if !queue.Pause("dl-test") || !job.isPaused() || job.currentStatus() != "paused" {
		t.Fatal("pause did not apply")
	}
	if queue.Pause("dl-test") {
		t.Fatal("pausing twice should be rejected")
	}
	if !queue.Resume("dl-test") || job.isPaused() {
		t.Fatal("resume did not apply")
	}
	if !queue.Cancel("dl-test") || !job.isCanceled() {
		t.Fatal("cancel did not apply")
	}
	if queue.Cancel("dl-test") {
		t.Fatal("canceling twice should be rejected")
	}
}

func TestSeedSegmentsFromExistingPart(t *testing.T) {
	segments := []segmentProgress{
		{Start: 0, End: 99},
		{Start: 100, End: 199},
		{Start: 200, End: 299},
	}
	seedSegments(segments, 150)
	if segments[0].Done != 100 || segments[1].Done != 50 || segments[2].Done != 0 {
		t.Fatalf("seeded = %+v", segments)
	}
	seedSegments(segments, 0)
	if segments[0].Done != 0 || segments[1].Done != 0 || segments[2].Done != 0 {
		t.Fatalf("zero seed = %+v", segments)
	}
}
