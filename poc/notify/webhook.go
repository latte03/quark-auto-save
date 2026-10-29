package notify

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"
)

var bodyKeyLine = regexp.MustCompile(`^(\w+):\s*(.*)$`)

func parseHeaders(raw string) map[string]string {
	headers := map[string]string{}
	for _, line := range strings.Split(raw, "\n") {
		index := strings.Index(line, ":")
		if index < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:index]))
		value := strings.TrimSpace(line[index+1:])
		if existing, ok := headers[key]; ok {
			headers[key] = existing + ", " + value
		} else {
			headers[key] = value
		}
	}
	return headers
}

func parseStringBody(input string, format func(string) string) map[string]any {
	parsed := map[string]any{}
	var lastKey string
	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSuffix(line, "\r")
		if match := bodyKeyLine.FindStringSubmatch(line); match != nil {
			key := match[1]
			value := match[2]
			if format != nil {
				value = format(value)
			}
			parsed[key] = decodeValue(value)
			lastKey = key
			continue
		}
		if lastKey != "" {
			value := line
			if format != nil {
				value = format(value)
			}
			parsed[lastKey] = fmt.Sprint(parsed[lastKey]) + "\n" + value
		}
	}
	return parsed
}

func decodeValue(raw string) any {
	var decoded any
	if err := json.Unmarshal([]byte(raw), &decoded); err == nil {
		return decoded
	}
	return raw
}

func encodeForm(values map[string]any) string {
	form := url.Values{}
	for key, value := range values {
		switch typed := value.(type) {
		case []any:
			for _, item := range typed {
				form.Add(key, fmt.Sprint(item))
			}
		default:
			form.Set(key, fmt.Sprint(value))
		}
	}
	return form.Encode()
}

func parseBody(body, contentType string, format func(string) string) string {
	if body == "" {
		return ""
	}
	kind := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	if kind == "" || kind == "text/plain" {
		if format != nil {
			return format(body)
		}
		return body
	}
	parsed := parseStringBody(body, format)
	if kind == "application/json" {
		encoded, err := json.Marshal(parsed)
		if err != nil {
			return body
		}
		return string(encoded)
	}
	return encodeForm(parsed)
}

func webhook(s *Sender, ctx context.Context, cfg Config, title, content string) error {
	rawURL := cfg.Str("WEBHOOK_URL")
	bodyTemplate := cfg.Str("WEBHOOK_BODY")
	if !strings.Contains(rawURL, "$title") && !strings.Contains(bodyTemplate, "$title") {
		return fmt.Errorf("请求地址或请求体中必须包含 $title")
	}
	escape := func(value string) string { return strings.ReplaceAll(value, "\n", `\n`) }
	format := func(value string) string {
		value = strings.ReplaceAll(value, "$title", escape(title))
		return strings.ReplaceAll(value, "$content", escape(content))
	}

	headers := parseHeaders(cfg.Str("WEBHOOK_HEADERS"))
	contentType := cfg.Str("WEBHOOK_CONTENT_TYPE")
	if contentType != "" {
		if _, ok := headers["content-type"]; !ok {
			headers["content-type"] = contentType
		}
	}
	body := parseBody(bodyTemplate, contentType, format)
	formattedURL := strings.ReplaceAll(rawURL, "$title", url.QueryEscape(title))
	formattedURL = strings.ReplaceAll(formattedURL, "$content", url.QueryEscape(content))

	method := strings.ToUpper(cfg.Str("WEBHOOK_METHOD"))
	var payload []byte
	if body != "" {
		payload = []byte(body)
	}
	status, raw, err := s.request(ctx, method, formattedURL, headers, payload)
	if err != nil {
		return err
	}
	if statusOK(status) {
		return nil
	}
	return fmt.Errorf("HTTP %d %s", status, snippet(raw))
}
