package mcp

import "strings"

func secretKey(key string) bool {
	lower := strings.ToLower(key)
	for _, fragment := range []string{
		"token", "secret", "password", "passwd", "cookie", "credential", "api_key", "apikey",
	} {
		if strings.Contains(lower, fragment) {
			return true
		}
	}
	return lower == "key" || strings.HasSuffix(lower, "_key")
}

// sanitizeConfig removes credentials before a config view leaves the MCP
// server; plugin and notification sections keep their shape with *** values.
func sanitizeConfig(data map[string]any) map[string]any {
	out, _ := sanitizeValue(data).(map[string]any)
	if out == nil {
		return map[string]any{}
	}
	delete(out, "cookie")
	delete(out, "api_token")
	return out
}

func sanitizeValue(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		out := make(map[string]any, len(typed))
		for key, item := range typed {
			if secretKey(key) {
				out[key] = maskSecret(item)
				continue
			}
			out[key] = sanitizeValue(item)
		}
		return out
	case []any:
		out := make([]any, len(typed))
		for i, item := range typed {
			out[i] = sanitizeValue(item)
		}
		return out
	default:
		return value
	}
}

func maskSecret(value any) any {
	switch typed := value.(type) {
	case string:
		if typed != "" {
			return "***"
		}
	case []any:
		if len(typed) > 0 {
			return []any{"***"}
		}
	case float64:
		if typed != 0 {
			return "***"
		}
	}
	return value
}
