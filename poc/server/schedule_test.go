package server

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"qas-poc/quark"
)

func mustTask(t *testing.T, raw string) *quark.Task {
	t.Helper()
	var task quark.Task
	if err := json.Unmarshal([]byte(raw), &task); err != nil {
		t.Fatalf("parse task: %v", err)
	}
	return &task
}

func TestParseClock(t *testing.T) {
	cases := map[string]struct {
		want string
		ok   bool
	}{
		"08:00": {"08:00", true},
		"8:5":   {"08:05", true},
		"23:59": {"23:59", true},
		"24:00": {"", false},
		"08:60": {"", false},
		"0800":  {"", false},
		"":      {"", false},
	}
	for input, want := range cases {
		got, ok := parseClock(input)
		if got != want.want || ok != want.ok {
			t.Fatalf("parseClock(%q) = %q,%v want %q,%v", input, got, ok, want.want, want.ok)
		}
	}
}

func TestCrontabTimes(t *testing.T) {
	times, ok := crontabTimes("0 8,18,20 * * *")
	if !ok {
		t.Fatal("expected explicit crontab to convert")
	}
	want := []string{"08:00", "18:00", "20:00"}
	if !reflect.DeepEqual(times, want) {
		t.Fatalf("times = %v", times)
	}
	if _, ok := crontabTimes("*/5 * * * *"); ok {
		t.Fatal("step crontab must stay legacy")
	}
	if _, ok := crontabTimes("0 8 * * 1"); ok {
		t.Fatal("weekday crontab must stay legacy")
	}
	if _, ok := crontabTimes("0 8,30 * * *"); ok {
		t.Fatal("invalid hour must be rejected")
	}
}

func TestTaskTimesPrefersRuntimeAndFallsBack(t *testing.T) {
	task := mustTask(t, `{"taskname":"a","runtime":["20:00","8:00","08:00"]}`)
	got := taskTimes(task, "0 12 * * *")
	want := []string{"08:00", "20:00"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("task times = %v", got)
	}
	legacy := mustTask(t, `{"taskname":"b"}`)
	got = taskTimes(legacy, "0 8,18 * * *")
	if !reflect.DeepEqual(got, []string{"08:00", "18:00"}) {
		t.Fatalf("fallback times = %v", got)
	}
}

func TestDueTaskIndices(t *testing.T) {
	monday := time.Date(2026, 10, 26, 8, 0, 0, 0, time.Local) // Monday
	early := mustTask(t, `{"taskname":"early","runtime":["08:00"],"runweek":[1,2,3,4,5]}`)
	late := mustTask(t, `{"taskname":"late","runtime":["20:00"]}`)
	legacy := mustTask(t, `{"taskname":"legacy"}`)
	expired := mustTask(t, `{"taskname":"expired","runtime":["08:00"],"enddate":"2026-10-01"}`)
	tasks := []*quark.Task{early, late, legacy, expired}

	got := dueTaskIndices(tasks, "0 8,20 * * *", monday)
	if !reflect.DeepEqual(got, []int{0, 2}) {
		t.Fatalf("due at 08:00 = %v", got)
	}
	got = dueTaskIndices(tasks, "0 8,20 * * *", monday.Add(12*time.Hour))
	if !reflect.DeepEqual(got, []int{1, 2}) {
		t.Fatalf("due at 20:00 = %v", got)
	}
}

func TestLegacyTaskIndices(t *testing.T) {
	now := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)
	modern := mustTask(t, `{"taskname":"modern","runtime":["08:00"]}`)
	legacy := mustTask(t, `{"taskname":"legacy"}`)
	// Only a crontab without clock times leaves tasks on the legacy path.
	got := legacyTaskIndices([]*quark.Task{modern, legacy}, "*/5 * * * *", now)
	if !reflect.DeepEqual(got, []int{1}) {
		t.Fatalf("legacy indices = %v", got)
	}
	got = legacyTaskIndices([]*quark.Task{modern, legacy}, "0 8 * * *", now)
	if len(got) != 0 {
		t.Fatalf("legacy indices with explicit crontab = %v", got)
	}
}

func TestNextTimesForTask(t *testing.T) {
	monday := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local)
	task := mustTask(t, `{"taskname":"a","runtime":["08:00","20:00"],"runweek":[1],"enddate":"2026-11-02"}`)
	times := nextTimesForTask(task, "", monday, 3)
	if len(times) != 3 {
		t.Fatalf("times = %v", times)
	}
	want := []time.Time{
		time.Date(2026, 10, 26, 20, 0, 0, 0, time.Local),
		time.Date(2026, 11, 2, 8, 0, 0, 0, time.Local),
		time.Date(2026, 11, 2, 20, 0, 0, 0, time.Local),
	}
	if !reflect.DeepEqual(times, want) {
		t.Fatalf("times = %v", times)
	}
	if label := nextRunLabel(task, "", monday); label != "今天 20:00" {
		t.Fatalf("next run label = %q", label)
	}
	label, at := nextRunInfo(task, "", monday)
	if label != "今天 20:00" || !at.Equal(time.Date(2026, 10, 26, 20, 0, 0, 0, time.Local)) {
		t.Fatalf("next run info = %q / %v", label, at)
	}
	once := mustTask(t, `{"taskname":"once","mode":"once"}`)
	if label, at := nextRunInfo(once, "", monday); label != "—" || !at.IsZero() {
		t.Fatalf("once next run info = %q / %v", label, at)
	}
}

func TestConfigNextRunsFallsBackToCrontab(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(`{"crontab":"0 8,18 * * *","tasklist":[]}`), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := quark.LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	out := configNextRuns(cfg, time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local))
	if len(out) != 3 || out[0] != "今天 18:00" {
		t.Fatalf("fallback next runs = %v", out)
	}
}

func TestTaskRuntimeRoundTrip(t *testing.T) {
	task := mustTask(t, `{"taskname":"a","runtime":["08:00","20:00"]}`)
	raw, err := json.Marshal(task)
	if err != nil {
		t.Fatal(err)
	}
	var out struct {
		Runtime []string `json:"runtime"`
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(out.Runtime, []string{"08:00", "20:00"}) {
		t.Fatalf("runtime = %v", out.Runtime)
	}
}
