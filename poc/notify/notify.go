package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

const defaultTimeout = 15 * time.Second

// Result is the outcome of one channel attempt.
type Result struct {
	Channel string
	Err     error
}

type Sender struct {
	HTTP *http.Client
	Out  io.Writer
}

func New(out io.Writer) *Sender {
	return &Sender{
		HTTP: &http.Client{Timeout: defaultTimeout},
		Out:  out,
	}
}

type channel struct {
	name   string
	active func(Config) bool
	send   func(*Sender, context.Context, Config, string, string) error
}

var knownKeys = []string{
	"HITOKOTO",
	"BARK_PUSH", "BARK_ARCHIVE", "BARK_GROUP", "BARK_SOUND", "BARK_ICON", "BARK_LEVEL", "BARK_URL",
	"CONSOLE",
	"DD_BOT_SECRET", "DD_BOT_TOKEN",
	"FSKEY",
	"GOBOT_URL", "GOBOT_QQ", "GOBOT_TOKEN",
	"GOTIFY_URL", "GOTIFY_TOKEN", "GOTIFY_PRIORITY",
	"IGOT_PUSH_KEY",
	"PUSH_KEY",
	"DEER_KEY", "DEER_URL",
	"CHAT_URL", "CHAT_TOKEN",
	"PUSH_PLUS_TOKEN", "PUSH_PLUS_USER", "PUSH_PLUS_TEMPLATE", "PUSH_PLUS_CHANNEL",
	"PUSH_PLUS_WEBHOOK", "PUSH_PLUS_CALLBACKURL", "PUSH_PLUS_TO",
	"WE_PLUS_BOT_TOKEN", "WE_PLUS_BOT_RECEIVER", "WE_PLUS_BOT_VERSION",
	"QMSG_KEY", "QMSG_TYPE",
	"QYWX_ORIGIN", "QYWX_AM", "QYWX_KEY",
	"TG_BOT_TOKEN", "TG_USER_ID", "TG_API_HOST",
	"TG_PROXY_AUTH", "TG_PROXY_HOST", "TG_PROXY_PORT",
	"AIBOTK_KEY", "AIBOTK_TYPE", "AIBOTK_NAME",
	"SMTP_SERVER", "SMTP_SSL", "SMTP_EMAIL", "SMTP_PASSWORD", "SMTP_NAME",
	"SMTP_EMAIL_TO", "SMTP_NAME_TO",
	"PUSHME_KEY", "PUSHME_URL",
	"CHRONOCAT_QQ", "CHRONOCAT_TOKEN", "CHRONOCAT_URL",
	"DODO_BOTTOKEN", "DODO_BOTID", "DODO_LANDSOURCEID", "DODO_SOURCEID",
	"WEBHOOK_URL", "WEBHOOK_BODY", "WEBHOOK_HEADERS", "WEBHOOK_METHOD", "WEBHOOK_CONTENT_TYPE",
	"NTFY_URL", "NTFY_TOPIC", "NTFY_PRIORITY", "NTFY_TOKEN", "NTFY_USERNAME",
	"NTFY_PASSWORD", "NTFY_ACTIONS",
	"WXPUSHER_APP_TOKEN", "WXPUSHER_TOPIC_IDS", "WXPUSHER_UIDS",
	"QUARK_SIGN_NOTIFY",
}

var registry = []channel{
	{"Bark", func(c Config) bool { return c.Str("BARK_PUSH") != "" }, bark},
	{"Console", func(c Config) bool { return c.Truthy("CONSOLE") }, console},
	{"钉钉机器人", func(c Config) bool {
		return c.Str("DD_BOT_TOKEN") != "" && c.Str("DD_BOT_SECRET") != ""
	}, dingtalk},
	{"飞书机器人", func(c Config) bool { return c.Str("FSKEY") != "" }, feishu},
	{"go-cqhttp", func(c Config) bool {
		return c.Str("GOBOT_URL") != "" && c.Str("GOBOT_QQ") != ""
	}, gocqhttp},
	{"Gotify", func(c Config) bool {
		return c.Str("GOTIFY_URL") != "" && c.Str("GOTIFY_TOKEN") != ""
	}, gotify},
	{"iGot", func(c Config) bool { return c.Str("IGOT_PUSH_KEY") != "" }, igot},
	{"Server酱", func(c Config) bool { return c.Str("PUSH_KEY") != "" }, serverChan},
	{"PushDeer", func(c Config) bool { return c.Str("DEER_KEY") != "" }, pushDeer},
	{"Synology Chat", func(c Config) bool {
		return c.Str("CHAT_URL") != "" && c.Str("CHAT_TOKEN") != ""
	}, synologyChat},
	{"PushPlus", func(c Config) bool { return c.Str("PUSH_PLUS_TOKEN") != "" }, pushPlus},
	{"微加机器人", func(c Config) bool { return c.Str("WE_PLUS_BOT_TOKEN") != "" }, wePlus},
	{"Qmsg", func(c Config) bool {
		return c.Str("QMSG_KEY") != "" && c.Str("QMSG_TYPE") != ""
	}, qmsg},
	{"企业微信应用", func(c Config) bool { return c.Str("QYWX_AM") != "" }, wecomApp},
	{"企业微信机器人", func(c Config) bool { return c.Str("QYWX_KEY") != "" }, wecomBot},
	{"Telegram", func(c Config) bool {
		return c.Str("TG_BOT_TOKEN") != "" && c.Str("TG_USER_ID") != ""
	}, telegram},
	{"智能微秘书", func(c Config) bool {
		return c.Str("AIBOTK_KEY") != "" && c.Str("AIBOTK_TYPE") != "" && c.Str("AIBOTK_NAME") != ""
	}, aibotk},
	{"SMTP", func(c Config) bool {
		return c.Str("SMTP_SERVER") != "" && c.Str("SMTP_SSL") != "" &&
			c.Str("SMTP_EMAIL") != "" && c.Str("SMTP_PASSWORD") != "" && c.Str("SMTP_NAME") != ""
	}, smtpMail},
	{"PushMe", func(c Config) bool { return c.Str("PUSHME_KEY") != "" }, pushMe},
	{"Chronocat", func(c Config) bool {
		return c.Str("CHRONOCAT_URL") != "" && c.Str("CHRONOCAT_QQ") != "" && c.Str("CHRONOCAT_TOKEN") != ""
	}, chronocat},
	{"DoDo", func(c Config) bool {
		return c.Str("DODO_BOTTOKEN") != "" && c.Str("DODO_BOTID") != "" &&
			c.Str("DODO_LANDSOURCEID") != "" && c.Str("DODO_SOURCEID") != ""
	}, dodo},
	{"自定义通知", func(c Config) bool {
		return c.Str("WEBHOOK_URL") != "" && c.Str("WEBHOOK_METHOD") != ""
	}, webhook},
	{"Ntfy", func(c Config) bool { return c.Str("NTFY_TOPIC") != "" }, ntfy},
	{"WxPusher", func(c Config) bool {
		return c.Str("WXPUSHER_APP_TOKEN") != "" &&
			(c.Str("WXPUSHER_TOPIC_IDS") != "" || c.Str("WXPUSHER_UIDS") != "")
	}, wxPusher},
}

// Channels lists the configured channel names in registry order.
func Channels(cfg Config) []string {
	var names []string
	for _, item := range registry {
		if item.active(cfg) {
			names = append(names, item.name)
		}
	}
	return names
}

// Send pushes title/content to every configured channel in parallel and
// returns one result per active channel.
func (s *Sender) Send(ctx context.Context, cfg Config, title, content string) []Result {
	if strings.TrimSpace(content) == "" {
		return nil
	}
	if skip := os.Getenv("SKIP_PUSH_TITLE"); skip != "" {
		for _, line := range strings.Split(skip, "\n") {
			if strings.TrimSpace(line) == title {
				s.logf("%s 在 SKIP_PUSH_TITLE 内，跳过推送", title)
				return nil
			}
		}
	}
	if cfg.Truthy("HITOKOTO") {
		if line := s.hitokoto(ctx); line != "" {
			content += "\n\n" + line
		}
	}

	active := make([]channel, 0, len(registry))
	for _, item := range registry {
		if item.active(cfg) {
			active = append(active, item)
		}
	}
	if len(active) == 0 {
		s.logf("无推送渠道，请检查通知变量是否正确")
		return nil
	}

	results := make([]Result, len(active))
	var wg sync.WaitGroup
	for i, item := range active {
		wg.Add(1)
		go func(index int, item channel) {
			defer wg.Done()
			if item.name != "Console" {
				s.logf("%s 服务启动", item.name)
			}
			err := item.send(s, ctx, cfg, title, content)
			results[index] = Result{Channel: item.name, Err: err}
		}(i, item)
	}
	wg.Wait()

	for _, result := range results {
		if result.Err != nil {
			s.logf("%s 推送失败：%v", result.Channel, result.Err)
			continue
		}
		s.logf("%s 推送成功", result.Channel)
	}
	return results
}

// Summary renders a compact one-line outcome for run logs.
func Summarize(results []Result) string {
	if len(results) == 0 {
		return ""
	}
	ok := 0
	for _, result := range results {
		if result.Err == nil {
			ok++
		}
	}
	return fmt.Sprintf("%d/%d 个渠道推送成功", ok, len(results))
}

func (s *Sender) logf(format string, args ...any) {
	if s.Out != nil {
		fmt.Fprintf(s.Out, format+"\n", args...)
	}
}

func (s *Sender) client() *http.Client {
	if s.HTTP != nil {
		return s.HTTP
	}
	return &http.Client{Timeout: defaultTimeout}
}

// request performs an HTTP call and returns status plus the raw body.
func (s *Sender) request(ctx context.Context, method, rawURL string, headers map[string]string, body []byte) (int, []byte, error) {
	var reader io.Reader
	if body != nil {
		reader = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, rawURL, reader)
	if err != nil {
		return 0, nil, err
	}
	for key, value := range headers {
		req.Header.Set(key, value)
	}
	resp, err := s.client().Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, raw, nil
}

func (s *Sender) postJSON(ctx context.Context, rawURL string, payload any, headers map[string]string) (int, map[string]any, []byte, error) {
	encoded, err := json.Marshal(payload)
	if err != nil {
		return 0, nil, nil, err
	}
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/json;charset=utf-8"
	}
	status, raw, err := s.request(ctx, http.MethodPost, rawURL, headers, encoded)
	if err != nil {
		return status, nil, raw, err
	}
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return status, decoded, raw, nil
}

func (s *Sender) postForm(ctx context.Context, rawURL string, form url.Values, headers map[string]string) (int, map[string]any, []byte, error) {
	if headers == nil {
		headers = map[string]string{}
	}
	if _, ok := headers["Content-Type"]; !ok {
		headers["Content-Type"] = "application/x-www-form-urlencoded"
	}
	status, raw, err := s.request(ctx, http.MethodPost, rawURL, headers, []byte(form.Encode()))
	if err != nil {
		return status, nil, raw, err
	}
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	return status, decoded, raw, nil
}

// number converts JSON numbers/strings to float64 for response checks.
func number(value any) (float64, bool) {
	switch typed := value.(type) {
	case float64:
		return typed, true
	case int:
		return float64(typed), true
	case string:
		n, err := strconv.ParseFloat(strings.TrimSpace(typed), 64)
		return n, err == nil
	case json.Number:
		n, err := typed.Float64()
		return n, err == nil
	}
	return 0, false
}

func isOK(value any) bool {
	n, ok := number(value)
	return ok && n == 0
}

func statusOK(status int) bool {
	return status >= 200 && status < 300
}

func snippet(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}
