package plugins

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path"
	"regexp"
	"strings"

	"qas-poc/quark"
)

type alistSyncConfig struct {
	URL            string `json:"url"`
	Token          string `json:"token"`
	QuarkStorageID string `json:"quark_storage_id"`
	SaveStorageID  string `json:"save_storage_id"`
	TVMode         string `json:"tv_mode"`
}

type alistSyncTaskConfig struct {
	Enable       bool   `json:"enable"`
	SavePath     string `json:"save_path"`
	VerifyPath   string `json:"verify_path"`
	FullPathMode bool   `json:"full_path_mode"`
}

// AlistSync copies saved files to another AList storage.
type AlistSync struct {
	deps     Deps
	cfg      alistSyncConfig
	client   *alistClient
	active   bool
	tvMode   bool
	taskname string
}

func NewAlistSync(section json.RawMessage, deps Deps) *AlistSync {
	plugin := &AlistSync{deps: deps}
	if len(section) == 0 {
		return plugin
	}
	if err := decodeTaskConfig(section, &plugin.cfg); err != nil {
		deps.logf("alist_sync: 配置解析失败 %v", err)
		return plugin
	}
	if plugin.cfg.URL == "" || plugin.cfg.Token == "" {
		return plugin
	}
	plugin.client = newAlistClient(plugin.cfg.URL, plugin.cfg.Token, deps)
	if plugin.verifyServer() {
		plugin.active = true
	}
	return plugin
}

func (p *AlistSync) Name() string { return "alist_sync" }
func (p *AlistSync) Active() bool { return p.active }

func (p *AlistSync) TaskDefaults() map[string]any {
	return map[string]any{
		"enable":         false,
		"save_path":      "",
		"verify_path":    "",
		"full_path_mode": false,
	}
}

func (p *AlistSync) verifyServer() bool {
	status, body, err := p.client.get("/api/me", nil, p.client.headers())
	if err != nil {
		p.deps.logf("获取Alist信息出错: %v", err)
		return false
	}
	if status >= 400 {
		p.deps.logf("Alist同步: 连接服务器失败❌ status %d", status)
		return false
	}
	response, err := decodeAlist(body)
	if err != nil {
		p.deps.logf("获取Alist信息出错: %v", err)
		return false
	}
	if response.Code != 200 {
		p.deps.logf("Alist同步: 连接服务器失败❌ %s", response.Message)
		return false
	}
	var data struct {
		Username string `json:"username"`
	}
	if err := json.Unmarshal(response.Data, &data); err != nil {
		p.deps.logf("获取Alist信息出错: %v", err)
		return false
	}
	if data.Username == "guest" {
		p.deps.logf("Alist登陆失败，请检查token")
		return false
	}
	p.deps.logf("Alist登陆成功，当前用户: %s", data.Username)
	return true
}

func (p *AlistSync) Run(task *quark.Task, env *quark.PluginEnv) error {
	config := alistSyncTaskConfig{}
	if err := decodeTaskConfig(task.AdditionSection(p.Name()), &config); err != nil {
		return fmt.Errorf("task config: %w", err)
	}
	if !config.Enable {
		return nil
	}
	p.deps.logf("开始进行alist同步")
	p.taskname = task.TaskName
	p.tvMode = p.cfg.TVMode != "" && p.cfg.TVMode != "0"

	quarkStorage, err := p.client.getStorage(p.cfg.QuarkStorageID)
	if err != nil {
		p.deps.logf("Alist同步: 获取夸克存储失败❌ %v", err)
		return nil
	}
	if quarkStorage.Driver != "Quark" {
		p.deps.logf("Alist同步: 存储%s非夸克存储❌ %s", p.cfg.QuarkStorageID, quarkStorage.Driver)
		return nil
	}
	var addition struct {
		RootFolderID string `json:"root_folder_id"`
	}
	_ = json.Unmarshal([]byte(quarkStorage.Addition), &addition)
	if addition.RootFolderID != "0" && addition.RootFolderID != "" {
		p.deps.logf("Alist同步: 存储%s挂载的目录非夸克根目录❌", p.cfg.QuarkStorageID)
		return nil
	}
	quarkMount := quarkStorage.MountPath

	if p.cfg.SaveStorageID == "" {
		p.deps.logf("Alist同步: 未配置 save_storage_id，跳过")
		return nil
	}
	saveStorage, err := p.client.getStorage(p.cfg.SaveStorageID)
	if err != nil {
		p.deps.logf("Alist同步: 获取保存存储失败❌ %v", err)
		return nil
	}
	saveMount := saveStorage.MountPath

	savePath := p.buildPath(saveMount, config.SavePath, config.FullPathMode)
	if savePath == "" {
		savePath = path.Join(saveMount, task.SavePath)
	}
	verifyPath := savePath
	if config.VerifyPath != "" {
		verifyPath = p.buildPath(saveMount, config.VerifyPath, config.FullPathMode)
	}
	sourcePath := path.Join(quarkMount, task.SavePath)
	dirExists := p.pathExists(savePath)

	sourceList, ok := p.pathList(sourcePath)
	if !ok {
		p.deps.logf("获取夸克文件列表失败，请检查网络或手动刷新alist中的夸克目录")
		return nil
	}
	var files []string
	if !dirExists {
		files = p.pickFiles(nil, sourceList)
	} else if verifyList, ok := p.pathList(verifyPath); ok && len(verifyList) > 0 {
		files = p.pickFiles(verifyList, sourceList)
	} else {
		files = p.pickFiles(nil, sourceList)
	}

	if len(files) == 0 {
		p.deps.logf("没有需要同步的文件")
		return nil
	}
	p.copyFiles(sourcePath, savePath, files)
	p.deps.logf("同步的文件列表：")
	for _, name := range files {
		p.deps.logf("└── 🎞️%s", name)
	}
	return nil
}

// buildPath turns a configured path into an AList absolute path.
func (p *AlistSync) buildPath(mount, configured string, fullPathMode bool) string {
	if configured == "" {
		return ""
	}
	if fullPathMode {
		return "/" + strings.Trim(configured, "/")
	}
	return path.Join(mount, configured)
}

func (p *AlistSync) pathExists(alistPath string) bool {
	status, body, err := p.client.postJSON("/api/fs/list", map[string]any{
		"path":       alistPath,
		"password":   "",
		"force_root": false,
	}, p.client.headers())
	if err != nil || status != http.StatusOK {
		return false
	}
	response, err := decodeAlist(body)
	if err != nil {
		return false
	}
	return response.Message == "success"
}

func (p *AlistSync) pathList(alistPath string) ([]alistFile, bool) {
	files, response, err := p.client.fsList(alistPath, true)
	if err != nil || response.Code != 200 {
		return nil, false
	}
	return files, true
}

func (p *AlistSync) pickFiles(targets, sources []alistFile) []string {
	episode := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(p.taskname) + `\.s\d{1,3}e\d{1,3}\.(mkv|mp4)`)
	mp4 := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(p.taskname) + `\.s\d{1,3}e\d{1,3}\.mp4`)
	var picked []string
	if len(targets) == 0 {
		for _, source := range sources {
			if p.tvMode && !episode.MatchString(source.Name) {
				continue
			}
			picked = append(picked, source.Name)
		}
		return picked
	}
	for _, source := range sources {
		skip := false
		sourceName := normalizeSyncName(source.Name, p.taskname)
		for _, target := range targets {
			if source.IsDir {
				skip = true
				break
			}
			if p.tvMode {
				if sourceName == normalizeSyncName(target.Name, p.taskname) {
					skip = true
					break
				}
				if episode.MatchString(source.Name) && mp4.MatchString(source.Name) {
					mkvName := strings.ReplaceAll(source.Name, ".mp4", ".mkv")
					for _, other := range sources {
						if mkvName == other.Name {
							p.deps.logf("%s拥有相同版本的MKV文件，跳过复制", source.Name)
							skip = true
						}
					}
				}
			} else if source.Name == target.Name {
				skip = true
				break
			}
		}
		if !skip {
			picked = append(picked, source.Name)
		}
	}
	return picked
}

func normalizeSyncName(name, taskname string) string {
	name = strings.ReplaceAll(name, ".mp4", "")
	name = strings.ReplaceAll(name, ".mkv", "")
	name = strings.ReplaceAll(name, taskname+".", "")
	return strings.ToLower(name)
}

func (p *AlistSync) copyFiles(src, dst string, names []string) {
	status, _, err := p.client.postJSON("/api/fs/copy", map[string]any{
		"src_dir": src,
		"dst_dir": dst,
		"names":   names,
	}, p.client.headers())
	if err != nil {
		p.deps.logf("未能进行Alist同步，请手动同步: %v", err)
		return
	}
	if status != http.StatusOK {
		p.deps.logf("未能进行Alist同步，请手动同步: status %d", status)
		return
	}
	p.deps.logf("Alist创建任务成功")
}
