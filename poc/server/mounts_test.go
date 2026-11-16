package server

import (
	"context"
	"errors"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"
)

// stubMounts replaces the kernel mount table for the duration of a test.
func stubMounts(t *testing.T, points []string, err error) {
	t.Helper()
	original := readMountPoints
	readMountPoints = func() ([]string, error) { return points, err }
	t.Cleanup(func() { readMountPoints = original })
}

func TestParseMountPoints(t *testing.T) {
	data := []byte(`/dev/mapper/trim-a /vol2 btrfs rw,relatime,space_cache=v2 0 0
/dev/nvme0n1p3 / ext4 rw 0 0
tmpfs /mnt/my\040disk tmpfs rw 0 0
garbage-line
`)
	got := parseMountPoints(data)
	want := []string{"/vol2", "/", "/mnt/my disk"}
	if len(got) != len(want) {
		t.Fatalf("mount points = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("mount point %d = %q, want %q", i, got[i], want[i])
		}
	}
}

func TestStorageSpaceOf(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"/vol2/1000/影视热资源", "/vol2"},
		{"/vol2", "/vol2"},
		{"/vol2/", "/vol2"},
		{"/vol02/1000-1-a8f36853/dav", "/vol02"},
		{"/vol22/tv", "/vol22"},
		{"/volume1/media", ""},
		{"/vol/media", ""},
		{"/vol2abc/media", ""},
		{"/tmp/qas", ""},
		{"", ""},
	}
	for _, tc := range cases {
		if got := storageSpaceOf(tc.path); got != tc.want {
			t.Fatalf("storageSpaceOf(%q) = %q, want %q", tc.path, got, tc.want)
		}
	}
}

func TestCheckStorageMounted(t *testing.T) {
	real := []string{"/", "/vol1", "/vol2", "/vol02/1000-1-a8f36853"}
	cases := []struct {
		name   string
		root   string
		mounts []string
		want   string
	}{
		{"mounted space", "/vol2/1000/影视热资源/电影/空枪", real, ""},
		{"space itself", "/vol2", real, ""},
		{"trailing slash", "/vol2/", real, ""},
		{"cloud mount below vol02", "/vol02/1000-1-a8f36853/movies", real, ""},
		{"deeper mount inside a space", "/vol1/1000/mnt1/movies", []string{"/", "/vol1", "/vol1/1000/mnt1"}, ""},
		{"unmounted space", "/vol2/1000/影视热资源", []string{"/", "/vol1"}, "/vol2"},
		{"another space is not a prefix", "/vol20/1000", []string{"/", "/vol2"}, "/vol20"},
		{"outside any space", t.TempDir(), real, ""},
	}
	for _, tc := range cases {
		stubMounts(t, tc.mounts, nil)
		got := checkStorageMounted(tc.root)
		if tc.want == "" {
			if got != "" {
				t.Fatalf("%s: checkStorageMounted(%q) = %q, want no error", tc.name, tc.root, got)
			}
			continue
		}
		if !strings.Contains(got, "未挂载") || !strings.Contains(got, tc.want) {
			t.Fatalf("%s: checkStorageMounted(%q) = %q, want a 未挂载 message for %s", tc.name, tc.root, got, tc.want)
		}
	}
}

// Without a usable mount table there is nothing to compare against; the
// writability probe still runs, so the download must not be blocked.
func TestCheckStorageMountedToleratesMissingMountTable(t *testing.T) {
	stubMounts(t, nil, errors.New("open /proc/mounts: no such file or directory"))
	if got := checkStorageMounted("/vol2/1000/media"); got != "" {
		t.Fatalf("checkStorageMounted = %q, want no error", got)
	}
}

func TestPrepareRefusesUnmountedStorageSpace(t *testing.T) {
	original := localRootValidator
	localRootValidator = func(string) string { return "" }
	t.Cleanup(func() { localRootValidator = original })
	stubMounts(t, []string{"/", "/vol1"}, nil)

	root := "/vol2/qas-mount-guard-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	queue := &downloadQueue{}
	job := &downloadJob{TaskName: "空枪", Root: root}
	message := queue.prepare(job)
	if !strings.Contains(message, "未挂载") {
		t.Fatalf("prepare = %q, want a 未挂载 message", message)
	}
	// The point of the check: nothing may be created on the system disk.
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("%s was created although the space is not mounted", root)
	}
}

func TestTransferStopsWhenSpaceIsUnmounted(t *testing.T) {
	stubMounts(t, []string{"/", "/vol1"}, nil)
	root := "/vol2/qas-transfer-guard-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	t.Cleanup(func() { _ = os.RemoveAll(root) })

	queue := &downloadQueue{}
	job := &downloadJob{TaskName: "空枪", Root: root}
	result := queue.transfer(job, context.Background(), downloadItem{RelPath: "电影/空枪.mkv", FID: "f1"})
	if result != transferFailed {
		t.Fatalf("transfer result = %d, want transferFailed", result)
	}
	view := job.view()
	if view.Status != "failed" || !strings.Contains(view.Message, "未挂载") {
		t.Fatalf("job view = %+v, want failed with a 未挂载 message", view)
	}
	if _, err := os.Stat(root); !os.IsNotExist(err) {
		t.Fatalf("%s was created although the space is not mounted", root)
	}
}
