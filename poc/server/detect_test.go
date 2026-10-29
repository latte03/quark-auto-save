package server

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProbeCloudSaver(t *testing.T) {
	online := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><title>CloudSaver</title></html>"))
	}))
	defer online.Close()
	if !probeCloudSaver(context.Background(), online.Client(), online.URL) {
		t.Fatal("expected CloudSaver to be detected")
	}

	offline := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("not the app"))
	}))
	defer offline.Close()
	if probeCloudSaver(context.Background(), offline.Client(), offline.URL) {
		t.Fatal("unrelated server must not be detected as CloudSaver")
	}
}

func TestProbePanSou(t *testing.T) {
	online := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"code":200,"data":{"merged_by_type":{}}}`))
	}))
	defer online.Close()
	if !probePanSou(context.Background(), online.Client(), online.URL) {
		t.Fatal("expected PanSou to be detected")
	}

	offline := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "nope", http.StatusNotFound)
	}))
	defer offline.Close()
	if probePanSou(context.Background(), offline.Client(), offline.URL) {
		t.Fatal("404 must not be detected as PanSou")
	}
}

func TestProbeAlistPing(t *testing.T) {
	online := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ping" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte("pong"))
	}))
	defer online.Close()
	if !probeAlist(context.Background(), online.Client(), online.URL) {
		t.Fatal("expected Alist to be detected by /ping")
	}
}

func TestProbeAlistPage(t *testing.T) {
	online := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte("<html><head><title>OpenList</title></head></html>"))
	}))
	defer online.Close()
	if !probeAlist(context.Background(), online.Client(), online.URL) {
		t.Fatal("expected OpenList to be detected by page title")
	}

	offline := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("unrelated service"))
	}))
	defer offline.Close()
	if probeAlist(context.Background(), offline.Client(), offline.URL) {
		t.Fatal("unrelated server must not be detected as Alist")
	}
}
