package server

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func makeRun(id, label string, write bool, started time.Time, lines ...string) *Run {
	run := &Run{ID: id, Label: label, Write: write, StartedAt: started}
	for _, line := range lines {
		run.Logf("%s", line)
	}
	run.finish(nil)
	return run
}

func TestRunLogStoreSaveListPrune(t *testing.T) {
	dir := t.TempDir()
	store := newRunLogStore(filepath.Join(dir, "runs"))
	base := time.Date(2026, 10, 29, 10, 0, 0, 0, time.UTC)
	for index, id := range []string{"run-1", "run-2", "run-3"} {
		run := makeRun(id, "任务"+id, index != 1, base.Add(time.Duration(index)*time.Minute), "line a", "line b")
		if err := store.save(run, 2); err != nil {
			t.Fatal(err)
		}
	}

	views := store.views()
	if len(views) != 2 {
		t.Fatalf("views = %d, want 2", len(views))
	}
	if views[0].ID != "run-3" || views[1].ID != "run-2" {
		t.Fatalf("unexpected order: %+v", views)
	}
	if views[0].Dry || !views[1].Dry {
		t.Fatalf("dry flags wrong: %+v", views)
	}

	if _, _, ok := store.read("run-1"); ok {
		t.Fatal("pruned run should be gone")
	}
	_, lines, ok := store.read("run-3")
	if !ok || len(lines) != 2 || lines[0] != "line a" {
		t.Fatalf("read run-3 = %v, %v", lines, ok)
	}

	if _, err := os.Stat(filepath.Join(dir, "runs", "run-1.log")); !os.IsNotExist(err) {
		t.Fatal("pruned log file should be deleted")
	}
}

func TestRunLogReadRejectsBadIDs(t *testing.T) {
	store := newRunLogStore(t.TempDir())
	for _, id := range []string{"", "../secret", "a/b", "run id"} {
		if _, _, ok := store.read(id); ok {
			t.Fatalf("id %q must be rejected", id)
		}
	}
}
