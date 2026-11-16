package plugins

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"qas-poc/quark"
)

// writeEnvelope answers like Quark does: {"status":200,"code":0,"data":...}.
func writeEnvelope(w http.ResponseWriter, data any) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status": 200, "code": 0, "message": "ok", "data": data,
	})
}

// TestAutoUnarchivePollsRealClient drives the plugin through the real client
// against a mock drive that reproduces the observed behaviour: the unarchive
// task reports done while its folder still lists empty for a moment.
func TestAutoUnarchivePollsRealClient(t *testing.T) {
	var (
		mu       sync.Mutex
		sortHits = map[string]int{}
		unpacked int
		renames  []renameCall
		deleted  []string
		moved    []string
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		switch {
		case strings.HasSuffix(r.URL.Path, "/archive/unarchive"):
			unpacked++
			writeEnvelope(w, map[string]any{"task_id": "un-1"})
		case strings.HasSuffix(r.URL.Path, "/task"):
			writeEnvelope(w, map[string]any{
				"status":     2,
				"task_title": "show",
				"unarchive_result": map[string]any{"list": []any{
					map[string]any{"fid": "sub1", "file_name": "show", "size": 0, "include_items": 1},
				}},
			})
		case strings.HasSuffix(r.URL.Path, "/file/sort"):
			folder := r.URL.Query().Get("pdir_fid")
			// The client pages until an empty answer; only page 1 has entries.
			if page := r.URL.Query().Get("_page"); page != "" && page != "1" {
				writeEnvelope(w, map[string]any{"list": []any{}})
				return
			}
			sortHits[folder]++
			switch folder {
			case "sub1":
				if sortHits[folder] <= 2 {
					writeEnvelope(w, map[string]any{"list": []any{}})
					return
				}
				writeEnvelope(w, map[string]any{"list": []any{
					map[string]any{"fid": "f1", "file_name": "ep1.mkv", "size": 10},
				}})
			case "target":
				var list []any
				for _, fid := range moved {
					list = append(list, map[string]any{"fid": fid, "file_name": "show.mkv"})
				}
				writeEnvelope(w, map[string]any{"list": list})
			default:
				writeEnvelope(w, map[string]any{"list": []any{}})
			}
		case strings.HasSuffix(r.URL.Path, "/file/rename"):
			var body struct {
				FID      string `json:"fid"`
				FileName string `json:"file_name"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			renames = append(renames, renameCall{FID: body.FID, Name: body.FileName})
			writeEnvelope(w, map[string]any{})
		case strings.HasSuffix(r.URL.Path, "/file/move"):
			var body struct {
				Filelist []string `json:"filelist"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			moved = append(moved, body.Filelist...)
			writeEnvelope(w, map[string]any{"task_id": "move-1"})
		case strings.HasSuffix(r.URL.Path, "/file/delete"):
			var body struct {
				Filelist []string `json:"filelist"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			deleted = append(deleted, body.Filelist...)
			writeEnvelope(w, map[string]any{"task_id": "del-1"})
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			writeEnvelope(w, map[string]any{})
		}
	}))
	defer server.Close()

	client := quark.New("__pus=test")
	client.SetBaseURL(server.URL, "http://app.invalid")

	deps, logs := testDeps(t)
	plugin := NewAutoUnarchive(nil, deps)
	plugin.sleep = func(time.Duration) {}
	env := &quark.PluginEnv{
		Account:     client,
		Tree:        archiveTree(),
		SavepathFID: map[string]string{"/tv": "target"},
	}
	if err := plugin.Run(pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true}}`), env); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if unpacked != 1 {
		t.Fatalf("unarchive calls = %d, want 1", unpacked)
	}
	if sortHits["sub1"] < 3 {
		t.Fatalf("extract dir listed %d times, want the plugin to keep polling", sortHits["sub1"])
	}
	if len(renames) != 2 || renames[0] != (renameCall{"f1", "show.mkv"}) || renames[1] != (renameCall{"sub1", "show.zip"}) {
		t.Fatalf("renames = %#v", renames)
	}
	if len(moved) != 1 || moved[0] != "f1" {
		t.Fatalf("moved = %#v", moved)
	}
	if len(deleted) != 1 || deleted[0] != "zip1" {
		t.Fatalf("deleted = %#v", deleted)
	}
	if !hasLog(*logs, "解压结果延迟出现") || !hasLog(*logs, "批量清理完成") {
		t.Fatalf("logs = %v", *logs)
	}
}
