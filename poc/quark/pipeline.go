package quark

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

// API is the slice of the Quark client the save pipeline needs.
type API interface {
	GetStoken(pwdID, passcode string) (Stoken, error)
	GetDetail(pwdID, stoken, pdirFID string, fetchShare, fetchFullPath int) (Detail, error)
	GetFids(paths []string) ([]PathFID, error)
	Mkdir(dirPath string) (string, error)
	SaveFile(fidList, fidTokenList []string, toPdirFID, pwdID, stoken string) (string, error)
	QueryTask(taskID string) ([]string, error)
	LsDir(pdirFID string) ([]ShareFile, error)
	Rename(fid, fileName string) error
	Delete(fids []string) (string, error)
	RecycleList() ([]RecycleItem, error)
	RecycleRemove(recordIDs []string) error
}

type NodeData struct {
	IsDir       bool
	Size        int64
	FileName    string
	FileNameRe  string
	FID         string
	Path        string
	ObjCategory string
	Planned     bool
}

type TreeNode struct {
	ID       string
	Name     string
	Data     NodeData
	Children []*TreeNode
}

type Tree struct {
	root *TreeNode
}

func NewTree() *Tree { return &Tree{root: &TreeNode{}} }

func (t *Tree) Root() *TreeNode { return t.root }

func (t *Tree) SetRoot(node *TreeNode) { t.root = node }

func (t *Tree) Size() int {
	var walk func(*TreeNode) int
	walk = func(node *TreeNode) int {
		n := len(node.Children)
		for _, child := range node.Children {
			n += walk(child)
		}
		return n
	}
	return walk(t.root)
}

func (t *Tree) Add(parent *TreeNode, name, id string, data NodeData) *TreeNode {
	node := &TreeNode{ID: id, Name: name, Data: data}
	parent.Children = append(parent.Children, node)
	return node
}

func (t *Tree) Print(w io.Writer) {
	fmt.Fprintln(w, t.root.Name)
	printChildren(w, t.root, "")
}

// Walk visits the root first, then children depth-first.
func (t *Tree) Walk(fn func(node *TreeNode, isRoot bool)) {
	var walk func(*TreeNode, bool)
	walk = func(node *TreeNode, isRoot bool) {
		fn(node, isRoot)
		for _, child := range node.Children {
			walk(child, false)
		}
	}
	walk(t.root, true)
}

func (t *Tree) AllNodes() []*TreeNode {
	var nodes []*TreeNode
	t.Walk(func(node *TreeNode, isRoot bool) {
		nodes = append(nodes, node)
	})
	return nodes
}

func printChildren(w io.Writer, node *TreeNode, prefix string) {
	for i, child := range node.Children {
		last := i == len(node.Children)-1
		branch := "├── "
		next := prefix + "│   "
		if last {
			branch = "└── "
			next = prefix + "    "
		}
		fmt.Fprintln(w, prefix+branch+child.Name)
		printChildren(w, child, next)
	}
}

type Runner struct {
	api           API
	rules         []Rule
	DryRun        bool
	CreateDirs    bool
	Out           io.Writer
	Now           func() time.Time
	SaveBatch     int
	savepathFID   map[string]string
	Notifications []string
}

func NewRunner(api API, rules []Rule) *Runner {
	return &Runner{
		api:         api,
		rules:       rules,
		DryRun:      true,
		Out:         os.Stdout,
		Now:         time.Now,
		SaveBatch:   100,
		savepathFID: map[string]string{"/": "0"},
	}
}

// SavepathFID exposes the destination cache to plugins; callers may mutate it.
func (r *Runner) SavepathFID() map[string]string { return r.savepathFID }

func (r *Runner) logf(format string, args ...any) {
	fmt.Fprintf(r.Out, format+"\n", args...)
}

func (r *Runner) notify(text string) {
	r.Notifications = append(r.Notifications, text)
	fmt.Fprintln(r.Out, "📢", text)
}

// UpdateSavepathFID resolves (and, unless in dry-run, creates) every
// destination directory once so tasks do not repeat the lookups.
func (r *Runner) UpdateSavepathFID(tasks []*Task) error {
	now := r.Now()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	var paths []string
	seen := map[string]bool{}
	for _, task := range tasks {
		// One-shot tasks are run by hand, so a leftover end date must not
		// keep their destination directory from being created.
		if task.EndDate != "" && !task.IsOnce() {
			end, err := time.ParseInLocation("2006-01-02", task.EndDate, now.Location())
			if err != nil || today.After(end) {
				continue
			}
		}
		path := normalizePath(task.SavePath)
		if !seen[path] {
			seen[path] = true
			paths = append(paths, path)
		}
	}
	if len(paths) == 0 {
		return nil
	}
	fids, err := r.api.GetFids(paths)
	if err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, item := range fids {
		r.savepathFID[item.FilePath] = item.FID
		existing[item.FilePath] = true
	}
	for _, path := range paths {
		if path == "/" || existing[path] {
			continue
		}
		if !r.CreateDirs {
			r.logf("（演练）将创建目录 %s", path)
			continue
		}
		fid, err := r.api.Mkdir(path)
		if err != nil {
			r.logf("创建文件夹：%s 失败, %v", path, err)
			continue
		}
		r.savepathFID[path] = fid
		r.logf("创建文件夹：%s", path)
	}
	return nil
}

// DoSaveTask mirrors Quark.do_save_task: validate the share, diff the
// destination, transfer missing files and rename them.
func (r *Runner) DoSaveTask(task *Task) (*Tree, error) {
	if task.ShareURLBan != "" {
		r.logf("《%s》：%s", task.TaskName, task.ShareURLBan)
		return nil, nil
	}
	ref, err := ExtractShareURL(task.ShareURL)
	if err != nil {
		return nil, err
	}
	stoken, err := r.api.GetStoken(ref.PwdID, ref.Passcode)
	if err != nil {
		var apiErr *APIError
		if errors.As(err, &apiErr) && apiErr.Status == 500 {
			r.logf("跳过任务：网络异常 %s", apiErr.Message)
			return nil, nil
		}
		message := errorMessage(err)
		r.notify(fmt.Sprintf("❌《%s》：%s", task.TaskName, message))
		task.ShareURLBan = message
		return nil, nil
	}
	tree, err := r.dirCheckAndSave(task, ref.PwdID, stoken.Stoken, ref.PdirFID, "")
	if err != nil {
		return nil, err
	}
	if tree.Size() > 0 {
		r.doRename(tree)
		r.logf("")
		var buf strings.Builder
		tree.Print(&buf)
		r.notify(fmt.Sprintf("✅《%s》添加追更：\n%s", task.TaskName, strings.TrimRight(buf.String(), "\n")))
		return tree, nil
	}
	r.logf("任务结束：没有新的转存任务")
	return nil, nil
}

func (r *Runner) dirCheckAndSave(task *Task, pwdID, stoken, pdirFID, subdirPath string) (*Tree, error) {
	tree := NewTree()
	detail, err := r.api.GetDetail(pwdID, stoken, pdirFID, 0, 0)
	if err != nil {
		return nil, err
	}
	shareFiles := detail.List
	if len(shareFiles) == 0 {
		if subdirPath == "" {
			task.ShareURLBan = "分享为空，文件已被分享者删除"
			r.notify(fmt.Sprintf("❌《%s》：%s", task.TaskName, task.ShareURLBan))
		}
		return tree, nil
	}
	if len(shareFiles) == 1 && shareFiles[0].Dir && subdirPath == "" {
		r.logf("🧠 该分享是一个文件夹，读取文件夹内列表")
		nested, err := r.api.GetDetail(pwdID, stoken, shareFiles[0].FID, 0, 0)
		if err != nil {
			return nil, err
		}
		shareFiles = nested.List
	}

	savepath := normalizePath(task.SavePath + subdirPath)
	toPdirFID, known := r.savepathFID[savepath]
	if !known {
		fids, err := r.api.GetFids([]string{savepath})
		if err != nil {
			return nil, err
		}
		if len(fids) == 0 {
			if !r.DryRun {
				r.logf("❌ 目录 %s fid获取失败，跳过转存", savepath)
				return tree, nil
			}
			r.logf("（演练）目录 %s 不存在，按空目录计算", savepath)
		} else {
			toPdirFID = fids[0].FID
			r.savepathFID[savepath] = toPdirFID
		}
	}
	var dirFiles []ShareFile
	if toPdirFID != "" {
		dirFiles, err = r.api.LsDir(toPdirFID)
		if err != nil {
			return nil, err
		}
	}
	dirNames := make([]string, len(dirFiles))
	for i, file := range dirFiles {
		dirNames[i] = file.FileName
	}

	tree.SetRoot(&TreeNode{ID: pdirFID, Name: savepath, Data: NodeData{IsDir: true}})
	root := tree.Root()

	magic := NewMagic(r.rules)
	magic.SetTaskName(task.TaskName)
	magic.SetClock(r.Now)
	pattern, replace := magic.ResolveTask(task.MagicID, task.Pattern, task.Replace)

	var needSave []*ShareFile
	for _, item := range shareFiles {
		shareFile := item
		searchPattern := pattern
		if shareFile.Dir && task.UpdateSubdir != "" {
			searchPattern = task.UpdateSubdir
		}
		matched, err := magic.Match(searchPattern, shareFile.FileName)
		if err != nil {
			return nil, fmt.Errorf("pattern %q: %w", searchPattern, err)
		}
		if matched {
			existing := magic.Exists(shareFile.FileName, dirNames, task.IgnoreExtension && !shareFile.Dir)
			if existing == "" {
				if shareFile.Dir || subdirPath != "" {
					shareFile.FileNameRe = shareFile.FileName
					needSave = append(needSave, &shareFile)
				} else {
					nameRe, err := magic.Sub(pattern, replace, shareFile.FileName)
					if err != nil {
						return nil, err
					}
					if magic.Exists(nameRe, dirNames, task.IgnoreExtension) == "" {
						shareFile.FileNameRe = nameRe
						needSave = append(needSave, &shareFile)
					}
				}
			} else if shareFile.Dir && task.UpdateSubdir != "" {
				if yes, err := magic.Match(task.UpdateSubdir, shareFile.FileName); err != nil {
					return nil, err
				} else if yes {
					if task.UpdateSubdirResaveMode {
						r.logf("重存子目录：%s/%s", savepath, shareFile.FileName)
						if err := r.resaveSubdir(dirFiles, existing); err != nil {
							return nil, err
						}
						shareFile.FileNameRe = shareFile.FileName
						needSave = append(needSave, &shareFile)
					} else {
						r.logf("检查子目录：%s/%s", savepath, shareFile.FileName)
						subTree, err := r.dirCheckAndSave(task, pwdID, stoken, shareFile.FID, subdirPath+"/"+shareFile.FileName)
						if err != nil {
							return nil, err
						}
						if subTree.Size() > 0 {
							tree.Add(root, "📁"+shareFile.FileName, shareFile.FID, NodeData{IsDir: true})
							last := root.Children[len(root.Children)-1]
							last.Children = append(last.Children, subTree.root.Children...)
						}
					}
				}
			}
		}
		if task.StartFID != "" && shareFile.FID == task.StartFID {
			break
		}
	}

	if reIPlus.MatchString(replace) {
		if err := magic.SetDirFileList(dirFiles, replace); err != nil {
			return nil, err
		}
		magic.SortFileList(needSave)
	}

	if err := r.saveBatch(task, needSave, toPdirFID, pwdID, stoken, savepath, root); err != nil {
		return nil, err
	}
	return tree, nil
}

func (r *Runner) resaveSubdir(dirFiles []ShareFile, existingName string) error {
	var subdir *ShareFile
	for i := range dirFiles {
		if dirFiles[i].FileName == existingName {
			subdir = &dirFiles[i]
			break
		}
	}
	if subdir == nil {
		return fmt.Errorf("找不到子目录 %q 的 fid", existingName)
	}
	if r.DryRun {
		r.logf("（演练）将删除并重存 %s", existingName)
		return nil
	}
	taskID, err := r.api.Delete([]string{subdir.FID})
	if err != nil {
		return fmt.Errorf("删除子目录失败: %w", err)
	}
	if _, err := r.api.QueryTask(taskID); err != nil {
		return fmt.Errorf("等待删除完成失败: %w", err)
	}
	recycle, err := r.api.RecycleList()
	if err != nil {
		return fmt.Errorf("读取回收站失败: %w", err)
	}
	var records []string
	for _, item := range recycle {
		if item.FID == subdir.FID {
			records = append(records, item.RecordID)
		}
	}
	if len(records) > 0 {
		if err := r.api.RecycleRemove(records); err != nil {
			return fmt.Errorf("清理回收站失败: %w", err)
		}
	}
	return nil
}

func (r *Runner) saveBatch(task *Task, needSave []*ShareFile, toPdirFID, pwdID, stoken, savepath string, root *TreeNode) error {
	if len(needSave) == 0 {
		return nil
	}
	batchSize := r.SaveBatch
	if batchSize <= 0 {
		batchSize = 100
	}
	var topFIDs []string
	var errMsg string
	remaining := needSave
	for len(remaining) > 0 {
		n := min(batchSize, len(remaining))
		batch := remaining[:n]
		remaining = remaining[n:]
		if r.DryRun {
			r.logf("（演练）将转存 %d 项到 %s", len(batch), savepath)
			continue
		}
		fids := make([]string, len(batch))
		tokens := make([]string, len(batch))
		for i, item := range batch {
			fids[i] = item.FID
			tokens[i] = item.ShareFIDToken
		}
		taskID, err := r.api.SaveFile(fids, tokens, toPdirFID, pwdID, stoken)
		if err != nil {
			errMsg = errorMessage(err)
		} else if got, err := r.api.QueryTask(taskID); err != nil {
			errMsg = errorMessage(err)
		} else {
			topFIDs = append(topFIDs, got...)
		}
		if errMsg != "" {
			r.notify(fmt.Sprintf("❌《%s》转存失败：%s", task.TaskName, errMsg))
		}
	}
	if !r.DryRun && len(needSave) != len(topFIDs) {
		// The task result does not always report one fid per saved item.
		// Matching names in the destination keeps the tree (rename, stats and
		// local download) usable instead of dropping the whole batch.
		r.logf("转存结果数量不符：计划 %d，返回 %d，改为按目标目录匹配", len(needSave), len(topFIDs))
		topFIDs = r.matchSavedFIDs(toPdirFID, needSave)
	}
	for index, item := range needSave {
		data := NodeData{
			Size:        item.Size,
			FileName:    item.FileName,
			FileNameRe:  item.FileNameRe,
			Path:        savepath + "/" + item.FileNameRe,
			IsDir:       item.Dir,
			ObjCategory: item.ObjCategory,
		}
		if r.DryRun {
			data.Planned = true
		} else if topFIDs[index] == "" {
			r.logf("⚠ 目标目录中找不到 %q，跳过该项的改名与本地下载", item.FileName)
			continue
		} else {
			data.FID = topFIDs[index]
		}
		r.addNode(root, iconFor(item)+item.FileNameRe, item.FID, data)
	}
	return nil
}

// matchSavedFIDs maps each saved item to the fid of the same-named entry in
// the destination directory. Items without a match keep an empty fid.
func (r *Runner) matchSavedFIDs(toPdirFID string, needSave []*ShareFile) []string {
	fids := make([]string, len(needSave))
	entries, err := r.api.LsDir(toPdirFID)
	if err != nil {
		r.logf("✗ 读取目标目录失败，无法补全转存结果：%v", err)
		return fids
	}
	byName := make(map[string]string, len(entries))
	for _, entry := range entries {
		if entry.FileName != "" && entry.FID != "" {
			byName[entry.FileName] = entry.FID
		}
	}
	matched := 0
	for index, item := range needSave {
		if fid, ok := byName[item.FileName]; ok {
			fids[index] = fid
			matched++
		}
	}
	r.logf("按目标目录匹配到 %d/%d 个转存结果", matched, len(needSave))
	return fids
}

func (r *Runner) addNode(parent *TreeNode, name, id string, data NodeData) {
	parent.Children = append(parent.Children, &TreeNode{ID: id, Name: name, Data: data})
}

func (r *Runner) doRename(tree *Tree) {
	for _, child := range tree.root.Children {
		file := child.Data
		if file.IsDir {
			continue
		}
		if file.FileNameRe == "" || file.FileNameRe == file.FileName {
			continue
		}
		r.logf("重命名：%s → %s", file.FileName, file.FileNameRe)
		if r.DryRun {
			continue
		}
		if err := r.api.Rename(file.FID, file.FileNameRe); err != nil {
			r.logf("      ↑ 失败，%v", err)
		}
	}
}

func iconFor(file *ShareFile) string {
	if file.Dir {
		return "📁"
	}
	switch file.ObjCategory {
	case "video":
		return "🎞️"
	case "image":
		return "🖼️"
	case "audio":
		return "🎵"
	case "doc":
		return "📄"
	case "archive":
		return "📦"
	default:
		return ""
	}
}

func errorMessage(err error) string {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		if apiErr.Message != "" {
			return apiErr.Message
		}
		return apiErr.Error()
	}
	return err.Error()
}
