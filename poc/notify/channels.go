package notify

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func nowMillis() int64 { return time.Now().UnixMilli() }

func bark(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	base := cfg.Str("BARK_PUSH")
	rawURL := base
	if !strings.HasPrefix(base, "http") {
		rawURL = "https://api.day.app/" + base
	}
	payload := map[string]any{"title": title, "body": content}
	optional := map[string]string{
		"BARK_ARCHIVE": "isArchive",
		"BARK_GROUP":   "group",
		"BARK_SOUND":   "sound",
		"BARK_ICON":    "icon",
		"BARK_LEVEL":   "level",
		"BARK_URL":     "url",
	}
	for key, field := range optional {
		if value := cfg.Str(key); value != "" {
			payload[field] = value
		}
	}
	status, decoded, raw, err := s.postJSON(ctx, rawURL, payload, nil)
	if err != nil {
		return err
	}
	if code, ok := decoded["code"].(float64); ok && code == 200 {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func console(s *Sender, _ context.Context, cfg Config, title, content string) error {
	if s.Out == nil {
		return nil
	}
	if strings.EqualFold(cfg.Str("CONSOLE"), "false") {
		return nil
	}
	_, err := fmt.Fprintf(s.Out, "%s\n\n%s\n", title, content)
	return err
}

func dingtalkSign(secret, timestamp string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(timestamp + "\n" + secret))
	return url.QueryEscape(base64.StdEncoding.EncodeToString(mac.Sum(nil)))
}

func dingtalk(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	timestamp := strconv.FormatInt(nowMillis(), 10)
	sign := dingtalkSign(cfg.Str("DD_BOT_SECRET"), timestamp)
	rawURL := fmt.Sprintf("https://oapi.dingtalk.com/robot/send?access_token=%s&timestamp=%s&sign=%s",
		url.QueryEscape(cfg.Str("DD_BOT_TOKEN")), timestamp, sign)
	payload := map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": title + "\n\n" + content},
	}
	_, decoded, raw, err := s.postJSON(ctx, rawURL, payload, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["errcode"]) {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["errmsg"], snippet(raw))
}

func feishu(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := "https://open.feishu.cn/open-apis/bot/v2/hook/" + cfg.Str("FSKEY")
	payload := map[string]any{
		"msg_type": "text",
		"content":  map[string]string{"text": title + "\n\n" + content},
	}
	_, decoded, raw, err := s.postJSON(ctx, rawURL, payload, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["StatusCode"]) || isOK(decoded["code"]) {
		return nil
	}
	return fmt.Errorf("%s", snippet(raw))
}

func gocqhttp(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	base := cfg.Str("GOBOT_URL")
	params := url.Values{
		"access_token": {cfg.Str("GOBOT_TOKEN")},
		"message":      {"标题:" + title + "\n内容:" + content},
	}
	target := cfg.Str("GOBOT_QQ")
	if key, value, ok := strings.Cut(target, "="); ok {
		params.Set(strings.TrimSpace(key), strings.TrimSpace(value))
	} else {
		params.Set("user_id", target)
	}
	rawURL := base
	if strings.Contains(base, "?") {
		rawURL += "&" + params.Encode()
	} else {
		rawURL += "?" + params.Encode()
	}
	status, raw, err := s.request(ctx, http.MethodGet, rawURL, nil, nil)
	if err != nil {
		return err
	}
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if decoded["status"] == "ok" {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func gotify(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := strings.TrimRight(cfg.Str("GOTIFY_URL"), "/") + "/message?token=" + url.QueryEscape(cfg.Str("GOTIFY_TOKEN"))
	priority := cfg.Int("GOTIFY_PRIORITY")
	form := url.Values{
		"title":    {title},
		"message":  {content},
		"priority": {strconv.Itoa(priority)},
	}
	_, decoded, raw, err := s.postForm(ctx, rawURL, form, nil)
	if err != nil {
		return err
	}
	if _, ok := decoded["id"]; ok {
		return nil
	}
	return fmt.Errorf("%s", snippet(raw))
}

func igot(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := "https://push.hellyw.com/" + cfg.Str("IGOT_PUSH_KEY")
	form := url.Values{"title": {title}, "content": {content}}
	_, decoded, raw, err := s.postForm(ctx, rawURL, form, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["ret"]) {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["errMsg"], snippet(raw))
}

var serverChanKey = regexp.MustCompile(`^sctp(\d+)t`)

func serverChan(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	key := cfg.Str("PUSH_KEY")
	rawURL := "https://sctapi.ftqq.com/" + key + ".send"
	if match := serverChanKey.FindStringSubmatch(key); len(match) == 2 {
		rawURL = fmt.Sprintf("https://%s.push.ft07.com/send/%s.send", match[1], key)
	}
	form := url.Values{
		"text": {title},
		"desp": {strings.ReplaceAll(content, "\n", "\n\n")},
	}
	_, decoded, raw, err := s.postForm(ctx, rawURL, form, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["errno"]) || isOK(decoded["code"]) {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["message"], snippet(raw))
}

func pushDeer(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := cfg.Str("DEER_URL")
	if rawURL == "" {
		rawURL = "https://api2.pushdeer.com/message/push"
	}
	form := url.Values{
		"text":    {title},
		"desp":    {content},
		"type":    {"markdown"},
		"pushkey": {cfg.Str("DEER_KEY")},
	}
	_, decoded, raw, err := s.postForm(ctx, rawURL, form, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["code"]) {
		return nil
	}
	if content, ok := decoded["content"].(map[string]any); ok {
		if result, ok := content["result"].([]any); ok && len(result) > 0 {
			return nil
		}
	}
	return fmt.Errorf("%s", snippet(raw))
}

func synologyChat(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := cfg.Str("CHAT_URL") + cfg.Str("CHAT_TOKEN")
	payload, err := json.Marshal(map[string]string{"text": title + "\n" + content})
	if err != nil {
		return err
	}
	status, raw, err := s.request(ctx, http.MethodPost, rawURL, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}, []byte("payload="+url.QueryEscape(string(payload))))
	if err != nil {
		return err
	}
	if statusOK(status) {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func pushPlus(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	payload := map[string]any{
		"token":       cfg.Str("PUSH_PLUS_TOKEN"),
		"title":       title,
		"content":     content,
		"topic":       cfg.Str("PUSH_PLUS_USER"),
		"template":    cfg.Str("PUSH_PLUS_TEMPLATE"),
		"channel":     cfg.Str("PUSH_PLUS_CHANNEL"),
		"webhook":     cfg.Str("PUSH_PLUS_WEBHOOK"),
		"callbackUrl": cfg.Str("PUSH_PLUS_CALLBACKURL"),
		"to":          cfg.Str("PUSH_PLUS_TO"),
	}
	status, decoded, raw, err := s.postJSON(ctx, "https://www.pushplus.plus/send", payload, nil)
	if err != nil {
		return err
	}
	if code, ok := number(decoded["code"]); ok && code == 200 {
		return nil
	}
	status, decoded, raw, err = s.postJSON(ctx, "http://pushplus.hxtrip.com/send", payload, map[string]string{
		"Accept": "application/json",
	})
	if err != nil {
		return err
	}
	if code, ok := number(decoded["code"]); ok && code == 200 {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func wePlus(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	template := "txt"
	if len(content) > 800 {
		template = "html"
	}
	payload := map[string]any{
		"token":    cfg.Str("WE_PLUS_BOT_TOKEN"),
		"title":    title,
		"content":  content,
		"template": template,
		"receiver": cfg.Str("WE_PLUS_BOT_RECEIVER"),
		"version":  cfg.Str("WE_PLUS_BOT_VERSION"),
	}
	_, decoded, raw, err := s.postJSON(ctx, "https://www.weplusbot.com/send", payload, nil)
	if err != nil {
		return err
	}
	if code, ok := number(decoded["code"]); ok && code == 200 {
		return nil
	}
	return fmt.Errorf("%s", snippet(raw))
}

func qmsg(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := fmt.Sprintf("https://qmsg.zendee.cn/%s/%s", cfg.Str("QMSG_TYPE"), cfg.Str("QMSG_KEY"))
	form := url.Values{"msg": {title + "\n\n" + strings.ReplaceAll(content, "----", "-")}}
	_, decoded, raw, err := s.postForm(ctx, rawURL, form, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["code"]) {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["reason"], snippet(raw))
}

func wecomOrigin(cfg Config) string {
	if origin := strings.TrimRight(cfg.Str("QYWX_ORIGIN"), "/"); origin != "" {
		return origin
	}
	return "https://qyapi.weixin.qq.com"
}

func wecomApp(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	parts := strings.Split(cfg.Str("QYWX_AM"), ",")
	if len(parts) < 4 {
		return fmt.Errorf("QYWX_AM 需要 corpid,corpsecret,touser,agentid")
	}
	corpID := strings.TrimSpace(parts[0])
	corpSecret := strings.TrimSpace(parts[1])
	touser := strings.TrimSpace(parts[2])
	agentID := strings.TrimSpace(parts[3])
	mediaID := ""
	if len(parts) > 4 {
		mediaID = strings.TrimSpace(parts[4])
	}

	origin := wecomOrigin(cfg)
	tokenURL := fmt.Sprintf("%s/cgi-bin/gettoken?corpid=%s&corpsecret=%s",
		origin, url.QueryEscape(corpID), url.QueryEscape(corpSecret))
	status, raw, err := s.request(ctx, http.MethodPost, tokenURL, nil, nil)
	if err != nil {
		return err
	}
	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ErrMsg      string `json:"errmsg"`
	}
	if err := json.Unmarshal(raw, &tokenResp); err != nil || tokenResp.AccessToken == "" {
		return fmt.Errorf("获取 token 失败：HTTP %d %s", status, snippet(raw))
	}

	payload := map[string]any{
		"touser":  touser,
		"agentid": agentID,
		"safe":    "0",
	}
	if mediaID == "" {
		payload["msgtype"] = "text"
		payload["text"] = map[string]string{"content": title + "\n\n" + content}
	} else {
		payload["msgtype"] = "mpnews"
		payload["mpnews"] = map[string]any{
			"articles": []map[string]string{{
				"title":              title,
				"thumb_media_id":     mediaID,
				"author":             "Author",
				"content_source_url": "",
				"content":            strings.ReplaceAll(content, "\n", "<br/>"),
				"digest":             content,
				"show_cover_pic":     "0",
			}},
		}
	}
	sendURL := fmt.Sprintf("%s/cgi-bin/message/send?access_token=%s", origin, url.QueryEscape(tokenResp.AccessToken))
	_, decoded, raw, err := s.postJSON(ctx, sendURL, payload, nil)
	if err != nil {
		return err
	}
	if decoded["errmsg"] == "ok" {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["errmsg"], snippet(raw))
}

func wecomBot(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := fmt.Sprintf("%s/cgi-bin/webhook/send?key=%s", wecomOrigin(cfg), url.QueryEscape(cfg.Str("QYWX_KEY")))
	payload := map[string]any{
		"msgtype": "text",
		"text":    map[string]string{"content": title + "\n\n" + content},
	}
	_, decoded, raw, err := s.postJSON(ctx, rawURL, payload, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["errcode"]) {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["errmsg"], snippet(raw))
}

func telegram(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	host := strings.TrimRight(cfg.Str("TG_API_HOST"), "/")
	if host == "" {
		host = "https://api.telegram.org"
	}
	rawURL := fmt.Sprintf("%s/bot%s/sendMessage", host, cfg.Str("TG_BOT_TOKEN"))
	params := url.Values{
		"chat_id":                  {cfg.Str("TG_USER_ID")},
		"text":                     {title + "\n\n" + content},
		"disable_web_page_preview": {"true"},
	}
	query := rawURL + "?" + params.Encode()
	status, raw, err := s.telegramRequest(ctx, cfg, query)
	if err != nil {
		return err
	}
	var decoded map[string]any
	_ = json.Unmarshal(raw, &decoded)
	if ok, _ := decoded["ok"].(bool); ok {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func (s *Sender) telegramRequest(ctx context.Context, cfg Config, rawURL string) (int, []byte, error) {
	proxyHost := cfg.Str("TG_PROXY_HOST")
	proxyPort := cfg.Str("TG_PROXY_PORT")
	if proxyHost == "" || proxyPort == "" {
		return s.request(ctx, http.MethodPost, rawURL, nil, nil)
	}
	if auth := cfg.Str("TG_PROXY_AUTH"); auth != "" && !strings.Contains(proxyHost, "@") {
		proxyHost = auth + "@" + proxyHost
	}
	proxyURL, err := url.Parse(fmt.Sprintf("http://%s:%s", proxyHost, proxyPort))
	if err != nil {
		return 0, nil, err
	}
	client := &http.Client{
		Timeout:   defaultTimeout,
		Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, rawURL, nil)
	if err != nil {
		return 0, nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	return resp.StatusCode, raw, err
}

func aibotk(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	message := map[string]any{"type": 1, "content": "【青龙快讯】\n\n" + title + "\n" + content}
	payload := map[string]any{
		"apiKey":  cfg.Str("AIBOTK_KEY"),
		"message": message,
	}
	rawURL := "https://api-bot.aibotk.com/openapi/v1/chat/contact"
	if cfg.Str("AIBOTK_TYPE") == "room" {
		rawURL = "https://api-bot.aibotk.com/openapi/v1/chat/room"
		payload["roomName"] = cfg.Str("AIBOTK_NAME")
	} else {
		payload["name"] = cfg.Str("AIBOTK_NAME")
	}
	_, decoded, raw, err := s.postJSON(ctx, rawURL, payload, nil)
	if err != nil {
		return err
	}
	if isOK(decoded["code"]) {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["error"], snippet(raw))
}

func pushMe(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := cfg.Str("PUSHME_URL")
	if rawURL == "" {
		rawURL = "https://push.i-i.me/"
	}
	form := url.Values{
		"push_key": {cfg.Str("PUSHME_KEY")},
		"title":    {title},
		"content":  {content},
	}
	status, raw, err := s.request(ctx, http.MethodPost, rawURL, map[string]string{
		"Content-Type": "application/x-www-form-urlencoded",
	}, []byte(form.Encode()))
	if err != nil {
		return err
	}
	if statusOK(status) && strings.TrimSpace(string(raw)) == "success" {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

var (
	chronoUser  = regexp.MustCompile(`user_id=(\d+)`)
	chronoGroup = regexp.MustCompile(`group_id=(\d+)`)
)

func chronocat(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := strings.TrimRight(cfg.Str("CHRONOCAT_URL"), "/") + "/api/message/send"
	headers := map[string]string{
		"Content-Type":  "application/json",
		"Authorization": "Bearer " + cfg.Str("CHRONOCAT_TOKEN"),
	}
	targets := cfg.Str("CHRONOCAT_QQ")
	userIDs := chronoUser.FindAllStringSubmatch(targets, -1)
	groupIDs := chronoGroup.FindAllStringSubmatch(targets, -1)
	send := func(chatType int, id string) error {
		payload := map[string]any{
			"peer": map[string]any{"chatType": chatType, "peerUin": id},
			"elements": []map[string]any{{
				"elementType": 1,
				"textElement": map[string]string{"content": title + "\n\n" + content},
			}},
		}
		status, _, raw, err := s.postJSON(ctx, rawURL, payload, headers)
		if err != nil {
			return err
		}
		if !statusOK(status) {
			return fmt.Errorf("HTTP %d %s", status, snippet(raw))
		}
		return nil
	}
	for _, match := range userIDs {
		if err := send(1, match[1]); err != nil {
			return err
		}
	}
	for _, match := range groupIDs {
		if err := send(2, match[1]); err != nil {
			return err
		}
	}
	return nil
}

func dodo(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := "https://botopen.imdodo.com/api/v2/personal/message/send"
	headers := map[string]string{
		"Authorization": fmt.Sprintf("Bot %s.%s", cfg.Str("DODO_BOTID"), cfg.Str("DODO_BOTTOKEN")),
		"Content-Type":  "application/json",
		"Host":          "botopen.imdodo.com",
	}
	payload := map[string]any{
		"islandSourceId": cfg.Str("DODO_LANDSOURCEID"),
		"dodoSourceId":   cfg.Str("DODO_SOURCEID"),
		"messageType":    1,
		"messageBody":    map[string]string{"content": title + "\n\n" + content},
	}
	status, decoded, raw, err := s.postJSON(ctx, rawURL, payload, headers)
	if err != nil {
		return err
	}
	if isOK(decoded["status"]) && decoded["message"] == "success" {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func ntfy(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := strings.TrimRight(cfg.Str("NTFY_URL"), "/") + "/" + cfg.Str("NTFY_TOPIC")
	headers := map[string]string{
		"Title":    encodeRFC2047(title),
		"Priority": "3",
		"Icon":     "https://qn.whyour.cn/logo.png",
	}
	if priority := cfg.Str("NTFY_PRIORITY"); priority != "" {
		headers["Priority"] = priority
	}
	if token := cfg.Str("NTFY_TOKEN"); token != "" {
		headers["Authorization"] = "Bearer " + token
	} else if user := cfg.Str("NTFY_USERNAME"); user != "" && cfg.Str("NTFY_PASSWORD") != "" {
		headers["Authorization"] = "Basic " + base64.StdEncoding.EncodeToString([]byte(user+":"+cfg.Str("NTFY_PASSWORD")))
	}
	if actions := cfg.Str("NTFY_ACTIONS"); actions != "" {
		headers["Actions"] = encodeRFC2047(actions)
	}
	status, raw, err := s.request(ctx, http.MethodPost, rawURL, headers, []byte(content))
	if err != nil {
		return err
	}
	if statusOK(status) {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}

func encodeRFC2047(text string) string {
	return "=?utf-8?B?" + base64.StdEncoding.EncodeToString([]byte(text)) + "?="
}

func wxPusher(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	topicIDs := []int{}
	for _, part := range strings.Split(cfg.Str("WXPUSHER_TOPIC_IDS"), ";") {
		if part = strings.TrimSpace(part); part == "" {
			continue
		}
		if id, err := strconv.Atoi(part); err == nil {
			topicIDs = append(topicIDs, id)
		}
	}
	uids := []string{}
	for _, part := range strings.Split(cfg.Str("WXPUSHER_UIDS"), ";") {
		if part = strings.TrimSpace(part); part != "" {
			uids = append(uids, part)
		}
	}
	payload := map[string]any{
		"appToken":      cfg.Str("WXPUSHER_APP_TOKEN"),
		"content":       "<h1>" + title + "</h1><br/><div style='white-space: pre-wrap;'>" + content + "</div>",
		"summary":       title,
		"contentType":   2,
		"topicIds":      topicIDs,
		"uids":          uids,
		"verifyPayType": 0,
	}
	_, decoded, raw, err := s.postJSON(ctx, "https://wxpusher.zjiecode.com/api/send/message", payload, nil)
	if err != nil {
		return err
	}
	if code, ok := number(decoded["code"]); ok && code == 1000 {
		return nil
	}
	return fmt.Errorf("%v %s", decoded["msg"], snippet(raw))
}

func (s *Sender) hitokoto(ctx context.Context) string {
	client := &http.Client{Timeout: defaultTimeout}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://v1.hitokoto.cn/", nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	var decoded struct {
		Hitokoto string `json:"hitokoto"`
		From     string `json:"from"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&decoded); err != nil || decoded.Hitokoto == "" {
		return ""
	}
	return decoded.Hitokoto + "    ----" + decoded.From
}
