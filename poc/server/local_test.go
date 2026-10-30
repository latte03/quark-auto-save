package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"qas-poc/quark"
)

func TestValidateLocalRoot(t *testing.T) {
	rejected := map[string]string{
		"":                       "请选择或填写下载目录",
		"relative/path":          "绝对路径",
		"/":                      "根目录",
		"/tmp/media":             "/vol",
		"/vol1/@appdata/com.qas": "应用系统目录",
	}
	for path, want := range rejected {
		if got := validateLocalRoot(path); !strings.Contains(got, want) {
			t.Fatalf("validateLocalRoot(%q) = %q, want containing %q", path, got, want)
		}
	}
	if got := validateLocalRoot("/vol1/1000/Media"); got != "" {
		t.Fatalf("valid path rejected: %q", got)
	}
}

func TestMatchAuthorizedRoot(t *testing.T) {
	roots := []string{"/vol2/1000/影视热资源", "/vol1/1000/Download"}
	if got := matchAuthorizedRoot("/vol2/1000/影视热资源", roots); got == "" {
		t.Fatal("root itself should match")
	}
	if got := matchAuthorizedRoot("/vol2/1000/影视热资源/剧集", roots); got == "" {
		t.Fatal("child should match")
	}
	for _, path := range []string{"/vol2/1000/影视热资源备份", "/vol1/1000", "/etc"} {
		if got := matchAuthorizedRoot(path, roots); got != "" {
			t.Fatalf("%s should not match, got %q", path, got)
		}
	}
}

func TestResolveAuthorizedPathRejectsEscapingSymlink(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "root")
	outside := filepath.Join(base, "outside")
	if err := os.MkdirAll(filepath.Join(root, "child"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(outside, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Fatal(err)
	}
	// t.TempDir 在 macOS 上经过 /var → /private/var 软链，按解析后的路径比对。
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatal(err)
	}
	roots := []string{resolvedRoot}

	if _, _, ok := resolveAuthorizedPath(filepath.Join(resolvedRoot, "child"), roots); !ok {
		t.Fatal("a child directory should stay authorized")
	}
	if resolved, _, ok := resolveAuthorizedPath(filepath.Join(resolvedRoot, "escape"), roots); ok {
		t.Fatalf("symlink escaping the root was accepted as %q", resolved)
	}
}

func TestLocalDownloadMinFreeDefaults(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want int64
	}{
		{"missing section", `{"tasklist":[]}`, 5},
		{"missing key", `{"local_download":{"dir":"/vol1/media"}}`, 5},
		{"explicit zero", `{"local_download":{"min_free_gb":0}}`, 0},
		{"explicit value", `{"local_download":{"min_free_gb":12}}`, 12},
	}
	for _, tc := range cases {
		path := filepath.Join(t.TempDir(), "config.json")
		if err := os.WriteFile(path, []byte(tc.raw), 0o644); err != nil {
			t.Fatal(err)
		}
		cfg, err := quark.LoadConfig(path)
		if err != nil {
			t.Fatal(err)
		}
		if got := localDownloadFromConfig(cfg).MinFreeGB; got != tc.want {
			t.Fatalf("%s: min_free_gb = %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestCheckLocalDir(t *testing.T) {
	dir := t.TempDir()
	check := checkLocalDir(dir)
	if !check.OK || !check.Writable {
		t.Fatalf("temp dir should be writable: %+v", check)
	}
	if check.FreeBytes == 0 {
		t.Fatalf("free bytes should be reported: %+v", check)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("probe file left behind: %v", entries)
	}

	missing := checkLocalDir(filepath.Join(dir, "nope"))
	if missing.OK || !strings.Contains(missing.Message, "不存在") {
		t.Fatalf("missing dir should fail: %+v", missing)
	}
}

func TestHandleLocalValidate(t *testing.T) {
	body := strings.NewReader(`{"path":"relative/dir"}`)
	request := httptest.NewRequest("POST", "/api/local/validate", body)
	recorder := httptest.NewRecorder()
	(&Server{}).handleLocalValidate(recorder, request)

	var payload struct {
		Success bool          `json:"success"`
		Data    localDirCheck `json:"data"`
	}
	if err := json.Unmarshal(recorder.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v (%s)", err, recorder.Body.String())
	}
	if !payload.Success || payload.Data.OK || payload.Data.Message == "" {
		t.Fatalf("relative path should fail with a message, got %s", recorder.Body.String())
	}
}
