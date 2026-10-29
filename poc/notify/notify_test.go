package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"mime"
	"net"
	"net/http"
	"net/http/httptest"
	"net/mail"
	"net/url"
	"strings"
	"testing"
	"time"
)

func decodeConfig(t *testing.T, values map[string]any) Config {
	t.Helper()
	raw, err := json.Marshal(values)
	if err != nil {
		t.Fatal(err)
	}
	return Decode(raw)
}

func TestChannelsReportConfiguredOnly(t *testing.T) {
	cfg := decodeConfig(t, map[string]any{
		"BARK_PUSH":     "abc",
		"CONSOLE":       true,
		"DD_BOT_TOKEN":  "token",
		"DD_BOT_SECRET": "secret",
		"SMTP_SERVER":   "smtp.example.com:465",
	})
	got := strings.Join(Channels(cfg), ",")
	want := "Bark,Console,钉钉机器人"
	if got != want {
		t.Fatalf("Channels = %q, want %q", got, want)
	}
}

func TestDecodeAppliesEnvOverrides(t *testing.T) {
	t.Setenv("TG_BOT_TOKEN", "from-env")
	cfg := decodeConfig(t, map[string]any{"TG_BOT_TOKEN": "from-file"})
	if cfg.Str("TG_BOT_TOKEN") != "from-env" {
		t.Fatalf("env override = %q", cfg.Str("TG_BOT_TOKEN"))
	}
}

func TestSendBark(t *testing.T) {
	var payload map[string]any
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method = %s", r.Method)
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		_ = json.NewEncoder(w).Encode(map[string]any{"code": 200})
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{
		"BARK_PUSH":  server.URL + "/push",
		"BARK_GROUP": "转存",
		"BARK_SOUND": "bell",
	})
	var out strings.Builder
	results := New(&out).Send(context.Background(), cfg, "标题", "正文")
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("results = %#v, log=%s", results, out.String())
	}
	if payload["title"] != "标题" || payload["body"] != "正文" {
		t.Fatalf("payload = %#v", payload)
	}
	if payload["group"] != "转存" || payload["sound"] != "bell" {
		t.Fatalf("optional params = %#v", payload)
	}
}

func TestSendGotify(t *testing.T) {
	var token, title string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/message" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = r.ParseForm()
		token = r.URL.Query().Get("token")
		title = r.PostForm.Get("title")
		_ = json.NewEncoder(w).Encode(map[string]any{"id": 7})
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{
		"GOTIFY_URL": server.URL, "GOTIFY_TOKEN": "gotify-token", "GOTIFY_PRIORITY": 4,
	})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if token != "gotify-token" || title != "标题" {
		t.Fatalf("token=%q title=%q", token, title)
	}
}

func TestSendPushDeer(t *testing.T) {
	var form url.Values
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		form = r.PostForm
		_ = json.NewEncoder(w).Encode(map[string]any{
			"code": 0, "content": map[string]any{"result": []any{"ok"}},
		})
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{"DEER_KEY": "k", "DEER_URL": server.URL})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if form.Get("pushkey") != "k" || form.Get("type") != "markdown" {
		t.Fatalf("form = %#v", form)
	}
}

func TestSendSynologyChat(t *testing.T) {
	var payload string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/tok" {
			t.Errorf("path = %s", r.URL.Path)
		}
		_ = r.ParseForm()
		payload = r.PostForm.Get("payload")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{"CHAT_URL": server.URL + "/", "CHAT_TOKEN": "tok"})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if !strings.Contains(payload, "标题") {
		t.Fatalf("payload = %q", payload)
	}
}

func TestSendNtfy(t *testing.T) {
	var title, priority, body string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/topic1" {
			t.Errorf("path = %s", r.URL.Path)
		}
		title = r.Header.Get("Title")
		priority = r.Header.Get("Priority")
		raw, _ := io.ReadAll(r.Body)
		body = string(raw)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{
		"NTFY_URL": server.URL, "NTFY_TOPIC": "topic1", "NTFY_PRIORITY": "5",
	})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if priority != "5" || body != "正文" {
		t.Fatalf("priority=%q body=%q", priority, body)
	}
	if decoded, err := new(mime.WordDecoder).DecodeHeader(title); err != nil || decoded != "标题" {
		t.Fatalf("title header = %q (%v)", title, err)
	}
}

func TestSendTelegram(t *testing.T) {
	var path, text string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		text = r.URL.Query().Get("text")
		_ = json.NewEncoder(w).Encode(map[string]any{"ok": true})
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{
		"TG_BOT_TOKEN": "123:abc", "TG_USER_ID": "42", "TG_API_HOST": server.URL,
	})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if path != "/bot123:abc/sendMessage" {
		t.Fatalf("path = %s", path)
	}
	if !strings.Contains(text, "标题") || !strings.Contains(text, "正文") {
		t.Fatalf("text = %q", text)
	}
}

func TestSendWecomBot(t *testing.T) {
	var key, content string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/cgi-bin/webhook/send" {
			t.Errorf("path = %s", r.URL.Path)
		}
		key = r.URL.Query().Get("key")
		var payload struct {
			Text struct {
				Content string `json:"content"`
			} `json:"text"`
		}
		_ = json.NewDecoder(r.Body).Decode(&payload)
		content = payload.Text.Content
		_ = json.NewEncoder(w).Encode(map[string]any{"errcode": 0})
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{"QYWX_KEY": "wxkey", "QYWX_ORIGIN": server.URL})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if key != "wxkey" || !strings.Contains(content, "正文") {
		t.Fatalf("key=%q content=%q", key, content)
	}
}

func TestSendPushMe(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("success"))
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{"PUSHME_KEY": "k", "PUSHME_URL": server.URL})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "正文")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
}

func TestWebhookJSONSubstitution(t *testing.T) {
	var gotQuery, gotTitle, gotContent, gotHeader, gotContentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("t")
		gotHeader = r.Header.Get("X-Token")
		gotContentType = r.Header.Get("Content-Type")
		var payload map[string]any
		_ = json.NewDecoder(r.Body).Decode(&payload)
		gotTitle, _ = payload["title"].(string)
		gotContent, _ = payload["content"].(string)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	cfg := decodeConfig(t, map[string]any{
		"WEBHOOK_URL":          server.URL + "/hook?t=$title",
		"WEBHOOK_METHOD":       "POST",
		"WEBHOOK_CONTENT_TYPE": "application/json",
		"WEBHOOK_BODY":         "title: $title\ncontent: $content",
		"WEBHOOK_HEADERS":      "X-Token: abc",
	})
	results := New(io.Discard).Send(context.Background(), cfg, "标题", "第一行\n第二行")
	if results[0].Err != nil {
		t.Fatal(results[0].Err)
	}
	if gotQuery != "标题" || gotHeader != "abc" {
		t.Fatalf("query=%q header=%q", gotQuery, gotHeader)
	}
	if gotContentType != "application/json" {
		t.Fatalf("content type = %q", gotContentType)
	}
	if gotTitle != "标题" {
		t.Fatalf("title = %q", gotTitle)
	}
	if gotContent != `第一行\n第二行` {
		t.Fatalf("content = %q", gotContent)
	}
}

func TestParseBodyForm(t *testing.T) {
	body := parseBody("title: $title\ncount: 2\nflag: true", "application/x-www-form-urlencoded", func(v string) string {
		return strings.ReplaceAll(v, "$title", "你好 世界")
	})
	values, err := url.ParseQuery(body)
	if err != nil {
		t.Fatal(err)
	}
	if values.Get("title") != "你好 世界" || values.Get("count") != "2" || values.Get("flag") != "true" {
		t.Fatalf("body = %q", body)
	}
}

func TestParseHeadersMergesDuplicates(t *testing.T) {
	headers := parseHeaders("X-A: 1\nx-a: 2\n\nWeird\nX-B: 3")
	if headers["x-a"] != "1, 2" || headers["x-b"] != "3" {
		t.Fatalf("headers = %#v", headers)
	}
}

func TestDingtalkSign(t *testing.T) {
	secret := "SECabc"
	timestamp := "1700000000000"
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	want := url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
	if got := dingtalkSign(secret, timestamp); got != want {
		t.Fatalf("sign = %q, want %q", got, want)
	}
}

func TestSignNotifyEnabled(t *testing.T) {
	if !decodeConfig(t, map[string]any{}).SignNotifyEnabled() {
		t.Fatal("sign notify should default to enabled")
	}
	if decodeConfig(t, map[string]any{"QUARK_SIGN_NOTIFY": "false"}).SignNotifyEnabled() {
		t.Fatal("string false should disable")
	}
	t.Setenv("QUARK_SIGN_NOTIFY", "false")
	if decodeConfig(t, map[string]any{}).SignNotifyEnabled() {
		t.Fatal("env false should disable")
	}
}

func TestSendSkipsInactiveChannels(t *testing.T) {
	var out strings.Builder
	results := New(&out).Send(context.Background(), decodeConfig(t, map[string]any{}), "标题", "正文")
	if len(results) != 0 {
		t.Fatalf("results = %#v", results)
	}
	if !strings.Contains(out.String(), "无推送渠道") {
		t.Fatalf("log = %q", out.String())
	}
}

// --- local SMTP fixture -------------------------------------------------

func startSMTPServer(t *testing.T) (string, <-chan string) {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	messages := make(chan string, 1)
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			go handleSMTP(conn, messages)
		}
	}()
	return listener.Addr().String(), messages
}

func handleSMTP(conn net.Conn, messages chan<- string) {
	defer conn.Close()
	reader := newLineReader(conn)
	write := func(text string) { _, _ = conn.Write([]byte(text + "\r\n")) }
	write("220 test ESMTP")
	for {
		line, ok := reader.read()
		if !ok {
			return
		}
		upper := strings.ToUpper(line)
		switch {
		case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
			write("250-test")
			write("250 AUTH PLAIN")
		case strings.HasPrefix(upper, "AUTH"):
			write("235 ok")
		case strings.HasPrefix(upper, "MAIL FROM"), strings.HasPrefix(upper, "RCPT TO"):
			write("250 ok")
		case strings.HasPrefix(upper, "DATA"):
			write("354 go ahead")
			var body strings.Builder
			for {
				dataLine, ok := reader.read()
				if !ok {
					return
				}
				if dataLine == "." {
					break
				}
				body.WriteString(strings.TrimPrefix(dataLine, ".") + "\n")
			}
			messages <- body.String()
			write("250 ok")
		case strings.HasPrefix(upper, "QUIT"):
			write("221 bye")
			return
		default:
			write("250 ok")
		}
	}
}

type lineReader struct {
	conn net.Conn
	buf  []byte
}

func newLineReader(conn net.Conn) *lineReader { return &lineReader{conn: conn} }

func (r *lineReader) read() (string, bool) {
	for {
		if index := strings.IndexByte(string(r.buf), '\n'); index >= 0 {
			line := strings.TrimSuffix(string(r.buf[:index]), "\r")
			r.buf = r.buf[index+1:]
			return line, true
		}
		chunk := make([]byte, 4096)
		n, err := r.conn.Read(chunk)
		if n > 0 {
			r.buf = append(r.buf, chunk[:n]...)
			continue
		}
		if err != nil {
			return "", false
		}
	}
}

func TestSendSMTP(t *testing.T) {
	addr, messages := startSMTPServer(t)
	cfg := decodeConfig(t, map[string]any{
		"SMTP_SERVER":   addr,
		"SMTP_SSL":      "false",
		"SMTP_EMAIL":    "from@example.com",
		"SMTP_PASSWORD": "pw",
		"SMTP_NAME":     "发件人",
		"SMTP_EMAIL_TO": "a@example.com, b@example.com",
		"SMTP_NAME_TO":  "甲,乙",
	})

	var out strings.Builder
	results := New(&out).Send(context.Background(), cfg, "标题", "第一行\n第二行")
	if results[0].Err != nil {
		t.Fatalf("%v log=%s", results[0].Err, out.String())
	}

	select {
	case raw := <-messages:
		message, err := mail.ReadMessage(strings.NewReader(raw))
		if err != nil {
			t.Fatal(err)
		}
		if subject, err := new(mime.WordDecoder).DecodeHeader(message.Header.Get("Subject")); err != nil || subject != "标题" {
			t.Fatalf("subject = %q (%v)", message.Header.Get("Subject"), err)
		}
		to := message.Header.Get("To")
		if !strings.Contains(to, "a@example.com") || !strings.Contains(to, "b@example.com") {
			t.Fatalf("to = %q", to)
		}
		encoded, err := io.ReadAll(message.Body)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := base64.StdEncoding.DecodeString(strings.ReplaceAll(string(encoded), "\n", ""))
		if err != nil {
			t.Fatal(err)
		}
		if string(decoded) != "第一行\n第二行" {
			t.Fatalf("body = %q", decoded)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("SMTP server received no message")
	}
}
