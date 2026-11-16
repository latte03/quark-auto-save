package quark

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func fids(count int) []string {
	out := make([]string, count)
	for index := range out {
		out[index] = fmt.Sprintf("f%d", index)
	}
	return out
}

func moveServer(t *testing.T, batches *[]int, failAt int) *httptest.Server {
	t.Helper()
	calls := 0
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var body struct {
			Filelist []string `json:"filelist"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Error(err)
			return
		}
		*batches = append(*batches, len(body.Filelist))
		w.Header().Set("content-type", "application/json")
		if calls == failAt {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"status": 200, "code": 10002, "message": "操作失败，请稍后重试", "data": map[string]any{},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status": 200, "code": 0, "message": "ok", "data": map[string]any{"task_id": "move-1"},
		})
	}))
}

func TestMoveFilesSplitsIntoServerSizedBatches(t *testing.T) {
	var batches []int
	server := moveServer(t, &batches, 0)
	defer server.Close()
	client := New("__pus=pc-only")
	client.SetBaseURL(server.URL, "http://app.invalid")

	if err := client.MoveFiles(fids(250), "target"); err != nil {
		t.Fatal(err)
	}
	if len(batches) != 3 || batches[0] != 100 || batches[1] != 100 || batches[2] != 50 {
		t.Fatalf("batches = %v, want [100 100 50]", batches)
	}
}

func TestMoveFilesStopsAtFailingBatch(t *testing.T) {
	var batches []int
	server := moveServer(t, &batches, 2)
	defer server.Close()
	client := New("__pus=pc-only")
	client.SetBaseURL(server.URL, "http://app.invalid")

	err := client.MoveFiles(fids(250), "target")
	if err == nil {
		t.Fatal("expected the failing batch to be reported")
	}
	if !strings.Contains(err.Error(), "移动第 101-200 个文件") {
		t.Fatalf("error = %v", err)
	}
	if len(batches) != 2 {
		t.Fatalf("batches = %v, want the move to stop at the failing one", batches)
	}
}
