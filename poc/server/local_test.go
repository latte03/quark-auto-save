package server

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
