package plugins

import (
	"time"

	"qas-poc/quark"
)

type renameCall struct {
	FID  string
	Name string
}

type moveCall struct {
	FIDs []string
	To   string
}

type unarchiveCall struct {
	FID string
	To  string
}

type fakeAccount struct {
	dirs            map[string][]quark.ShareFile
	dirErr          error
	downloads       []quark.DownloadItem
	cookie          string
	unarchiveTaskID string
	unarchiveErr    error
	unarchiveResult quark.TaskResult
	// taskResults replays one result per WaitTask call before falling back to
	// unarchiveResult, so a test can model "the result was empty, then filled".
	taskResults []quark.TaskResult
	waitErr     error
	waitCalls   int
	downloaded  []string
	moves       []moveCall
	unarchives  []unarchiveCall
	deletes     [][]string
	renames     []renameCall
	moveErr     error
	// dropMoves models a server that answers move successfully but never lands
	// the files.
	dropMoves bool
	renameErr error
	deleteErr error
	// dirEmptyCalls answers the first N LsDir calls for a folder with an empty
	// list, which is what Quark does right after an unarchive task reports done.
	dirEmptyCalls map[string]int
	dirCalls      map[string]int
}

func (f *fakeAccount) LsDir(pdirFID string) ([]quark.ShareFile, error) {
	if f.dirCalls == nil {
		f.dirCalls = map[string]int{}
	}
	f.dirCalls[pdirFID]++
	if f.dirErr != nil {
		return nil, f.dirErr
	}
	if f.dirCalls[pdirFID] <= f.dirEmptyCalls[pdirFID] {
		return nil, nil
	}
	return f.dirs[pdirFID], nil
}

func (f *fakeAccount) Download(fids []string) ([]quark.DownloadItem, string, error) {
	f.downloaded = append(f.downloaded, fids...)
	return f.downloads, f.cookie, nil
}

func (f *fakeAccount) MoveFiles(fids []string, toPdirFID string) error {
	f.moves = append(f.moves, moveCall{FIDs: append([]string(nil), fids...), To: toPdirFID})
	if f.moveErr != nil {
		return f.moveErr
	}
	if f.dropMoves {
		return nil
	}
	if f.dirs == nil {
		f.dirs = map[string][]quark.ShareFile{}
	}
	for _, fid := range fids {
		f.dirs[toPdirFID] = append(f.dirs[toPdirFID], quark.ShareFile{FID: fid, FileName: f.nameOf(fid)})
	}
	return nil
}

func (f *fakeAccount) Unarchive(fid, toPdirFID string) (string, error) {
	f.unarchives = append(f.unarchives, unarchiveCall{FID: fid, To: toPdirFID})
	if f.unarchiveErr != nil {
		return "", f.unarchiveErr
	}
	return f.unarchiveTaskID, nil
}

func (f *fakeAccount) WaitTask(taskID string, timeout time.Duration) (quark.TaskResult, error) {
	call := f.waitCalls
	f.waitCalls++
	if f.waitErr != nil {
		return quark.TaskResult{}, f.waitErr
	}
	if call < len(f.taskResults) {
		return f.taskResults[call], nil
	}
	return f.unarchiveResult, nil
}

func (f *fakeAccount) Delete(fids []string) (string, error) {
	f.deletes = append(f.deletes, append([]string(nil), fids...))
	if f.deleteErr != nil {
		return "", f.deleteErr
	}
	return "delete-task", nil
}

func (f *fakeAccount) Rename(fid, fileName string) error {
	f.renames = append(f.renames, renameCall{FID: fid, Name: fileName})
	if f.renameErr != nil {
		return f.renameErr
	}
	for dir, list := range f.dirs {
		for index := range list {
			if list[index].FID == fid {
				list[index].FileName = fileName
				f.dirs[dir] = list
			}
		}
	}
	return nil
}

// nameOf resolves the current name of a fid, mirroring the server's view after
// a rename, so a move lands under the name the plugin expects.
func (f *fakeAccount) nameOf(fid string) string {
	for _, list := range f.dirs {
		for _, item := range list {
			if item.FID == fid {
				return item.FileName
			}
		}
	}
	return ""
}

func (f *fakeAccount) CookieHeader() string { return "cookie=main" }
func (f *fakeAccount) UserAgent() string    { return "test-ua" }
