package plugins

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"qas-poc/quark"
)

type alistStrmGenConfig struct {
	URL             string `json:"url"`
	Token           string `json:"token"`
	StorageID       string `json:"storage_id"`
	StrmSaveDir     string `json:"strm_save_dir"`
	StrmReplaceHost string `json:"strm_replace_host"`
}

var strmVideoExts = []string{"mp4", "mkv", "flv", "mov", "m4v", "avi", "webm", "wmv"}

// AlistStrmGen walks the mounted directory and writes local .strm files.
type AlistStrmGen struct {
	deps      Deps
	cfg       alistStrmGenConfig
	client    *alistClient
	active    bool
	mountPath string
	quarkRoot string
	strmHost  string
}

func NewAlistStrmGen(section json.RawMessage, deps Deps) *AlistStrmGen {
	plugin := &AlistStrmGen{deps: deps, cfg: alistStrmGenConfig{StrmSaveDir: "/media"}}
	if len(section) == 0 {
		return plugin
	}
	if err := decodeTaskConfig(section, &plugin.cfg); err != nil {
		deps.logf("Alist-Strm生成: 配置解析失败 %v", err)
		return plugin
	}
	if plugin.cfg.URL == "" || plugin.cfg.Token == "" || plugin.cfg.StorageID == "" {
		return plugin
	}
	plugin.client = newAlistClient(plugin.cfg.URL, plugin.cfg.Token, deps)
	mount, root, ok := plugin.client.storageMount(plugin.cfg.StorageID, "Alist-Strm生成")
	if !ok {
		return plugin
	}
	plugin.mountPath, plugin.quarkRoot = mount, root
	plugin.deps.logf("Alist-Strm生成: [%s:%s]", mount, root)
	replaceHost := strings.TrimSpace(plugin.cfg.StrmReplaceHost)
	switch {
	case replaceHost == "":
		plugin.strmHost = strings.TrimRight(strings.TrimSpace(plugin.cfg.URL), "/") + "/d"
	case strings.HasPrefix(replaceHost, "http"):
		plugin.strmHost = replaceHost + "/d"
	default:
		plugin.strmHost = "http://" + replaceHost + "/d"
	}
	plugin.active = true
	return plugin
}

func (p *AlistStrmGen) Name() string { return "alist_strm_gen" }
func (p *AlistStrmGen) Active() bool { return p.active }

func (p *AlistStrmGen) TaskDefaults() map[string]any {
	return map[string]any{"auto_gen": true}
}

func (p *AlistStrmGen) Run(task *quark.Task, env *quark.PluginEnv) error {
	var config struct {
		AutoGen bool `json:"auto_gen"`
	}
	config.AutoGen = true
	if err := decodeTaskConfig(task.AdditionSection(p.Name()), &config); err != nil {
		return fmt.Errorf("auto_gen: %w", err)
	}
	if !config.AutoGen {
		return nil
	}
	if task.SavePath == "" || p.quarkRoot == "" || !strings.HasPrefix(task.SavePath, p.quarkRoot) {
		return nil
	}
	if strings.TrimSpace(p.cfg.StrmSaveDir) == "" {
		p.deps.logf("📺 Alist-Strm生成: strm_save_dir 为空，跳过")
		return nil
	}
	relative := strings.Replace(task.SavePath, p.quarkRoot, "", 1)
	p.checkDir(joinAlistPath(p.mountPath, relative))
	return nil
}

func (p *AlistStrmGen) checkDir(alistPath string) {
	files, response, err := p.client.fsList(alistPath, false)
	if err != nil || response.Code != 200 {
		message := response.Message
		if err != nil {
			message = err.Error()
		}
		p.deps.logf("📺 Alist-Strm生成: 获取文件列表失败❌%s", message)
		return
	}
	for _, file := range files {
		itemPath := strings.ReplaceAll(alistPath+"/"+file.Name, "//", "/")
		if file.IsDir {
			p.checkDir(itemPath)
			continue
		}
		p.generateStrm(itemPath, file)
	}
}

func (p *AlistStrmGen) generateStrm(filePath string, file alistFile) {
	root, ext := splitExt(filePath)
	if !containsStringFold(strmVideoExts, strings.TrimPrefix(ext, ".")) {
		return
	}
	base := filepath.Clean(p.cfg.StrmSaveDir)
	strmPath := filepath.Clean(filepath.FromSlash(strings.ReplaceAll(p.cfg.StrmSaveDir+root+".strm", "//", "/")))
	if !strings.HasPrefix(strmPath, base+string(filepath.Separator)) {
		p.deps.logf("📺 Alist-Strm生成: 拒绝越界路径 %s", strmPath)
		return
	}
	if _, err := os.Stat(strmPath); err == nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(strmPath), 0o755); err != nil {
		p.deps.logf("📺 Alist-Strm生成: 创建目录失败 %v", err)
		return
	}
	sign := ""
	if file.Sign != "" {
		sign = "?sign=" + file.Sign
	}
	content := p.strmHost + filePath + sign
	if err := os.WriteFile(strmPath, []byte(content), 0o644); err != nil {
		p.deps.logf("📺 Alist-Strm生成: 写入失败 %v", err)
		return
	}
	p.deps.logf("📺 生成STRM文件 %s 成功✅", strmPath)
}

func containsStringFold(list []string, want string) bool {
	for _, item := range list {
		if strings.EqualFold(item, want) {
			return true
		}
	}
	return false
}
