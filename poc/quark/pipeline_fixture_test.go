package quark

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

type fixtureDoc struct {
	MagicRegex json.RawMessage        `json:"magic_regex"`
	Tasks      []json.RawMessage      `json:"tasks"`
	Share      map[string][]ShareFile `json:"share"`
	Dest       map[string][]ShareFile `json:"dest"`
}

type planDoc struct {
	Tasks []planTask `json:"tasks"`
}

type planTask struct {
	TaskName string       `json:"taskname"`
	Saved    []planItem   `json:"saved"`
	Renames  []planRename `json:"renames"`
	Ban      string       `json:"ban"`
}

type planItem struct {
	Path       string `json:"path"`
	FileName   string `json:"file_name"`
	FileNameRe string `json:"file_name_re"`
	Dir        bool   `json:"dir"`
}

type planRename struct {
	FID  string `json:"fid"`
	Name string `json:"name"`
}

type fakeAPI struct {
	share     map[string][]ShareFile
	dest      map[string][]ShareFile
	top       int
	renames   []planRename
	stokenErr error
	saveErr   error
	saves     int
	mkdirs    []string
	// savedFiles are appended to the destination on save so a test can model
	// what the drive looks like afterwards.
	savedFiles []ShareFile
	// shortFids truncates the task result to model a partial save_as_top_fids.
	shortFids int
}

func destFID(path string) string { return "dest:" + path }

func (f *fakeAPI) GetStoken(pwdID, passcode string) (Stoken, error) {
	if f.stokenErr != nil {
		return Stoken{}, f.stokenErr
	}
	return Stoken{Stoken: "fake", Title: "fixture"}, nil
}

func (f *fakeAPI) GetDetail(pwdID, stoken, pdirFID string, fetchShare, fetchFullPath int) (Detail, error) {
	return Detail{Title: "fixture", List: cloneFiles(f.share[pdirFID])}, nil
}

func (f *fakeAPI) GetFids(paths []string) ([]PathFID, error) {
	var out []PathFID
	for _, path := range paths {
		if _, ok := f.dest[path]; ok {
			out = append(out, PathFID{FilePath: path, FID: destFID(path)})
		}
	}
	return out, nil
}

func (f *fakeAPI) Mkdir(dirPath string) (string, error) {
	f.mkdirs = append(f.mkdirs, dirPath)
	if f.dest == nil {
		f.dest = map[string][]ShareFile{}
	}
	f.dest[dirPath] = []ShareFile{}
	return destFID(dirPath), nil
}

func (f *fakeAPI) SaveFile(fidList, fidTokenList []string, toPdirFID, pwdID, stoken string) (string, error) {
	f.saves++
	if f.saveErr != nil {
		return "", f.saveErr
	}
	if len(f.savedFiles) > 0 {
		path := strings.TrimPrefix(toPdirFID, "dest:")
		f.dest[path] = append(cloneFiles(f.dest[path]), f.savedFiles...)
	}
	return fmt.Sprintf("task-%d", len(fidList)), nil
}

func (f *fakeAPI) QueryTask(taskID string) ([]string, error) {
	var count int
	if _, err := fmt.Sscanf(taskID, "task-%d", &count); err != nil {
		return nil, err
	}
	if f.shortFids > 0 && count > f.shortFids {
		count = f.shortFids
	}
	var ids []string
	for i := 0; i < count; i++ {
		f.top++
		ids = append(ids, fmt.Sprintf("top-%04d", f.top))
	}
	return ids, nil
}

func (f *fakeAPI) LsDir(pdirFID string) ([]ShareFile, error) {
	path := strings.TrimPrefix(pdirFID, "dest:")
	return cloneFiles(f.dest[path]), nil
}

func (f *fakeAPI) Rename(fid, fileName string) error {
	f.renames = append(f.renames, planRename{FID: fid, Name: fileName})
	return nil
}

func (f *fakeAPI) Delete(fids []string) (string, error)   { return "task-0", nil }
func (f *fakeAPI) RecycleList() ([]RecycleItem, error)    { return nil, nil }
func (f *fakeAPI) RecycleRemove(recordIDs []string) error { return nil }

func cloneFiles(in []ShareFile) []ShareFile {
	if in == nil {
		return []ShareFile{}
	}
	out := make([]ShareFile, len(in))
	copy(out, in)
	return out
}

func TestPipelineMatchesPythonPlan(t *testing.T) {
	plans, err := filepath.Glob("testdata/*.plan.json")
	if err != nil || len(plans) == 0 {
		t.Fatalf("no python plans found (run poc/tools/parity_plan.py): %v", err)
	}
	for _, planPath := range plans {
		fixturePath := strings.TrimSuffix(planPath, ".plan.json") + ".json"
		t.Run(filepath.Base(fixturePath), func(t *testing.T) {
			compareFixture(t, fixturePath, planPath)
		})
	}
}

func compareFixture(t *testing.T, fixturePath, planPath string) {
	fixtureRaw, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	var fixture fixtureDoc
	if err := json.Unmarshal(fixtureRaw, &fixture); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	goldenRaw, err := os.ReadFile(planPath)
	if err != nil {
		t.Fatalf("read python plan: %v", err)
	}
	var golden planDoc
	if err := json.Unmarshal(goldenRaw, &golden); err != nil {
		t.Fatalf("parse python plan: %v", err)
	}
	if len(golden.Tasks) != len(fixture.Tasks) {
		t.Fatalf("fixture has %d tasks, plan has %d", len(fixture.Tasks), len(golden.Tasks))
	}

	for i, raw := range fixture.Tasks {
		var task Task
		if err := json.Unmarshal(raw, &task); err != nil {
			t.Fatalf("parse task %d: %v", i, err)
		}
		api := &fakeAPI{share: fixture.Share, dest: fixture.Dest}
		rules, err := ParseRules(fixture.MagicRegex)
		if err != nil {
			t.Fatalf("parse fixture rules: %v", err)
		}
		runner := NewRunner(api, rules)
		runner.DryRun = false
		runner.Out = io.Discard
		tree, err := runner.DoSaveTask(&task)
		if err != nil {
			t.Fatalf("%s: %v", task.TaskName, err)
		}
		got := planTask{TaskName: task.TaskName, Saved: []planItem{}, Renames: api.renames, Ban: task.ShareURLBan}
		if got.Renames == nil {
			got.Renames = []planRename{}
		}
		if tree != nil {
			var collect func(node *TreeNode)
			collect = func(node *TreeNode) {
				for _, child := range node.Children {
					data := child.Data
					if data.FileName != "" {
						got.Saved = append(got.Saved, planItem{
							Path:       data.Path,
							FileName:   data.FileName,
							FileNameRe: data.FileNameRe,
							Dir:        data.IsDir,
						})
					}
					collect(child)
				}
			}
			collect(tree.Root())
		}
		want := golden.Tasks[i]
		if want.Saved == nil {
			want.Saved = []planItem{}
		}
		if want.Renames == nil {
			want.Renames = []planRename{}
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("task %d %s differs from Python:\n got: %s\nwant: %s",
				i, task.TaskName, mustJSON(got), mustJSON(want))
		}
	}
}

func mustJSON(v any) string {
	raw, err := json.Marshal(v)
	if err != nil {
		return err.Error()
	}
	return string(raw)
}
