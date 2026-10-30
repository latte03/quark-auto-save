package server

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestQuarkQRLoginReplacesFirstAccount(t *testing.T) {
	instance, ts := newTestServer(t)

	var ticketCalls int32
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cas/ajax/getTokenForQrcodeLogin":
			_, _ = io.WriteString(w, `{"status":2000000,"message":"ok","data":{"members":{"token":"tok-1"}}}`)
		case "/cas/ajax/getServiceTicketByQrcodeToken":
			if atomic.AddInt32(&ticketCalls, 1) < 2 {
				_, _ = io.WriteString(w, `{"status":50004001,"message":"waiting"}`)
				return
			}
			_, _ = io.WriteString(w, `{"status":2000000,"message":"ok","data":{"members":{"service_ticket":"st-1"}}}`)
		case "/account/info":
			if got := r.URL.Query().Get("st"); got != "st-1" {
				t.Errorf("st = %q", got)
			}
			w.Header().Add("Set-Cookie", "__pus=abc; Domain=quark.cn; Path=/")
			w.Header().Add("Set-Cookie", "__uid=42; Domain=quark.cn; Path=/")
			w.Header().Add("Set-Cookie", "__kps=def; Domain=quark.cn; Path=/")
			_, _ = io.WriteString(w, `{"status":200,"data":{"nickname":"阿甘"}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()

	instance.qr.urls = qrEndpoints{
		token:   upstream.URL + "/cas/ajax/getTokenForQrcodeLogin",
		ticket:  upstream.URL + "/cas/ajax/getServiceTicketByQrcodeToken",
		scan:    "https://su.quark.cn/4_eMHBJ",
		account: upstream.URL + "/account/info",
	}
	var verified string
	instance.qr.verify = func(cookie string) (string, error) {
		verified = cookie
		return "阿甘", nil
	}

	client := loginClient(t, ts.URL)
	response, err := client.Post(ts.URL+"/api/quark/qr", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("start status = %d", response.StatusCode)
	}
	var start struct {
		Data qrStart `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&start); err != nil {
		t.Fatal(err)
	}
	if start.Data.ID == "" || !strings.Contains(start.Data.QRURL, "token=tok-1") ||
		!strings.HasPrefix(start.Data.QRURL, "https://su.quark.cn/4_eMHBJ?") {
		t.Fatalf("start = %+v", start.Data)
	}

	if status := pollQR(t, client, ts.URL, start.Data.ID); status.Status != "waiting" {
		t.Fatalf("first poll = %+v", status)
	}
	status := pollQR(t, client, ts.URL, start.Data.ID)
	if status.Status != "confirmed" || status.Nickname != "阿甘" {
		t.Fatalf("second poll = %+v", status)
	}
	if !strings.Contains(verified, "__pus=abc") || !strings.Contains(verified, "__uid=42") {
		t.Fatalf("verified cookie = %q", verified)
	}
	cookies := instance.store.Snapshot().Cookies
	if len(cookies) != 1 {
		t.Fatalf("cookies = %v", cookies)
	}
	if !strings.Contains(cookies[0], "__pus=abc") {
		t.Fatalf("stored cookie = %q", cookies[0])
	}
	if strings.Contains(cookies[0], "kps=placeholder") {
		t.Fatalf("old cookie kept: %q", cookies[0])
	}
	if status.Cookies == nil || len(status.Cookies) != 1 {
		t.Fatalf("returned cookies = %v", status.Cookies)
	}
}

func TestQuarkQRStalePollKeepsSession(t *testing.T) {
	instance, ts := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/cas/ajax/getTokenForQrcodeLogin":
			_, _ = io.WriteString(w, `{"status":2000000,"data":{"members":{"token":"tok-3"}}}`)
		case "/cas/ajax/getServiceTicketByQrcodeToken":
			_, _ = io.WriteString(w, `{"status":50004001,"message":"waiting"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer upstream.Close()
	instance.qr.urls = qrEndpoints{
		token:   upstream.URL + "/cas/ajax/getTokenForQrcodeLogin",
		ticket:  upstream.URL + "/cas/ajax/getServiceTicketByQrcodeToken",
		scan:    "https://su.quark.cn/4_eMHBJ",
		account: upstream.URL + "/account/info",
	}

	client := loginClient(t, ts.URL)
	response, err := client.Post(ts.URL+"/api/quark/qr", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var start struct {
		Data qrStart `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&start); err != nil {
		t.Fatal(err)
	}

	if status := pollQR(t, client, ts.URL, "stale-id"); status.Status != "expired" {
		t.Fatalf("stale poll = %+v", status)
	}
	if status := pollQR(t, client, ts.URL, start.Data.ID); status.Status != "waiting" {
		t.Fatalf("live session was dropped by a stale poll: %+v", status)
	}
}

func TestQRFinishOnlyClearsMatchingSession(t *testing.T) {
	q := newQRLogin(nil, log.New(io.Discard, "", 0))
	q.session = &qrSession{id: "new", token: "tok", createdAt: time.Now()}

	q.finish("stale")
	if q.session == nil {
		t.Fatal("a stale finish wiped the live session")
	}
	q.finish("new")
	if q.session != nil {
		t.Fatal("finishing the matching session should clear it")
	}
}

func TestQuarkQRSessionExpires(t *testing.T) {
	instance, ts := newTestServer(t)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/cas/ajax/getTokenForQrcodeLogin" {
			_, _ = io.WriteString(w, `{"status":2000000,"data":{"members":{"token":"tok-2"}}}`)
			return
		}
		http.NotFound(w, r)
	}))
	defer upstream.Close()
	instance.qr.urls = qrEndpoints{
		token:   upstream.URL + "/cas/ajax/getTokenForQrcodeLogin",
		ticket:  upstream.URL + "/cas/ajax/getServiceTicketByQrcodeToken",
		scan:    "https://su.quark.cn/4_eMHBJ",
		account: upstream.URL + "/account/info",
	}
	base := time.Now()
	instance.qr.now = func() time.Time { return base }

	client := loginClient(t, ts.URL)
	response, err := client.Post(ts.URL+"/api/quark/qr", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var start struct {
		Data qrStart `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&start); err != nil {
		t.Fatal(err)
	}

	instance.qr.now = func() time.Time { return base.Add(3 * time.Minute) }
	if status := pollQR(t, client, ts.URL, start.Data.ID); status.Status != "expired" {
		t.Fatalf("status = %+v", status)
	}
	if status := pollQR(t, client, ts.URL, "unknown"); status.Status != "expired" {
		t.Fatalf("unknown id = %+v", status)
	}
}

type qrStatusResponse struct {
	Status   string   `json:"status"`
	Nickname string   `json:"nickname"`
	Cookies  []string `json:"cookies"`
}

func pollQR(t *testing.T, client *http.Client, base, id string) qrStatusResponse {
	t.Helper()
	response, err := client.Get(base + "/api/quark/qr/" + id)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("poll status = %d", response.StatusCode)
	}
	var payload struct {
		Data qrStatusResponse `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	return payload.Data
}
