package quark

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// Config reads quark_config.json and keeps every field it does not know
// about, so a write-back never drops upstream-only settings.
type Config struct {
	Path       string
	Cookie     json.RawMessage
	Plugins    map[string]json.RawMessage
	MagicRegex []Rule
	Tasks      []*Task
	raw        map[string]json.RawMessage
	order      []string
}

// TaskDownload holds the per-task override for the built-in downloader.
type TaskDownload struct {
	Enable    bool   `json:"enable"`
	Dir       string `json:"dir,omitempty"`
	Overwrite bool   `json:"overwrite,omitempty"`
}

type Task struct {
	TaskName               string
	ShareURL               string
	SavePath               string
	Pattern                string
	Replace                string
	MagicID                string
	StartFID               string
	EndDate                string
	RunWeek                []int
	runWeekSet             bool
	RunTime                []string
	runTimeSet             bool
	Mode                   string
	OnceDoneAt             string
	UpdateSubdir           string
	UpdateSubdirResaveMode bool
	IgnoreExtension        bool
	ShareURLBan            string
	LocalDownload          *TaskDownload
	Addition               json.RawMessage
	raw                    map[string]json.RawMessage
	order                  []string
}

func LoadConfig(path string) (*Config, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	cfg := &Config{Path: path, raw: doc, order: topLevelOrder(raw)}
	cfg.Cookie = doc["cookie"]
	if value, ok := doc["plugins"]; ok && len(value) > 0 {
		if err := json.Unmarshal(value, &cfg.Plugins); err != nil {
			return nil, fmt.Errorf("parse plugins: %w", err)
		}
	}
	if value, ok := doc["magic_regex"]; ok && len(value) > 0 {
		rules, err := ParseRules(value)
		if err != nil {
			return nil, fmt.Errorf("parse magic_regex: %w", err)
		}
		cfg.MagicRegex = rules
		// Rewrite the legacy name-keyed object into the list form so ids are
		// persisted on the next save.
		if err := cfg.SetKey("magic_regex", rules); err != nil {
			return nil, err
		}
	}
	if value, ok := doc["tasklist"]; ok && len(value) > 0 {
		if err := json.Unmarshal(value, &cfg.Tasks); err != nil {
			return nil, fmt.Errorf("parse tasklist: %w", err)
		}
	}
	cfg.migrateTaskMagic()
	return cfg, nil
}

// migrateTaskMagic turns legacy "pattern is the rule name" tasks into stable
// rule references so renaming a rule never touches tasks.
func (c *Config) migrateTaskMagic() {
	if len(c.MagicRegex) == 0 {
		return
	}
	ids := make(map[string]string, len(c.MagicRegex))
	for _, rule := range c.MagicRegex {
		if rule.Name == "" {
			continue
		}
		if _, ok := ids[rule.Name]; !ok {
			ids[rule.Name] = rule.ID
		}
	}
	for _, task := range c.Tasks {
		if task.MagicID != "" || task.Pattern == "" {
			continue
		}
		if id, ok := ids[task.Pattern]; ok {
			task.MagicID = id
			task.Pattern = ""
		}
	}
}

// DeriveRuleID builds a stable id from a rule name so legacy configs and
// defaults keep the same id across restarts.
func DeriveRuleID(name string) string {
	sum := sha1.Sum([]byte(name))
	return "m" + hex.EncodeToString(sum[:4])
}

// NormalizeRules fills missing ids and names and keeps ids unique.
func NormalizeRules(rules []Rule) []Rule {
	out := make([]Rule, 0, len(rules))
	seen := map[string]bool{}
	for index, rule := range rules {
		rule.ID = strings.TrimSpace(rule.ID)
		rule.Name = strings.TrimSpace(rule.Name)
		base := rule.ID
		if base == "" {
			name := rule.Name
			if name == "" {
				name = fmt.Sprintf("rule-%d", index+1)
			}
			base = DeriveRuleID(name)
		}
		id := base
		for suffix := 2; seen[id]; suffix++ {
			id = fmt.Sprintf("%s-%d", base, suffix)
		}
		rule.ID = id
		if rule.Name == "" {
			rule.Name = rule.ID
		}
		seen[id] = true
		out = append(out, rule)
	}
	return out
}

// ParseRules accepts both the list form and the legacy name-keyed object.
func ParseRules(raw json.RawMessage) ([]Rule, error) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 || string(trimmed) == "null" {
		return nil, nil
	}
	if trimmed[0] == '[' {
		var rules []Rule
		if err := json.Unmarshal(trimmed, &rules); err != nil {
			return nil, err
		}
		return NormalizeRules(rules), nil
	}
	var legacy map[string]Rule
	if err := json.Unmarshal(trimmed, &legacy); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(legacy))
	for name := range legacy {
		names = append(names, name)
	}
	sort.Strings(names)
	rules := make([]Rule, 0, len(names))
	for _, name := range names {
		rule := legacy[name]
		rule.Name = name
		rules = append(rules, rule)
	}
	return NormalizeRules(rules), nil
}

// RuleByID finds a rule by its stable id.
func (c *Config) RuleByID(id string) (Rule, bool) {
	if id == "" {
		return Rule{}, false
	}
	for _, rule := range c.MagicRegex {
		if rule.ID == id {
			return rule, true
		}
	}
	return Rule{}, false
}

// RuleName returns the rule name for a stable id, or "" when unknown.
func (c *Config) RuleName(id string) string {
	if rule, ok := c.RuleByID(id); ok {
		return rule.Name
	}
	return ""
}

// Cookies returns the configured cookies; a multi-line string is split the
// same way the Python config loader does.
func (c *Config) Cookies() []string {
	var list []string
	if err := json.Unmarshal(c.Cookie, &list); err == nil {
		return cleanCookies(list)
	}
	var single string
	if err := json.Unmarshal(c.Cookie, &single); err == nil {
		return cleanCookies(strings.Split(single, "\n"))
	}
	return nil
}

func cleanCookies(in []string) []string {
	var out []string
	for _, item := range in {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}
	return out
}

// Raw returns a top-level field as stored on disk.
func (c *Config) Raw(key string) json.RawMessage {
	return c.raw[key]
}

// SetKey replaces a top-level field; Save writes it back in place.
func (c *Config) SetKey(key string, value any) error {
	encoded, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("encode %s: %w", key, err)
	}
	if c.raw == nil {
		c.raw = map[string]json.RawMessage{}
	}
	c.raw[key] = encoded
	switch key {
	case "cookie":
		c.Cookie = encoded
	case "magic_regex":
		rules, err := ParseRules(encoded)
		if err != nil {
			return fmt.Errorf("parse magic_regex: %w", err)
		}
		// Store the normalized list (ids/names filled in), not the caller's
		// raw shape.
		normalized, err := json.Marshal(rules)
		if err != nil {
			return err
		}
		c.MagicRegex = rules
		c.raw[key] = normalized
	case "plugins":
		c.Plugins = nil
		if err := json.Unmarshal(encoded, &c.Plugins); err != nil {
			return fmt.Errorf("parse plugins: %w", err)
		}
	case "tasklist":
		c.Tasks = nil
		if err := json.Unmarshal(encoded, &c.Tasks); err != nil {
			return fmt.Errorf("parse tasklist: %w", err)
		}
	}
	return nil
}

// ReplaceTasks swaps the task list and keeps the raw document in sync.
func (c *Config) ReplaceTasks(tasks []*Task) error {
	c.Tasks = tasks
	encoded, err := json.Marshal(tasks)
	if err != nil {
		return err
	}
	if c.raw == nil {
		c.raw = map[string]json.RawMessage{}
	}
	c.raw["tasklist"] = encoded
	return nil
}

func (c *Config) Save() error {
	if c.raw == nil {
		c.raw = map[string]json.RawMessage{}
	}
	if c.Tasks != nil {
		tasks, err := json.Marshal(c.Tasks)
		if err != nil {
			return err
		}
		c.raw["tasklist"] = tasks
	}
	if c.Cookie != nil {
		c.raw["cookie"] = c.Cookie
	}
	order := c.order
	for key := range c.raw {
		if !containsString(order, key) {
			order = append(order, key)
		}
	}
	var buf bytes.Buffer
	buf.WriteString("{")
	for _, key := range order {
		value, ok := c.raw[key]
		if !ok {
			continue
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, value); err != nil {
			return fmt.Errorf("compact %s: %w", key, err)
		}
		if buf.Len() > 1 {
			buf.WriteString(",")
		}
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return err
		}
		buf.Write(encodedKey)
		buf.WriteString(":")
		buf.Write(compact.Bytes())
	}
	buf.WriteString("}")
	var indented bytes.Buffer
	if err := json.Indent(&indented, buf.Bytes(), "", "  "); err != nil {
		return err
	}
	return os.WriteFile(c.Path, indented.Bytes(), 0o644)
}

func (t *Task) UnmarshalJSON(data []byte) error {
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	t.raw = m
	t.order = topLevelOrder(data)
	t.TaskName = stringField(m, "taskname")
	t.ShareURL = stringField(m, "shareurl")
	t.SavePath = stringField(m, "savepath")
	t.Pattern = stringField(m, "pattern")
	t.Replace = stringField(m, "replace")
	t.MagicID = stringField(m, "magic_id")
	t.StartFID = stringField(m, "startfid")
	t.EndDate = stringField(m, "enddate")
	t.UpdateSubdir = stringField(m, "update_subdir")
	t.ShareURLBan = stringField(m, "shareurl_ban")
	t.Mode = stringField(m, "mode")
	t.OnceDoneAt = stringField(m, "once_done_at")
	t.IgnoreExtension = boolField(m, "ignore_extension")
	t.UpdateSubdirResaveMode = boolField(m, "update_subdir_resave_mode")
	t.Addition = m["addition"]
	if value, ok := m["local_download"]; ok {
		var download TaskDownload
		if err := json.Unmarshal(value, &download); err != nil {
			return fmt.Errorf("parse local_download: %w", err)
		}
		t.LocalDownload = &download
	}
	if value, ok := m["runweek"]; ok {
		t.runWeekSet = true
		if err := json.Unmarshal(value, &t.RunWeek); err != nil {
			return fmt.Errorf("parse runweek: %w", err)
		}
	}
	if value, ok := m["runtime"]; ok {
		t.runTimeSet = true
		if err := json.Unmarshal(value, &t.RunTime); err != nil {
			return fmt.Errorf("parse runtime: %w", err)
		}
	}
	return nil
}

func (t *Task) MarshalJSON() ([]byte, error) {
	m := make(map[string]json.RawMessage, len(t.raw)+8)
	for key, value := range t.raw {
		m[key] = value
	}
	setString(m, "taskname", t.TaskName)
	setString(m, "shareurl", t.ShareURL)
	setString(m, "savepath", t.SavePath)
	setString(m, "pattern", t.Pattern)
	setString(m, "replace", t.Replace)
	setString(m, "magic_id", t.MagicID)
	setString(m, "startfid", t.StartFID)
	setString(m, "enddate", t.EndDate)
	setString(m, "update_subdir", t.UpdateSubdir)
	setString(m, "shareurl_ban", t.ShareURLBan)
	setBool(m, "ignore_extension", t.IgnoreExtension)
	setBool(m, "update_subdir_resave_mode", t.UpdateSubdirResaveMode)
	// Empty values are dropped so switching back to the scheduled mode does
	// not leave stale keys in the config.
	if t.Mode != "" {
		setString(m, "mode", t.Mode)
	} else {
		delete(m, "mode")
	}
	if t.OnceDoneAt != "" {
		setString(m, "once_done_at", t.OnceDoneAt)
	} else {
		delete(m, "once_done_at")
	}
	if t.runWeekSet || len(t.RunWeek) > 0 {
		encoded, err := json.Marshal(t.RunWeek)
		if err != nil {
			return nil, err
		}
		m["runweek"] = encoded
	}
	if t.runTimeSet || len(t.RunTime) > 0 {
		encoded, err := json.Marshal(t.RunTime)
		if err != nil {
			return nil, err
		}
		m["runtime"] = encoded
	}
	if t.LocalDownload != nil {
		encoded, err := json.Marshal(t.LocalDownload)
		if err != nil {
			return nil, err
		}
		m["local_download"] = encoded
	}
	if len(t.Addition) > 0 {
		m["addition"] = t.Addition
	}
	order := t.order
	for key := range m {
		if !containsString(order, key) {
			order = append(order, key)
		}
	}
	var buf bytes.Buffer
	buf.WriteString("{")
	first := true
	for _, key := range order {
		value, ok := m[key]
		if !ok {
			continue
		}
		if !first {
			buf.WriteString(",")
		}
		first = false
		encodedKey, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		buf.Write(encodedKey)
		buf.WriteString(":")
		buf.Write(value)
	}
	buf.WriteString("}")
	return buf.Bytes(), nil
}

func (t *Task) HasRunWeek() bool { return t.runWeekSet }

// IsOnce marks tasks that run a single time and never join the scheduler.
func (t *Task) IsOnce() bool { return t.Mode == "once" }

// NormalizeMode drops unknown values so only "once" changes the schedule.
func (t *Task) NormalizeMode() {
	if t.Mode != "once" {
		t.Mode = ""
	}
}

// OnceFinished reports a one-shot task that already completed a real run.
func (t *Task) OnceFinished() bool { return t.IsOnce() && t.OnceDoneAt != "" }

// RemoveFields drops UI-only or derived keys so they are not written back.
func (t *Task) RemoveFields(names ...string) {
	for _, name := range names {
		delete(t.raw, name)
		kept := t.order[:0]
		for _, key := range t.order {
			if key != name {
				kept = append(kept, key)
			}
		}
		t.order = kept
	}
}

// IsScheduled mirrors is_time(): the end date is inclusive and runweek uses
// 1=Monday..7=Sunday.
func (t *Task) IsScheduled(now time.Time) (bool, error) {
	if t.EndDate != "" {
		end, err := time.ParseInLocation("2006-01-02", t.EndDate, now.Location())
		if err != nil {
			return false, fmt.Errorf("enddate %q: %w", t.EndDate, err)
		}
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		if today.After(end) {
			return false, nil
		}
	}
	if t.runWeekSet {
		today := int(now.Weekday())
		if today == 0 {
			today = 7
		}
		for _, day := range t.RunWeek {
			if day == today {
				return true, nil
			}
		}
		return false, nil
	}
	return true, nil
}

func stringField(m map[string]json.RawMessage, key string) string {
	var out string
	if value, ok := m[key]; ok {
		_ = json.Unmarshal(value, &out)
	}
	return out
}

func boolField(m map[string]json.RawMessage, key string) bool {
	var out bool
	if value, ok := m[key]; ok {
		_ = json.Unmarshal(value, &out)
	}
	return out
}

func setString(m map[string]json.RawMessage, key, value string) {
	_, present := m[key]
	if value == "" && !present {
		return
	}
	encoded, _ := json.Marshal(value)
	m[key] = encoded
}

func setBool(m map[string]json.RawMessage, key string, value bool) {
	_, present := m[key]
	if !value && !present {
		return
	}
	encoded, _ := json.Marshal(value)
	m[key] = encoded
}

// topLevelOrder collects object keys at depth 0 in their original order.
func topLevelOrder(data []byte) []string {
	decoder := json.NewDecoder(bytes.NewReader(data))
	depth := 0
	var keys []string
	for {
		token, err := decoder.Token()
		if err != nil {
			return keys
		}
		switch value := token.(type) {
		case json.Delim:
			switch value {
			case '{', '[':
				depth++
			case '}', ']':
				depth--
			}
		case string:
			if depth == 1 {
				keys = append(keys, value)
			}
		}
	}
}

func containsString(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
