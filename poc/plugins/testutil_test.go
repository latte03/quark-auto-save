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

type fakeAccount struct {
	dirs            map[string][]quark.ShareFile
	dirErr          error
	downloads       []quark.DownloadItem
	cookie          string
	unarchiveTaskID string
	unarchiveErr    error
	unarchiveResult quark.TaskResult
	waitErr         error
	downloaded      []string
	moves           []moveCall
	deletes         [][]string
	renames         []renameCall
}

func (f *fakeAccount) LsDir(pdirFID string) ([]quark.ShareFile, error) {
	if f.dirErr != nil {
		return nil, f.dirErr
	}
	return f.dirs[pdirFID], nil
}

func (f *fakeAccount) Download(fids []string) ([]quark.DownloadItem, string, error) {
	f.downloaded = append(f.downloaded, fids...)
	return f.downloads, f.cookie, nil
}

func (f *fakeAccount) MoveFiles(fids []string, toPdirFID string) error {
	f.moves = append(f.moves, moveCall{FIDs: append([]string(nil), fids...), To: toPdirFID})
	return nil
}

func (f *fakeAccount) Unarchive(fid, toPdirFID string) (string, error) {
	if f.unarchiveErr != nil {
		return "", f.unarchiveErr
	}
	return f.unarchiveTaskID, nil
}

func (f *fakeAccount) WaitTask(taskID string, timeout time.Duration) (quark.TaskResult, error) {
	return f.unarchiveResult, f.waitErr
}

func (f *fakeAccount) Delete(fids []string) (string, error) {
	f.deletes = append(f.deletes, append([]string(nil), fids...))
	return "delete-task", nil
}

func (f *fakeAccount) Rename(fid, fileName string) error {
	f.renames = append(f.renames, renameCall{FID: fid, Name: fileName})
	return nil
}

func (f *fakeAccount) CookieHeader() string { return "cookie=main" }
func (f *fakeAccount) UserAgent() string    { return "test-ua" }
