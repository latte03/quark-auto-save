package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// taskStatsStore keeps the last transfer count per task on disk, so the task
// list's "最新" column survives restarts instead of resetting to "—".
type taskStatsStore struct {
	mu    sync.RWMutex
	path  string
	stats map[string]TaskStats
}

type taskStatEntry struct {
	Saved int       `json:"saved"`
	At    time.Time `json:"at"`
}

type taskStatsFile struct {
	Stats map[string]taskStatEntry `json:"stats"`
}

func newTaskStatsStore(path string) *taskStatsStore {
	store := &taskStatsStore{
		path:  path,
		stats: map[string]TaskStats{},
	}
	raw, err := os.ReadFile(path)
	if err == nil {
		var doc taskStatsFile
		if json.Unmarshal(raw, &doc) == nil && doc.Stats != nil {
			for name, entry := range doc.Stats {
				store.stats[name] = TaskStats{Saved: entry.Saved, At: entry.At}
			}
		}
	}
	return store
}

// record merges one finished write run's per-task counts. A task absent from
// this run keeps its previous number: "latest transfer" is per task, not per
// run. Save failures only affect the display after a restart.
func (s *taskStatsStore) record(stats map[string]int) {
	if len(stats) == 0 {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	for name, saved := range stats {
		s.stats[name] = TaskStats{Saved: saved, At: now}
	}
	if raw, err := json.MarshalIndent(&taskStatsFile{Stats: func() map[string]taskStatEntry {
		out := make(map[string]taskStatEntry, len(s.stats))
		for name, stat := range s.stats {
			out[name] = taskStatEntry{Saved: stat.Saved, At: stat.At}
		}
		return out
	}()}, "", "  "); err == nil {
		_ = os.MkdirAll(filepath.Dir(s.path), 0o755)
		_ = os.WriteFile(s.path, raw, 0o644)
	}
}

func (s *taskStatsStore) all() map[string]TaskStats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make(map[string]TaskStats, len(s.stats))
	for key, value := range s.stats {
		out[key] = value
	}
	return out
}
