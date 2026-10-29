package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type runLogMeta struct {
	ID         string     `json:"id"`
	Label      string     `json:"label"`
	StartedAt  time.Time  `json:"started_at"`
	FinishedAt *time.Time `json:"finished_at,omitempty"`
	Done       bool       `json:"done"`
	Error      string     `json:"error,omitempty"`
	SavedTotal int        `json:"saved_total"`
	Dry        bool       `json:"dry,omitempty"`
	Lines      int        `json:"lines"`
}

// runLogStore persists finished runs as <id>.json + <id>.log and keeps only
// the newest N pairs.
type runLogStore struct {
	dir string
	mu  sync.Mutex
}

func newRunLogStore(dir string) *runLogStore {
	return &runLogStore{dir: dir}
}

func (s *runLogStore) save(run *Run, keep int) error {
	view := run.View()
	lines := run.AllLines()
	meta := runLogMeta{
		ID:         view.ID,
		Label:      view.Label,
		StartedAt:  view.StartedAt,
		FinishedAt: view.FinishedAt,
		Done:       true,
		Error:      view.Error,
		SavedTotal: view.SavedTotal,
		Dry:        view.Dry,
		Lines:      len(lines),
	}
	encoded, err := json.Marshal(meta)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.MkdirAll(s.dir, 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, meta.ID+".json"), encoded, 0o644); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(s.dir, meta.ID+".log"), []byte(strings.Join(lines, "\n")), 0o644); err != nil {
		return err
	}
	s.pruneLocked(keep)
	return nil
}

func (s *runLogStore) list() []runLogMeta {
	s.mu.Lock()
	defer s.mu.Unlock()
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil
	}
	metas := make([]runLogMeta, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var meta runLogMeta
		if err := json.Unmarshal(raw, &meta); err != nil || meta.ID == "" {
			continue
		}
		if meta.FinishedAt == nil {
			finished := meta.StartedAt
			meta.FinishedAt = &finished
		}
		metas = append(metas, meta)
	}
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].StartedAt.After(metas[j].StartedAt)
	})
	return metas
}

func (s *runLogStore) read(id string) (runLogMeta, []string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !validRunID(id) {
		return runLogMeta{}, nil, false
	}
	raw, err := os.ReadFile(filepath.Join(s.dir, id+".json"))
	if err != nil {
		return runLogMeta{}, nil, false
	}
	var meta runLogMeta
	if err := json.Unmarshal(raw, &meta); err != nil {
		return runLogMeta{}, nil, false
	}
	logRaw, err := os.ReadFile(filepath.Join(s.dir, id+".log"))
	if err != nil {
		return meta, nil, true
	}
	text := strings.TrimSuffix(string(logRaw), "\n")
	lines := []string{}
	if text != "" {
		lines = strings.Split(text, "\n")
	}
	return meta, lines, true
}

func (s *runLogStore) pruneLocked(keep int) {
	if keep <= 0 {
		return
	}
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return
	}
	ids := make([]string, 0, len(entries))
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			ids = append(ids, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	if len(ids) <= keep {
		return
	}
	sort.Strings(ids)
	for _, id := range ids[:len(ids)-keep] {
		_ = os.Remove(filepath.Join(s.dir, id+".json"))
		_ = os.Remove(filepath.Join(s.dir, id+".log"))
	}
}

func validRunID(id string) bool {
	if id == "" || len(id) > 64 {
		return false
	}
	for _, char := range id {
		switch {
		case char >= 'a' && char <= 'z', char >= 'A' && char <= 'Z', char >= '0' && char <= '9', char == '-':
		default:
			return false
		}
	}
	return true
}

func metaView(meta runLogMeta) RunView {
	return RunView{
		ID:         meta.ID,
		Label:      meta.Label,
		StartedAt:  meta.StartedAt,
		FinishedAt: meta.FinishedAt,
		Done:       meta.Done,
		Error:      meta.Error,
		SavedTotal: meta.SavedTotal,
		Dry:        meta.Dry,
	}
}

func (s *runLogStore) views() []RunView {
	metas := s.list()
	views := make([]RunView, 0, len(metas))
	for _, meta := range metas {
		views = append(views, metaView(meta))
	}
	return views
}
