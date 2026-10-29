package quark

import (
	"net/url"
	"strings"
	"testing"
)

func TestNormalizePath(t *testing.T) {
	cases := map[string]string{
		"poc-verify":  "/poc-verify",
		"//a//b/":     "/a/b/",
		"/":           "/",
		" /video/tv ": "/video/tv",
	}
	for in, want := range cases {
		if got := normalizePath(in); got != want {
			t.Errorf("normalizePath(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestRedactURL(t *testing.T) {
	u, err := url.Parse("https://example.com/x?stoken=abc&kps=k&fr=pc")
	if err != nil {
		t.Fatal(err)
	}
	out := redactURL(u)
	if strings.Contains(out, "abc") || strings.Contains(out, "kps=k") {
		t.Fatalf("redactURL leaked secrets: %s", out)
	}
	if !strings.Contains(out, "fr=pc") {
		t.Fatalf("redactURL dropped benign params: %s", out)
	}
}

func TestParseMParam(t *testing.T) {
	cookie := "__pus=a; kps=abc%25d; sign=s1; vcode=v1; __kp=b"
	got := parseMParam(cookie)
	if got["kps"] != "abc%d" || got["sign"] != "s1" || got["vcode"] != "v1" {
		t.Fatalf("parseMParam = %#v", got)
	}
	if parseMParam("__pus=a; __kp=b") != nil {
		t.Fatal("expected nil when mobile params are missing")
	}
}

func TestExtractShareURL(t *testing.T) {
	ref, err := ExtractShareURL("https://pan.quark.cn/s/6f1d2a9c8b7e?pwd=ab12#/list/share")
	if err != nil {
		t.Fatal(err)
	}
	if ref.PwdID != "6f1d2a9c8b7e" || ref.Passcode != "ab12" {
		t.Fatalf("ref = %#v", ref)
	}
	if _, err := ExtractShareURL("https://pan.quark.cn/"); err == nil {
		t.Fatal("expected error for url without /s/<id>")
	}
}
