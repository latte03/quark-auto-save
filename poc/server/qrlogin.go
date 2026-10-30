package server

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"qas-poc/quark"
)

const (
	qrTokenURL   = "https://uop.quark.cn/cas/ajax/getTokenForQrcodeLogin"
	qrTicketURL  = "https://uop.quark.cn/cas/ajax/getServiceTicketByQrcodeToken"
	qrScanURL    = "https://su.quark.cn/4_eMHBJ"
	qrAccountURL = "https://pan.quark.cn/account/info"
	qrClientID   = "532"
	qrVersion    = "1.2"
	qrBizStr     = "S:custom|OPT:SAREA@0|OPT:IMMERSIVE@1|OPT:BACK_BTN_STYLE@0"
	qrUserAgent  = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124 Safari/537.36"
	qrLifetime   = 2 * time.Minute

	qrStatusOK        = 2000000
	qrStatusWaiting   = 50004001
	qrStatusExpired   = 50004002
	qrStatusFailed    = 50004003
	qrStatusCancelled = 50004004
)

var errQRSession = errors.New("二维码会话不存在或已过期")

type qrEndpoints struct {
	token   string
	ticket  string
	scan    string
	account string
}

type qrSession struct {
	id        string
	token     string
	createdAt time.Time
}

// qrLogin drives the Quark web QR login: token → scan URL → service ticket →
// drive cookies. Only one QR is pending at a time; single-operator appliance.
type qrLogin struct {
	mu      sync.Mutex
	session *qrSession
	client  *http.Client
	logger  *log.Logger
	urls    qrEndpoints
	now     func() time.Time
	verify  func(cookie string) (string, error)
}

func newQRLogin(client *http.Client, logger *log.Logger) *qrLogin {
	return &qrLogin{
		client: client,
		logger: logger,
		urls: qrEndpoints{
			token:   qrTokenURL,
			ticket:  qrTicketURL,
			scan:    qrScanURL,
			account: qrAccountURL,
		},
		now: time.Now,
		verify: func(cookie string) (string, error) {
			info, err := quark.New(cookie).GetAccountInfo()
			if err != nil {
				return "", err
			}
			return info.Nickname, nil
		},
	}
}

type qrStart struct {
	ID        string `json:"id"`
	QRURL     string `json:"qr_url"`
	ExpiresIn int    `json:"expires_in"`
}

func (q *qrLogin) start(ctx context.Context) (qrStart, error) {
	requestID, err := randomID()
	if err != nil {
		return qrStart{}, err
	}
	endpoint := q.urls.token + "?" + url.Values{
		"client_id":  {qrClientID},
		"v":          {qrVersion},
		"request_id": {requestID},
	}.Encode()
	var payload qrResponse
	if err := q.fetchJSON(ctx, endpoint, &payload); err != nil {
		return qrStart{}, err
	}
	token := payload.Data.Members.Token
	if token == "" {
		return qrStart{}, fmt.Errorf("夸克没有返回二维码 token（status=%d）", payload.Status)
	}
	id, err := randomID()
	if err != nil {
		return qrStart{}, err
	}
	q.mu.Lock()
	q.session = &qrSession{id: id, token: token, createdAt: q.now()}
	q.mu.Unlock()
	query := url.Values{
		"token":        {token},
		"client_id":    {qrClientID},
		"ssb":          {"weblogin"},
		"uc_param_str": {""},
		"uc_biz_str":   {qrBizStr},
	}
	return qrStart{
		ID:        id,
		QRURL:     q.urls.scan + "?" + query.Encode(),
		ExpiresIn: int(qrLifetime.Seconds()),
	}, nil
}

// ticket returns the current service ticket, or errQRSession when the pending
// QR is unknown, stale, or belongs to another browser tab.
func (q *qrLogin) ticket(ctx context.Context, id string) (int, string, error) {
	q.mu.Lock()
	session := q.session
	if session == nil || session.id != id {
		// 旧标签页拿旧 id 轮询时不能顺手清掉新会话。
		q.mu.Unlock()
		return 0, "", errQRSession
	}
	if q.now().Sub(session.createdAt) > qrLifetime {
		q.session = nil
		q.mu.Unlock()
		return 0, "", errQRSession
	}
	token := session.token
	q.mu.Unlock()

	endpoint := q.urls.ticket + "?" + url.Values{
		"client_id":  {qrClientID},
		"v":          {qrVersion},
		"token":      {token},
		"request_id": {id},
	}.Encode()
	var payload qrResponse
	if err := q.fetchJSON(ctx, endpoint, &payload); err != nil {
		return 0, "", err
	}
	if payload.Status == qrStatusExpired || payload.Status == qrStatusFailed || payload.Status == qrStatusCancelled {
		q.finish()
	}
	return payload.Status, payload.Data.Members.ServiceTicket, nil
}

func (q *qrLogin) finish() {
	q.mu.Lock()
	q.session = nil
	q.mu.Unlock()
}

// redeem exchanges a service ticket for drive cookies. Quark answers with a
// redirect chain, so every hop's Set-Cookie is recorded, not just the last one.
func (q *qrLogin) redeem(ctx context.Context, ticket string) (string, string, error) {
	jar := newRecordingJar()
	client := &http.Client{
		Transport: q.client.Transport,
		Timeout:   20 * time.Second,
		Jar:       jar,
	}
	endpoint := q.urls.account + "?" + url.Values{"st": {ticket}, "lw": {"scan"}}.Encode()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", "", err
	}
	applyQRHeaders(request)
	response, err := client.Do(request)
	if err != nil {
		return "", "", fmt.Errorf("兑换登录票据失败: %w", err)
	}
	defer response.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(response.Body, 1<<20))
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return "", "", fmt.Errorf("兑换登录票据返回 HTTP %d", response.StatusCode)
	}
	cookie := jar.header()
	if cookie == "" {
		return "", "", errors.New("夸克没有下发 Cookie，请重新扫码")
	}
	nickname, err := q.verify(cookie)
	if err != nil {
		return "", "", fmt.Errorf("校验 Cookie 失败: %w", err)
	}
	return cookie, nickname, nil
}

// ---------- handlers ----------

func (s *Server) handleStartQuarkQR(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	start, err := s.qr.start(ctx)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": start})
}

func (s *Server) handleQuarkQRStatus(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	status, ticket, err := s.qr.ticket(ctx, r.PathValue("id"))
	if errors.Is(err, errQRSession) {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"status": "expired"},
		})
		return
	}
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	switch status {
	case qrStatusWaiting:
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"status": "waiting"},
		})
	case qrStatusExpired:
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"status": "expired"},
		})
	case qrStatusOK:
		s.qr.finish()
		cookie, nickname, err := s.qr.redeem(ctx, ticket)
		if err != nil {
			writeError(w, http.StatusBadGateway, err.Error())
			return
		}
		cookies := s.store.Snapshot().Cookies
		if len(cookies) == 0 {
			cookies = []string{cookie}
		} else {
			cookies[0] = cookie
		}
		raw, err := json.Marshal(cookies)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "保存 Cookie 失败")
			return
		}
		if err := s.store.Update(map[string]json.RawMessage{"cookie": raw}); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		s.logger.Printf("扫码登录成功：%s", nickname)
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data": map[string]any{
				"status":   "confirmed",
				"nickname": nickname,
				"cookies":  cookies,
			},
		})
	default:
		s.qr.finish()
		message := "扫码已取消"
		if status == qrStatusFailed {
			message = "扫码确认失败，请重新扫码"
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"status": "error", "message": message},
		})
	}
}

func (s *Server) handleQuarkAccount(w http.ResponseWriter, r *http.Request) {
	cookies := s.store.Snapshot().Cookies
	if len(cookies) == 0 {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"ok": false, "message": "还没有配置 Cookie"},
		})
		return
	}
	info, err := s.newClient(cookies[0]).GetAccountInfo()
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"success": true,
			"data":    map[string]any{"ok": false, "message": err.Error()},
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"data": map[string]any{
			"ok":         true,
			"nickname":   info.Nickname,
			"checked_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
}

type qrResponse struct {
	Status  int    `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Members struct {
			Token         string `json:"token"`
			ServiceTicket string `json:"service_ticket"`
		} `json:"members"`
	} `json:"data"`
}

func (q *qrLogin) fetchJSON(ctx context.Context, endpoint string, out *qrResponse) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return err
	}
	applyQRHeaders(request)
	response, err := q.client.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return err
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("夸克登录接口返回 HTTP %d", response.StatusCode)
	}
	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("夸克登录接口返回异常: %w", err)
	}
	return nil
}

func applyQRHeaders(request *http.Request) {
	request.Header.Set("User-Agent", qrUserAgent)
	request.Header.Set("Accept", "application/json, text/plain, */*")
	request.Header.Set("Accept-Language", "zh-CN,zh;q=0.9")
	request.Header.Set("Referer", "https://pan.quark.cn/")
}

func randomID() (string, error) {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", err
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", raw[0:4], raw[4:6], raw[6:8], raw[8:10], raw[10:16]), nil
}

func isQuarkDomain(host string) bool {
	host = strings.TrimPrefix(strings.ToLower(host), ".")
	return host == "quark.cn" || strings.HasSuffix(host, ".quark.cn")
}

type recordingJar struct {
	base   http.CookieJar
	mu     sync.Mutex
	values map[string]string
}

func newRecordingJar() *recordingJar {
	base, _ := cookiejar.New(nil)
	return &recordingJar{base: base, values: map[string]string{}}
}

func (j *recordingJar) SetCookies(u *url.URL, cookies []*http.Cookie) {
	j.base.SetCookies(u, cookies)
	j.mu.Lock()
	defer j.mu.Unlock()
	for _, cookie := range cookies {
		if cookie == nil || cookie.Name == "" {
			continue
		}
		domain := cookie.Domain
		if domain == "" {
			domain = u.Hostname()
		}
		if !isQuarkDomain(domain) {
			continue
		}
		j.values[cookie.Name] = cookie.Value
	}
}

func (j *recordingJar) Cookies(u *url.URL) []*http.Cookie {
	return j.base.Cookies(u)
}

func (j *recordingJar) header() string {
	j.mu.Lock()
	defer j.mu.Unlock()
	names := make([]string, 0, len(j.values))
	for name := range j.values {
		names = append(names, name)
	}
	sort.Strings(names)
	pairs := make([]string, 0, len(names))
	for _, name := range names {
		pairs = append(pairs, name+"="+j.values[name])
	}
	return strings.Join(pairs, "; ")
}
