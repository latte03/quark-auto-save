package plugins

import (
	"errors"
	"testing"
	"time"

	"qas-poc/quark"
)

// newPollingPlugin records the polling windows instead of sleeping through them.
func newPollingPlugin(t *testing.T) (*AutoUnarchive, *[]string, *[]time.Duration) {
	t.Helper()
	deps, logs := testDeps(t)
	plugin := NewAutoUnarchive(nil, deps)
	var sleeps []time.Duration
	plugin.sleep = func(d time.Duration) { sleeps = append(sleeps, d) }
	return plugin, logs, &sleeps
}

func runUnarchive(t *testing.T, plugin *AutoUnarchive, account quark.PluginAccount) {
	t.Helper()
	env := &quark.PluginEnv{
		Account:     account,
		Tree:        archiveTree(),
		SavepathFID: map[string]string{"/tv": "target"},
	}
	if err := plugin.Run(pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true}}`), env); err != nil {
		t.Fatal(err)
	}
}

// Quark reports 解压完成 before the extracted file can be listed; the plugin has
// to keep asking instead of taking the first empty answer for "nothing there".
func TestAutoUnarchiveWaitsForExtractDir(t *testing.T) {
	plugin, logs, sleeps := newPollingPlugin(t)
	account := unarchiveAccount()
	account.dirEmptyCalls = map[string]int{"sub1": 3}

	runUnarchive(t, plugin, account)

	if len(*sleeps) != 3 {
		t.Fatalf("slept %d times, want 3: %v", len(*sleeps), *sleeps)
	}
	for _, waited := range *sleeps {
		if waited != extractPollInterval {
			t.Fatalf("poll interval = %s, want %s", waited, extractPollInterval)
		}
	}
	if len(account.moves) != 1 || len(account.moves[0].FIDs) != 1 || account.moves[0].FIDs[0] != "x1" {
		t.Fatalf("moves = %#v", account.moves)
	}
	if len(account.deletes) != 1 || len(account.deletes[0]) != 1 || account.deletes[0][0] != "zip1" {
		t.Fatalf("deletes = %#v", account.deletes)
	}
	if !hasLog(*logs, "重命名: show.mkv") || !hasLog(*logs, "批量清理完成") {
		t.Fatalf("logs = %v", *logs)
	}
	if !hasLog(*logs, "解压结果延迟出现") {
		t.Fatalf("a delayed extract dir should be reported: %v", *logs)
	}
}

// An extract folder that never fills up must leave both the archive and the
// folder alone; deleting them is what lost episodes permanently.
func TestAutoUnarchiveKeepsArchiveWhenExtractStaysEmpty(t *testing.T) {
	plugin, logs, sleeps := newPollingPlugin(t)
	account := unarchiveAccount()
	account.dirEmptyCalls = map[string]int{"sub1": 1 << 20}

	runUnarchive(t, plugin, account)

	if len(account.moves) != 0 {
		t.Fatalf("moved files out of an empty extract dir: %#v", account.moves)
	}
	if len(account.deletes) != 0 {
		t.Fatalf("cleaned up although nothing was extracted: %#v", account.deletes)
	}
	if len(*sleeps) != extractPollAttempts-1 {
		t.Fatalf("polled %d times, want %d", len(*sleeps)+1, extractPollAttempts)
	}
	if account.waitCalls != 2 {
		t.Fatalf("WaitTask calls = %d, want the result to be read again", account.waitCalls)
	}
	if !hasLog(*logs, "解压目录为空") || !hasLog(*logs, "保留压缩包") {
		t.Fatalf("logs = %v", *logs)
	}
}

// The folder itself can show up late, so an empty unarchive result is re-read.
func TestAutoUnarchiveRereadsTaskForLateExtractDir(t *testing.T) {
	plugin, logs, _ := newPollingPlugin(t)
	account := unarchiveAccount()
	account.taskResults = []quark.TaskResult{
		{UnarchiveList: nil},
		{UnarchiveList: []quark.ShareFile{{FID: "sub1", FileName: "show"}}},
	}

	runUnarchive(t, plugin, account)

	if account.waitCalls != 2 {
		t.Fatalf("WaitTask calls = %d, want 2", account.waitCalls)
	}
	if len(account.moves) != 1 || account.moves[0].FIDs[0] != "x1" {
		t.Fatalf("moves = %#v", account.moves)
	}
	if len(account.deletes) != 1 || account.deletes[0][0] != "zip1" {
		t.Fatalf("deletes = %#v", account.deletes)
	}
	if !hasLog(*logs, "解压完成: show.zip") {
		t.Fatalf("logs = %v", *logs)
	}
}

func TestAutoUnarchiveKeepsArchiveWhenMoveFails(t *testing.T) {
	plugin, logs, _ := newPollingPlugin(t)
	account := unarchiveAccount()
	account.moveErr = errors.New("move rejected")

	runUnarchive(t, plugin, account)

	if len(account.deletes) != 0 {
		t.Fatalf("cleaned up after a failed move: %#v", account.deletes)
	}
	if !hasLog(*logs, "移动 1 个文件失败") || !hasLog(*logs, "移动不完整") {
		t.Fatalf("logs = %v", *logs)
	}
}

// A move the server accepts but never carries out must not count as success.
func TestAutoUnarchiveKeepsArchiveWhenTargetMissesFile(t *testing.T) {
	plugin, logs, _ := newPollingPlugin(t)
	account := unarchiveAccount()
	account.dropMoves = true

	runUnarchive(t, plugin, account)

	if len(account.deletes) != 0 {
		t.Fatalf("cleaned up although the target never showed the file: %#v", account.deletes)
	}
	if !hasLog(*logs, "移动后目标目录缺少 1 个文件") || !hasLog(*logs, "目标目录里看不到") {
		t.Fatalf("logs = %v", *logs)
	}
}

// The placeholder rename is what stops the next diff from re-saving the
// archive, so it may only happen once the files landed — and a failed rename
// must keep the archive instead of dropping it.
func TestAutoUnarchiveKeepsZipWhenPlaceholderRenameFails(t *testing.T) {
	plugin, logs, _ := newPollingPlugin(t)
	account := unarchiveAccount()
	account.renameErr = errors.New("rename rejected")

	runUnarchive(t, plugin, account)

	if len(account.deletes) != 0 {
		t.Fatalf("deleted the archive without a placeholder: %#v", account.deletes)
	}
	if !hasLog(*logs, "占位重命名失败") || !hasLog(*logs, "保留压缩包") {
		t.Fatalf("logs = %v", *logs)
	}
}

// After a verified unpack the archive is deleted from the drive, so its tree
// entry must be flagged: the download collector would otherwise queue a fid
// that Quark answers with 404 and report a healthy task as failed.
func TestAutoUnarchiveMarksCleanedArchiveConsumed(t *testing.T) {
	plugin, _, _ := newPollingPlugin(t)
	account := unarchiveAccount()
	tree := archiveTree()

	env := &quark.PluginEnv{Account: account, Tree: tree, SavepathFID: map[string]string{"/tv": "target"}}
	if err := plugin.Run(pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true}}`), env); err != nil {
		t.Fatal(err)
	}
	if node := findNode(tree, "zip1"); node == nil || !node.Data.Consumed {
		t.Fatalf("cleaned archive still downloadable: %+v", node)
	}

	// auto_clean=false keeps the archive, so nothing may be marked.
	plugin, _, _ = newPollingPlugin(t)
	account = unarchiveAccount()
	tree = archiveTree()
	env = &quark.PluginEnv{Account: account, Tree: tree, SavepathFID: map[string]string{"/tv": "target"}}
	if err := plugin.Run(pluginTask(t, "/tv", `{"auto_unarchive":{"enable":true,"auto_clean":false}}`), env); err != nil {
		t.Fatal(err)
	}
	if node := findNode(tree, "zip1"); node == nil || node.Data.Consumed {
		t.Fatalf("kept archive must stay downloadable: %+v", node)
	}
}

func findNode(tree *quark.Tree, fid string) *quark.TreeNode {
	var found *quark.TreeNode
	tree.Walk(func(node *quark.TreeNode, isRoot bool) {
		if !isRoot && node.Data.FID == fid {
			found = node
		}
	})
	return found
}

func TestAutoUnarchiveKeepsDirWhenListingFails(t *testing.T) {
	plugin, logs, _ := newPollingPlugin(t)
	account := unarchiveAccount()
	account.dirErr = errors.New("list rejected")

	runUnarchive(t, plugin, account)

	if len(account.deletes) != 0 {
		t.Fatalf("cleaned up without reading the extract dir: %#v", account.deletes)
	}
	if !hasLog(*logs, "读取解压目录失败") {
		t.Fatalf("logs = %v", *logs)
	}
}
