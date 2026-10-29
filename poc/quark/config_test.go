package quark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

const sampleConfig = `{
  "cookie": "a=1\nb=2",
  "push_config": {"CONSOLE": true},
  "plugins": {"alist": {"enable": true}},
  "magic_regex": {"$TV": {"pattern": "(\\d+)", "replace": "E\\1"}},
  "tasklist": [
    {
      "taskname": "剧集",
      "shareurl": "https://pan.quark.cn/s/abc",
      "savepath": "/tv",
      "pattern": "$TV",
      "replace": "",
      "enddate": "2099-01-30",
      "runweek": [1, 3],
      "x_unknown": {"keep": [1, 2]}
    }
  ],
  "source": {"cloudsaver": {"enable": false}}
}`

func TestLoadConfigPreservesUnknownFields(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte(sampleConfig), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := cfg.Cookies(); len(got) != 2 || got[0] != "a=1" || got[1] != "b=2" {
		t.Fatalf("cookies = %#v", got)
	}
	if len(cfg.Tasks) != 1 {
		t.Fatalf("tasks = %d", len(cfg.Tasks))
	}
	task := cfg.Tasks[0]
	if task.TaskName != "剧集" || task.SavePath != "/tv" || task.EndDate != "2099-01-30" {
		t.Fatalf("task = %#v", task)
	}
	if !task.runWeekSet || len(task.RunWeek) != 2 || task.RunWeek[1] != 3 {
		t.Fatalf("runweek = %#v", task.RunWeek)
	}
	// The legacy "pattern is the rule name" form is migrated to a stable id.
	if task.Pattern != "" || task.MagicID == "" {
		t.Fatalf("pattern/magic_id = %q/%q", task.Pattern, task.MagicID)
	}
	if rule, ok := cfg.RuleByID(task.MagicID); !ok || rule.Name != "$TV" {
		t.Fatalf("rule for task = %+v", rule)
	}

	task.ShareURLBan = "分享已失效"
	if err := cfg.Save(); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(saved)
	for _, want := range []string{`"source"`, `"x_unknown"`, `"shareurl_ban": "分享已失效"`, `"runweek"`} {
		if !strings.Contains(text, want) {
			t.Errorf("saved config lost %s:\n%s", want, text)
		}
	}
	if strings.Index(text, `"cookie"`) > strings.Index(text, `"tasklist"`) {
		t.Errorf("top-level key order changed:\n%s", text)
	}

	reloaded, err := LoadConfig(path)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Tasks[0].ShareURLBan != "分享已失效" || len(reloaded.Tasks[0].RunWeek) != 2 {
		t.Fatalf("round-trip task = %#v", reloaded.Tasks[0])
	}
}

func TestTaskIsScheduled(t *testing.T) {
	monday := time.Date(2026, 10, 26, 9, 0, 0, 0, time.Local) // Monday
	weekday := func(t *testing.T, raw string) *Task {
		t.Helper()
		var task Task
		if err := task.UnmarshalJSON([]byte(raw)); err != nil {
			t.Fatal(err)
		}
		return &task
	}
	cases := []struct {
		name string
		task *Task
		when time.Time
		want bool
	}{
		{"no schedule", weekday(t, `{}`), monday, true},
		{"enddate today", weekday(t, `{"enddate":"2026-10-26"}`), monday, true},
		{"enddate past", weekday(t, `{"enddate":"2026-10-25"}`), monday, false},
		{"runweek monday", weekday(t, `{"runweek":[1,3]}`), monday, true},
		{"runweek miss", weekday(t, `{"runweek":[2,4]}`), monday, false},
		{"empty runweek", weekday(t, `{"runweek":[]}`), monday, false},
		{"sunday is 7", weekday(t, `{"runweek":[7]}`), monday.AddDate(0, 0, 6), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := c.task.IsScheduled(c.when)
			if err != nil {
				t.Fatal(err)
			}
			if got != c.want {
				t.Errorf("IsScheduled = %v, want %v", got, c.want)
			}
		})
	}
	if _, err := weekday(t, `{"enddate":"nope"}`).IsScheduled(monday); err == nil {
		t.Fatal("expected an error for a malformed enddate")
	}
}

func TestExtractSharePath(t *testing.T) {
	raw := "https://pan.quark.cn/s/d07a34a9c695#/list/share/7e25ddd87cf64443b637125478733295-Show*101Name/71df3902f42d4270a58c0eb12aa2b014-%E7%BB%BC%E8%89%BA"
	ref, err := ExtractShareURL(raw)
	if err != nil {
		t.Fatal(err)
	}
	if ref.PwdID != "d07a34a9c695" || ref.PdirFID != "71df3902f42d4270a58c0eb12aa2b014" {
		t.Fatalf("ref = %#v", ref)
	}
	if len(ref.Paths) != 2 || ref.Paths[0].Name != "Show-Name" || ref.Paths[1].Name != "综艺" {
		t.Fatalf("paths = %#v", ref.Paths)
	}
	root, err := ExtractShareURL("https://pan.quark.cn/s/abc")
	if err != nil || root.PdirFID != "0" || len(root.Paths) != 0 {
		t.Fatalf("root ref = %#v, err = %v", root, err)
	}
}
