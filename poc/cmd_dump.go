package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"qas-poc/quark"
)

type dumpTask struct {
	TaskName               string `json:"taskname"`
	ShareURL               string `json:"shareurl"`
	SavePath               string `json:"savepath"`
	Pattern                string `json:"pattern,omitempty"`
	Replace                string `json:"replace,omitempty"`
	UpdateSubdir           string `json:"update_subdir,omitempty"`
	UpdateSubdirResaveMode bool   `json:"update_subdir_resave_mode,omitempty"`
	IgnoreExtension        bool   `json:"ignore_extension,omitempty"`
	StartFID               string `json:"startfid,omitempty"`
	EndDate                string `json:"enddate,omitempty"`
	RunWeek                []int  `json:"runweek,omitempty"`
}

type dumpFixture struct {
	Notes      string                       `json:"notes"`
	CapturedAt string                       `json:"captured_at"`
	MagicRegex []quark.Rule                 `json:"magic_regex"`
	Tasks      []dumpTask                   `json:"tasks"`
	Share      map[string][]quark.ShareFile `json:"share"`
	Dest       map[string][]quark.ShareFile `json:"dest"`
}

func cmdDump(args []string) {
	var common commonFlags
	fs := flag.NewFlagSet("dump", flag.ExitOnError)
	common.bind(fs)
	tasksSpec := fs.String("tasks", "", "任务序号或名称片段，逗号分隔；空为全部")
	out := fs.String("out", "quark/testdata/real_tasks.json", "夹具输出路径")
	depth := fs.Int("depth", 3, "分享目录递归深度")
	fs.Parse(args)

	configPath := mustConfigPath(common.config)
	cfg, err := quark.LoadConfig(configPath)
	if err != nil {
		fatalf("读取配置失败: %v", err)
	}
	tasks, err := selectTasks(cfg.Tasks, *tasksSpec)
	if err != nil {
		fatalf("%v", err)
	}
	client := common.client()

	fixture := dumpFixture{
		Notes:      "生成方式：qaspoc dump（真实分享与目标目录快照，不含 Cookie）",
		CapturedAt: time.Now().Format(time.RFC3339),
		MagicRegex: cfg.MagicRegex,
		Share:      map[string][]quark.ShareFile{},
		Dest:       map[string][]quark.ShareFile{},
	}
	destPaths := map[string]bool{}

	for _, task := range tasks {
		ref, err := quark.ExtractShareURL(task.ShareURL)
		if err != nil {
			fatalf("解析分享链接失败: %v", err)
		}
		stoken, err := client.GetStoken(ref.PwdID, ref.Passcode)
		if err != nil {
			fatalf("获取 stoken 失败（%s）: %v", task.TaskName, err)
		}
		chains, err := walkShare(client, ref.PwdID, stoken.Stoken, ref.PdirFID, *depth, fixture.Share)
		if err != nil {
			fatalf("读取分享失败（%s）: %v", task.TaskName, err)
		}
		base := quark.NormalizePath(task.SavePath)
		destPaths[base] = true
		for _, chain := range chains {
			destPaths[base+"/"+chain] = true
		}
		fixture.Tasks = append(fixture.Tasks, dumpTask{
			TaskName:               task.TaskName,
			ShareURL:               task.ShareURL,
			SavePath:               task.SavePath,
			Pattern:                task.Pattern,
			Replace:                task.Replace,
			UpdateSubdir:           task.UpdateSubdir,
			UpdateSubdirResaveMode: task.UpdateSubdirResaveMode,
			IgnoreExtension:        task.IgnoreExtension,
			StartFID:               task.StartFID,
			EndDate:                task.EndDate,
			RunWeek:                task.RunWeek,
		})
	}

	for path := range destPaths {
		fids, err := client.GetFids([]string{path})
		if err != nil {
			fatalf("解析目标目录失败（%s）: %v", path, err)
		}
		if len(fids) == 0 {
			fixture.Dest[path] = []quark.ShareFile{}
			continue
		}
		list, err := client.LsDir(fids[0].FID)
		if err != nil {
			fatalf("读取目标目录失败（%s）: %v", path, err)
		}
		if list == nil {
			list = []quark.ShareFile{}
		}
		fixture.Dest[path] = list
	}

	for _, files := range fixture.Share {
		for i := range files {
			files[i].ShareFIDToken = ""
		}
	}
	for _, files := range fixture.Dest {
		for i := range files {
			files[i].ShareFIDToken = ""
		}
	}
	encoded, err := json.MarshalIndent(fixture, "", "  ")
	if err != nil {
		fatalf("编码夹具失败: %v", err)
	}
	if dir := filepath.Dir(*out); dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			fatalf("创建目录失败: %v", err)
		}
	}
	if err := os.WriteFile(*out, append(encoded, '\n'), 0o644); err != nil {
		fatalf("写入夹具失败: %v", err)
	}
	fmt.Printf("已写入 %s：任务 %d 个，分享目录 %d 个，目标目录 %d 个\n",
		*out, len(fixture.Tasks), len(fixture.Share), len(fixture.Dest))
}

// walkShare records every share directory (up to depth) and returns the
// relative name chains of its subdirectories, which the pipeline may use as
// savepath suffixes.
func walkShare(client *quark.Client, pwdID, stoken, pdirFID string, depth int, out map[string][]quark.ShareFile) ([]string, error) {
	detail, err := client.GetDetail(pwdID, stoken, pdirFID, 0, 0)
	if err != nil {
		return nil, err
	}
	if detail.List == nil {
		detail.List = []quark.ShareFile{}
	}
	out[pdirFID] = detail.List
	var chains []string
	if depth <= 0 {
		return chains, nil
	}
	for _, file := range detail.List {
		if !file.Dir {
			continue
		}
		nested, err := walkShare(client, pwdID, stoken, file.FID, depth-1, out)
		if err != nil {
			return nil, err
		}
		chains = append(chains, file.FileName)
		for _, chain := range nested {
			chains = append(chains, file.FileName+"/"+chain)
		}
	}
	return chains, nil
}
