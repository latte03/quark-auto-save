package source

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

const cloudSaverResponse = `{
  "success": true,
  "data": [
    {
      "list": [
        {
          "title": "名称：黑镜 第七季",
          "content": "描述：更新至 <mark class=\"highlight\">E06</mark> 链接：...",
          "pubDate": "2026-10-27T14:14:00Z",
          "channelId": "tg-channel",
          "cloudLinks": [
            {"cloudType": "quark", "link": "https://pan.quark.cn/s/aaa"},
            {"cloudType": "baidu", "link": "https://pan.baidu.com/s/bbb"}
          ]
        }
      ]
    }
  ]
}`

func TestParseCloudSaver(t *testing.T) {
	results, message, ok := parseCloudSaver([]byte(cloudSaverResponse))
	if !ok {
		t.Fatalf("parse failed: %s", message)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d", len(results))
	}
	item := results[0]
	if item.TaskName != "黑镜 第七季" {
		t.Fatalf("title = %q", item.TaskName)
	}
	if item.Content != "更新至 E06" {
		t.Fatalf("content = %q", item.Content)
	}
	if item.ShareURL != "https://pan.quark.cn/s/aaa" || item.Source != "CloudSaver" {
		t.Fatalf("item = %+v", item)
	}
	if item.Datetime != "2026-10-27 22:14:00" {
		t.Fatalf("datetime = %q", item.Datetime)
	}
}

func TestParseCloudSaverError(t *testing.T) {
	_, message, ok := parseCloudSaver([]byte(`{"success":false,"message":"无效的 token"}`))
	if ok || message != "无效的 token" {
		t.Fatalf("ok=%v message=%q", ok, message)
	}
}

func TestParsePanSou(t *testing.T) {
	payload := `{
	  "code": 0,
	  "data": {
	    "merged_by_type": {
	      "quark": [
	        {"url": "https://pan.quark.cn/s/ccc", "note": "黑镜 第七季【简介】1080P 更新中", "datetime": "2026-10-27T10:00:00Z", "source": "hunhepan"},
	        {"url": "https://pan.quark.cn/s/ccc", "note": "重复", "datetime": "2026-10-27T10:00:00Z", "source": "dup"}
	      ]
	    }
	  }
	}`
	results, err := parsePanSou([]byte(payload))
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 1 {
		t.Fatalf("results = %d", len(results))
	}
	item := results[0]
	if item.TaskName != "黑镜 第七季" || item.Content != "1080P 更新中" {
		t.Fatalf("item = %+v", item)
	}
	if item.Channel != "hunhepan" || item.Source != "PanSou" {
		t.Fatalf("item = %+v", item)
	}
}

func TestIsoToCST(t *testing.T) {
	cases := map[string]string{
		"2026-10-27T14:14:00Z":      "2026-10-27 22:14:00",
		"2026-10-27T14:14:00+08:00": "2026-10-27 14:14:00",
		"2026-10-27T14:14:00":       "2026-10-27 14:14:00",
		"":                          "",
		"not-a-date":                "",
	}
	for input, want := range cases {
		if got := isoToCST(input); got != want {
			t.Fatalf("isoToCST(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSearchMergesSourcesAndRefreshesToken(t *testing.T) {
	cloudSaver := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/user/login":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"success": true,
				"data":    map[string]string{"token": "fresh-token"},
			})
		case "/api/search":
			if r.Header.Get("Authorization") != "Bearer fresh-token" {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"success": false,
					"message": "无效的 token",
				})
				return
			}
			_, _ = w.Write([]byte(cloudSaverResponse))
		default:
			http.NotFound(w, r)
		}
	}))
	defer cloudSaver.Close()

	panSou := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"code":0,"data":{"merged_by_type":{"quark":[
		  {"url":"https://pan.quark.cn/s/ccc","note":"黑镜 1080P","datetime":"2026-10-26T10:00:00Z","source":"hunhepan"},
		  {"url":"https://pan.quark.cn/s/aaa","note":"黑镜 重复链接","datetime":"2026-10-25T10:00:00Z","source":"dup"}
		]}}}`))
	}))
	defer panSou.Close()

	results, messages, token := Search("黑镜", false, Options{
		CloudSaver: CloudSaverConfig{Enable: true, Server: cloudSaver.URL, Username: "u", Password: "p", Token: "stale-token"},
		PanSou:     PanSouConfig{Enable: true, Server: panSou.URL},
		HTTP:       http.DefaultClient,
	})
	if len(messages) != 0 {
		t.Fatalf("messages = %v", messages)
	}
	if token != "fresh-token" {
		t.Fatalf("token = %q", token)
	}
	if len(results) != 2 {
		t.Fatalf("results = %+v", results)
	}
	// Sorted by publish time, newest first; duplicate links resolve to
	// CloudSaver regardless of which goroutine finished first.
	if results[0].ShareURL != "https://pan.quark.cn/s/aaa" || results[1].ShareURL != "https://pan.quark.cn/s/ccc" {
		t.Fatalf("order = %+v", results)
	}
	if results[0].Source != "CloudSaver" {
		t.Fatalf("duplicate link source = %q", results[0].Source)
	}
}

func TestSearchReportsPartialFailure(t *testing.T) {
	broken := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer broken.Close()

	results, messages, _ := Search("黑镜", false, Options{
		CloudSaver: CloudSaverConfig{Enable: true, Server: broken.URL, Token: "t"},
		HTTP:       http.DefaultClient,
	})
	if len(results) != 0 {
		t.Fatalf("results = %+v", results)
	}
	if len(messages) != 1 {
		t.Fatalf("messages = %v", messages)
	}
}
