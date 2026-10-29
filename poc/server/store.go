package server

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"qas-poc/notify"
	"qas-poc/plugins"
	"qas-poc/quark"
	"qas-poc/source"
)

const defaultCrontab = "0 8,18,20 * * *"

// Store serializes access to quark_config.json for the HTTP handlers.
type Store struct {
	mu   sync.RWMutex
	cfg  *quark.Config
	path string
}

type Snapshot struct {
	Tasks    []*quark.Task
	Cookies  []string
	Magic    []quark.Rule
	Plugins  map[string]json.RawMessage
	Crontab  string
	Username string
	Password string
	Sources  source.Options
	Push     notify.Config
	Download localDownloadSettings
}

type defaultsNotice struct {
	GeneratedPassword string
}

func OpenStore(path string) (*Store, defaultsNotice, error) {
	cfg, err := quark.LoadConfig(path)
	if err != nil {
		return nil, defaultsNotice{}, err
	}
	notice, err := ensureDefaults(cfg)
	if err != nil {
		return nil, defaultsNotice{}, err
	}
	if err := cfg.Save(); err != nil {
		return nil, defaultsNotice{}, err
	}
	return &Store{cfg: cfg, path: path}, notice, nil
}

func ensureDefaults(cfg *quark.Config) (defaultsNotice, error) {
	notice := defaultsNotice{}
	var webui struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if value := cfg.Raw("webui"); len(value) > 0 {
		_ = json.Unmarshal(value, &webui)
	}
	if webui.Username == "" {
		webui.Username = firstNonEmpty(os.Getenv("WEBUI_USERNAME"), "admin")
	}
	if webui.Password == "" {
		if env := os.Getenv("WEBUI_PASSWORD"); env != "" {
			webui.Password = env
		} else {
			webui.Password = randomPassword()
			notice.GeneratedPassword = webui.Password
		}
	}
	if err := cfg.SetKey("webui", webui); err != nil {
		return notice, err
	}

	var crontab string
	if value := cfg.Raw("crontab"); len(value) > 0 {
		_ = json.Unmarshal(value, &crontab)
	}
	if crontab == "" {
		crontab = defaultCrontab
		if err := cfg.SetKey("crontab", crontab); err != nil {
			return notice, err
		}
	}

	if len(cfg.MagicRegex) == 0 {
		defaults := []quark.Rule{
			{
				Name:    "$TV",
				Pattern: `.*?([Ss]\d{1,2})?(?:[第EePpXx\.\-\_\( ]{1,2}|^)(\d{1,3})(?!\d).*?\.(mp4|mkv)`,
				Replace: `\1E\2.\3`,
			},
			{
				Name:    "$BLACK_WORD",
				Pattern: `^(?!.*纯享)(?!.*加更)(?!.*超前企划)(?!.*训练室)(?!.*蒸蒸日上).*`,
				Replace: "",
			},
		}
		if err := cfg.SetKey("magic_regex", defaults); err != nil {
			return notice, err
		}
	}

	if err := cfg.SetKey("plugins", mergedPlugins(cfg.Plugins)); err != nil {
		return notice, err
	}

	sources := defaultSources()
	if value := cfg.Raw("source"); len(value) > 0 {
		_ = json.Unmarshal(value, &sources)
	}
	if err := cfg.SetKey("source", sources); err != nil {
		return notice, err
	}

	localDownload := defaultLocalDownload()
	if value := cfg.Raw("local_download"); len(value) > 0 {
		var stored map[string]any
		if err := json.Unmarshal(value, &stored); err == nil && stored != nil {
			localDownload = mergeLocalDownload(localDownload, stored)
		}
	}
	if err := cfg.SetKey("local_download", localDownload); err != nil {
		return notice, err
	}

	runLogs := defaultRunLogs()
	if value := cfg.Raw("run_logs"); len(value) > 0 {
		var stored map[string]any
		if err := json.Unmarshal(value, &stored); err == nil && stored != nil {
			runLogs = mergeLocalDownload(runLogs, stored)
		}
	}
	if err := cfg.SetKey("run_logs", runLogs); err != nil {
		return notice, err
	}
	return notice, nil
}

func defaultLocalDownload() map[string]any {
	return map[string]any{
		"dir":         "",
		"concurrency": 2,
		"min_free_gb": 5,
		"overwrite":   false,
	}
}

func defaultRunLogs() map[string]any {
	return map[string]any{"keep": 20}
}

func mergeLocalDownload(defaults, stored map[string]any) map[string]any {
	out := make(map[string]any, len(defaults))
	for key, value := range defaults {
		out[key] = value
	}
	for key, value := range stored {
		out[key] = value
	}
	return out
}

func mergedPlugins(existing map[string]json.RawMessage) map[string]any {
	out := map[string]any{}
	for name, raw := range existing {
		var section map[string]any
		if err := json.Unmarshal(raw, &section); err == nil {
			out[name] = section
		}
	}
	for name, defaults := range plugins.ConfigDefaults() {
		section, ok := out[name].(map[string]any)
		if !ok {
			section = map[string]any{}
			out[name] = section
		}
		for key, value := range defaults {
			if _, present := section[key]; !present {
				section[key] = value
			}
		}
	}
	return out
}

// visiblePlugins limits the settings payload to plugins this build implements;
// sections for unported plugins stay untouched on disk.
func visiblePlugins(existing map[string]json.RawMessage) map[string]any {
	merged := mergedPlugins(existing)
	out := map[string]any{}
	for name := range plugins.ConfigDefaults() {
		out[name] = merged[name]
	}
	return out
}

func defaultSources() map[string]any {
	return map[string]any{
		"cloudsaver": map[string]any{
			"enable": false, "server": "", "username": "", "password": "", "token": "",
		},
		"pansou": map[string]any{"enable": false, "server": ""},
	}
}

// mergeSources keeps the built-in search source fields when the stored config
// only carries a subset.
func mergeSources(base map[string]any, value any) map[string]any {
	extra, ok := value.(map[string]any)
	if !ok {
		return base
	}
	for name, raw := range extra {
		patch, patchOK := raw.(map[string]any)
		current, currentOK := base[name].(map[string]any)
		if !patchOK || !currentOK {
			base[name] = raw
			continue
		}
		merged := make(map[string]any, len(current)+len(patch))
		for key, item := range current {
			merged[key] = item
		}
		for key, item := range patch {
			merged[key] = item
		}
		base[name] = merged
	}
	return base
}

func randomPassword() string {
	buffer := make([]byte, 9)
	if _, err := rand.Read(buffer); err != nil {
		return fmt.Sprintf("qas-%d", time.Now().Unix())
	}
	return base64.RawURLEncoding.EncodeToString(buffer)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func (s *Store) Snapshot() Snapshot {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.cfg
	snapshot := Snapshot{
		Cookies:  cfg.Cookies(),
		Magic:    cloneRules(cfg.MagicRegex),
		Plugins:  clonePlugins(cfg.Plugins),
		Username: webuiValue(cfg, "username", "admin"),
		Password: webuiValue(cfg, "password", ""),
		Crontab:  crontabValue(cfg),
		Push:     notify.Decode(cfg.Raw("push_config")),
	}
	for _, task := range cfg.Tasks {
		snapshot.Tasks = append(snapshot.Tasks, cloneTask(task))
	}
	snapshot.Sources = source.Options{}
	var sourceCfg struct {
		CloudSaver source.CloudSaverConfig `json:"cloudsaver"`
		PanSou     source.PanSouConfig     `json:"pansou"`
	}
	if raw := cfg.Raw("source"); len(raw) > 0 {
		_ = json.Unmarshal(raw, &sourceCfg)
	}
	snapshot.Sources.CloudSaver = sourceCfg.CloudSaver
	snapshot.Sources.PanSou = sourceCfg.PanSou
	snapshot.Download = localDownloadFromConfig(cfg)
	return snapshot
}

func webuiValue(cfg *quark.Config, key, fallback string) string {
	var webui map[string]string
	if raw := cfg.Raw("webui"); len(raw) > 0 {
		_ = json.Unmarshal(raw, &webui)
	}
	if value := webui[key]; value != "" {
		return value
	}
	return fallback
}

func crontabValue(cfg *quark.Config) string {
	var value string
	if raw := cfg.Raw("crontab"); len(raw) > 0 {
		_ = json.Unmarshal(raw, &value)
	}
	return value
}

func cloneTask(task *quark.Task) *quark.Task {
	raw, err := json.Marshal(task)
	if err != nil {
		return &quark.Task{}
	}
	var out quark.Task
	if err := json.Unmarshal(raw, &out); err != nil {
		return &quark.Task{}
	}
	return &out
}

func cloneRules(rules []quark.Rule) []quark.Rule {
	return append([]quark.Rule(nil), rules...)
}

func clonePlugins(in map[string]json.RawMessage) map[string]json.RawMessage {
	out := make(map[string]json.RawMessage, len(in))
	for key, value := range in {
		out[key] = append(json.RawMessage(nil), value...)
	}
	return out
}

// ConfigView renders the settings payload; secrets stay behind auth.
func (s *Store) ConfigView(version string, apiToken string) map[string]any {
	s.mu.RLock()
	defer s.mu.RUnlock()
	cfg := s.cfg
	cookies := cfg.Cookies()
	if cookies == nil {
		cookies = []string{}
	}
	view := map[string]any{
		"cookie":        cookies,
		"crontab":       crontabValue(cfg),
		"next_runs":     configNextRuns(cfg, time.Now()),
		"version":       version,
		"api_token":     apiToken,
		"plugin_status": plugins.Configured(cfg.Plugins),
		"webui":         map[string]string{"username": webuiValue(cfg, "username", "admin")},
	}
	for key, fallback := range map[string]any{
		"push_config":    map[string]any{},
		"magic_regex":    []any{},
		"source":         defaultSources(),
		"local_download": defaultLocalDownload(),
		"run_logs":       defaultRunLogs(),
	} {
		value := fallback
		if raw := cfg.Raw(key); len(raw) > 0 {
			var decoded any
			if err := json.Unmarshal(raw, &decoded); err == nil && decoded != nil {
				value = decoded
			}
		}
		if key == "source" {
			value = mergeSources(defaultSources(), value)
		}
		if key == "local_download" {
			if stored, ok := value.(map[string]any); ok {
				value = mergeLocalDownload(defaultLocalDownload(), stored)
			}
		}
		if key == "run_logs" {
			if stored, ok := value.(map[string]any); ok {
				value = mergeLocalDownload(defaultRunLogs(), stored)
			}
		}
		view[key] = value
	}
	view["plugins"] = visiblePlugins(cfg.Plugins)
	return view
}

// RunLogKeep is how many finished runs are kept on disk.
func (s *Store) RunLogKeep() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	raw := s.cfg.Raw("run_logs")
	if len(raw) == 0 {
		return 20
	}
	var stored struct {
		Keep int `json:"keep"`
	}
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Keep <= 0 {
		return 20
	}
	if stored.Keep > 200 {
		return 200
	}
	return stored.Keep
}

func (s *Store) WebUI() (string, string) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return webuiValue(s.cfg, "username", "admin"), webuiValue(s.cfg, "password", "")
}

func (s *Store) Crontab() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return crontabValue(s.cfg)
}

func (s *Store) Tasks() []*quark.Task {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*quark.Task, 0, len(s.cfg.Tasks))
	for _, task := range s.cfg.Tasks {
		out = append(out, cloneTask(task))
	}
	return out
}

// Update applies an allowlisted partial config and persists it.
func (s *Store) Update(partial map[string]json.RawMessage) error {
	allowed := map[string]bool{
		"cookie": true, "crontab": true, "push_config": true,
		"plugins": true, "magic_regex": true, "source": true,
		"local_download": true,
		"run_logs":       true,
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, raw := range partial {
		if !allowed[key] {
			continue
		}
		if key == "crontab" {
			var crontab string
			if err := json.Unmarshal(raw, &crontab); err != nil {
				return fmt.Errorf("crontab: %w", err)
			}
			if strings.TrimSpace(crontab) != "" && !ValidCrontab(crontab) {
				return fmt.Errorf("crontab 格式无效: %s", crontab)
			}
		}
		if key == "plugins" {
			var incoming map[string]json.RawMessage
			if err := json.Unmarshal(raw, &incoming); err != nil {
				return fmt.Errorf("plugins: %w", err)
			}
			merged := clonePlugins(s.cfg.Plugins)
			for name, section := range incoming {
				merged[name] = section
			}
			if err := s.cfg.SetKey("plugins", merged); err != nil {
				return err
			}
			continue
		}
		if key == "magic_regex" {
			rules, err := quark.ParseRules(raw)
			if err != nil {
				return fmt.Errorf("magic_regex: %w", err)
			}
			known := map[string]bool{}
			for _, rule := range rules {
				known[rule.ID] = true
			}
			for _, task := range s.cfg.Tasks {
				if task.MagicID != "" && !known[task.MagicID] {
					return fmt.Errorf("任务《%s》仍在使用被删除的规则，请先修改该任务", task.TaskName)
				}
			}
			if err := s.cfg.SetKey("magic_regex", rules); err != nil {
				return err
			}
			continue
		}
		if err := s.cfg.SetKey(key, raw); err != nil {
			return err
		}
	}
	return s.cfg.Save()
}

func (s *Store) MutateTasks(fn func([]*quark.Task) ([]*quark.Task, error)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	tasks := make([]*quark.Task, 0, len(s.cfg.Tasks))
	for _, task := range s.cfg.Tasks {
		tasks = append(tasks, cloneTask(task))
	}
	next, err := fn(tasks)
	if err != nil {
		return err
	}
	if err := s.cfg.ReplaceTasks(next); err != nil {
		return err
	}
	return s.cfg.Save()
}

// ApplyRunResults persists bans, merged plugin defaults and one-shot
// completions after a real run.
func (s *Store) ApplyRunResults(bans map[int]string, additions map[int]json.RawMessage, completed map[int]bool) error {
	if len(bans) == 0 && len(additions) == 0 && len(completed) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for index, ban := range bans {
		if index >= 0 && index < len(s.cfg.Tasks) {
			s.cfg.Tasks[index].ShareURLBan = ban
		}
	}
	for index, addition := range additions {
		if index >= 0 && index < len(s.cfg.Tasks) {
			s.cfg.Tasks[index].Addition = addition
		}
	}
	doneAt := time.Now().Format(time.RFC3339)
	for index, done := range completed {
		if !done || index < 0 || index >= len(s.cfg.Tasks) {
			continue
		}
		task := s.cfg.Tasks[index]
		if task.IsOnce() && task.OnceDoneAt == "" {
			task.OnceDoneAt = doneAt
		}
	}
	return s.cfg.Save()
}

// RenameMagic changes only the display name of a rule. Tasks reference the
// stable id, so nothing else has to be rewritten.
func (s *Store) RenameMagic(id, name string) error {
	if strings.TrimSpace(name) == "" {
		return fmt.Errorf("规则名称不能为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	rules := append([]quark.Rule(nil), s.cfg.MagicRegex...)
	found := false
	for index, rule := range rules {
		if rule.ID != id {
			continue
		}
		found = true
		rules[index].Name = name
	}
	if !found {
		return fmt.Errorf("规则不存在: %s", id)
	}
	for _, rule := range rules {
		if rule.Name == name && rule.ID != id {
			return fmt.Errorf("已存在同名规则: %s", name)
		}
	}
	if err := s.cfg.SetKey("magic_regex", rules); err != nil {
		return err
	}
	return s.cfg.Save()
}

// UpdateSourceToken persists a refreshed CloudSaver token.
func (s *Store) UpdateSourceToken(token string) error {
	if token == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var sourceCfg map[string]any
	if raw := s.cfg.Raw("source"); len(raw) > 0 {
		_ = json.Unmarshal(raw, &sourceCfg)
	}
	if sourceCfg == nil {
		sourceCfg = defaultSources()
	}
	cloudsaver, _ := sourceCfg["cloudsaver"].(map[string]any)
	if cloudsaver == nil {
		cloudsaver = map[string]any{}
		sourceCfg["cloudsaver"] = cloudsaver
	}
	cloudsaver["token"] = token
	if err := s.cfg.SetKey("source", sourceCfg); err != nil {
		return err
	}
	return s.cfg.Save()
}
