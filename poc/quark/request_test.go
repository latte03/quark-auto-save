package quark

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeToken(w http.ResponseWriter) {
	w.Header().Set("content-type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"status":  200,
		"code":    0,
		"message": "ok",
		"data":    map[string]string{"stoken": "s-1", "title": "t"},
	})
}

func TestMobileShareRequestSwitchesHostAndDropsCookie(t *testing.T) {
	var pcHits, appHits int
	var gotCookie, gotKPS string
	pc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		pcHits++
		writeToken(w)
	}))
	defer pc.Close()
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		appHits++
		gotCookie = r.Header.Get("cookie")
		gotKPS = r.URL.Query().Get("kps")
		writeToken(w)
	}))
	defer app.Close()

	client := New("__pus=a; kps=k1; sign=s1; vcode=v1")
	client.SetBaseURL(pc.URL, app.URL)
	if _, err := client.GetStoken("pwd", ""); err != nil {
		t.Fatal(err)
	}
	if pcHits != 0 || appHits != 1 {
		t.Fatalf("hits: pc=%d app=%d, want pc=0 app=1", pcHits, appHits)
	}
	if gotCookie != "" {
		t.Fatalf("mobile request sent cookie %q", gotCookie)
	}
	if gotKPS != "k1" {
		t.Fatalf("mobile request kps = %q", gotKPS)
	}
}

func TestPCRequestKeepsCookie(t *testing.T) {
	var gotCookie string
	pc := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCookie = r.Header.Get("cookie")
		writeToken(w)
	}))
	defer pc.Close()

	client := New("__pus=pc-only")
	client.SetBaseURL(pc.URL, "http://app.invalid")
	if _, err := client.GetStoken("pwd", ""); err != nil {
		t.Fatal(err)
	}
	if gotCookie != "__pus=pc-only" {
		t.Fatalf("cookie = %q", gotCookie)
	}
}
