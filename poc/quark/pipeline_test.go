package quark

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"testing"
)

func newPipelineTask(savepath string) *Task {
	var task Task
	raw := fmt.Sprintf(`{"taskname":"测试","shareurl":"https://pan.quark.cn/s/abc","savepath":%q}`, savepath)
	if err := task.UnmarshalJSON([]byte(raw)); err != nil {
		panic(err)
	}
	return &task
}

func newFakeRunner(api *fakeAPI) (*Runner, *bytes.Buffer) {
	var out bytes.Buffer
	runner := NewRunner(api, nil)
	runner.DryRun = false
	runner.Out = &out
	return runner, &out
}

func TestDoSaveTaskBansInvalidShare(t *testing.T) {
	api := &fakeAPI{stokenErr: &APIError{Op: "share token", Status: 400, Code: 1, Message: "分享已失效"}}
	runner, out := newFakeRunner(api)
	task := newPipelineTask("/tv")
	tree, err := runner.DoSaveTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if tree != nil {
		t.Fatalf("tree = %#v, want nil", tree)
	}
	if task.ShareURLBan != "分享已失效" {
		t.Fatalf("ban = %q", task.ShareURLBan)
	}
	if !strings.Contains(out.String(), "❌《测试》：分享已失效") {
		t.Fatalf("missing failure notice:\n%s", out.String())
	}
}

func TestDoSaveTaskSkipsOnNetworkError(t *testing.T) {
	api := &fakeAPI{stokenErr: &APIError{Op: "http", Status: 500, Code: 1, Message: "request error"}}
	runner, out := newFakeRunner(api)
	task := newPipelineTask("/tv")
	if _, err := runner.DoSaveTask(task); err != nil {
		t.Fatal(err)
	}
	if task.ShareURLBan != "" {
		t.Fatalf("network errors must not ban the share, got %q", task.ShareURLBan)
	}
	if !strings.Contains(out.String(), "跳过任务：网络异常") {
		t.Fatalf("missing skip log:\n%s", out.String())
	}
}

func TestDoSaveTaskEmptyShare(t *testing.T) {
	api := &fakeAPI{share: map[string][]ShareFile{}, dest: map[string][]ShareFile{"/tv": {}}}
	runner, out := newFakeRunner(api)
	task := newPipelineTask("/tv")
	if _, err := runner.DoSaveTask(task); err != nil {
		t.Fatal(err)
	}
	if task.ShareURLBan != "分享为空，文件已被分享者删除" {
		t.Fatalf("ban = %q", task.ShareURLBan)
	}
	if !strings.Contains(out.String(), "分享为空") {
		t.Fatalf("missing empty-share log:\n%s", out.String())
	}
}

func TestDryRunPlansWithoutWriting(t *testing.T) {
	api := &fakeAPI{
		share: map[string][]ShareFile{"0": {{FID: "f1", ShareFIDToken: "t1", FileName: "Show.S01E02.mkv", ObjCategory: "video"}}},
		dest:  map[string][]ShareFile{"/tv": {}},
	}
	runner, out := newFakeRunner(api)
	runner.DryRun = true
	task := newPipelineTask("/tv")
	task.Pattern = `.*\.(mp4|mkv)`
	task.Replace = `{TASKNAME}.{SXX}E{E}.{EXT}`
	tree, err := runner.DoSaveTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if tree == nil || tree.Size() != 1 {
		t.Fatalf("tree = %#v", tree)
	}
	child := tree.Root().Children[0]
	if child.Data.FileNameRe != "测试.S01E02.mkv" || !child.Data.Planned || child.Data.FID != "" {
		t.Fatalf("planned node = %#v", child.Data)
	}
	if api.saves != 0 || len(api.renames) != 0 {
		t.Fatalf("dry-run wrote: saves=%d renames=%v", api.saves, api.renames)
	}
	if !strings.Contains(out.String(), "（演练）将转存 1 项") {
		t.Fatalf("missing dry-run log:\n%s", out.String())
	}
}

func TestSkipsExistingFile(t *testing.T) {
	api := &fakeAPI{
		share: map[string][]ShareFile{"0": {{FID: "f1", FileName: "Show.S01E02.mkv"}}},
		dest:  map[string][]ShareFile{"/tv": {{FID: "d1", FileName: "Show.S01E02.mkv"}}},
	}
	runner, _ := newFakeRunner(api)
	task := newPipelineTask("/tv")
	task.Pattern = `.*`
	task.Replace = `{TASKNAME}.{EXT}`
	tree, err := runner.DoSaveTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if tree != nil || api.saves != 0 {
		t.Fatalf("existing file was saved again: tree=%v saves=%d", tree, api.saves)
	}
}

func TestSaveFailureIsReported(t *testing.T) {
	api := &fakeAPI{
		share:   map[string][]ShareFile{"0": {{FID: "f1", ShareFIDToken: "t1", FileName: "a.mkv"}}},
		dest:    map[string][]ShareFile{"/tv": {}},
		saveErr: &APIError{Op: "save", Status: 400, Code: 1, Message: "空间不足"},
	}
	runner, out := newFakeRunner(api)
	task := newPipelineTask("/tv")
	tree, err := runner.DoSaveTask(task)
	if err != nil {
		t.Fatal(err)
	}
	if tree != nil {
		t.Fatalf("tree = %#v, want nil when no top fids came back", tree)
	}
	if !strings.Contains(out.String(), "转存失败：空间不足") {
		t.Fatalf("missing save failure log:\n%s", out.String())
	}
}

func TestUpdateSavepathFID(t *testing.T) {
	api := &fakeAPI{dest: map[string][]ShareFile{}}
	runner := NewRunner(api, nil)
	runner.Out = io.Discard
	task := newPipelineTask("/tv/new")
	if err := runner.UpdateSavepathFID([]*Task{task}); err != nil {
		t.Fatal(err)
	}
	if len(api.mkdirs) != 0 {
		t.Fatalf("dry-run created directories: %v", api.mkdirs)
	}
	runner.CreateDirs = true
	if err := runner.UpdateSavepathFID([]*Task{task}); err != nil {
		t.Fatal(err)
	}
	if len(api.mkdirs) != 1 || api.mkdirs[0] != "/tv/new" {
		t.Fatalf("mkdirs = %v", api.mkdirs)
	}
	if runner.savepathFID["/tv/new"] == "" {
		t.Fatalf("savepath fid not cached: %v", runner.savepathFID)
	}
}
