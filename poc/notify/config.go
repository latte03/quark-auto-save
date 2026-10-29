package notify

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config is the push_config object as stored in quark_config.json.
type Config map[string]any

func Decode(raw json.RawMessage) Config {
	cfg := Config{}
	if len(raw) > 0 {
		var decoded map[string]any
		if err := json.Unmarshal(raw, &decoded); err == nil && decoded != nil {
			cfg = decoded
		}
	}
	cfg.ApplyEnv()
	return cfg
}

// ApplyEnv lets deployment secrets override the config file, mirroring the
// way notify.py reads every known key from the environment.
func (c Config) ApplyEnv() {
	for _, key := range knownKeys {
		if value := os.Getenv(key); value != "" {
			c[key] = value
		}
	}
}

func (c Config) Str(key string) string {
	value, ok := c[key]
	if !ok || value == nil {
		return ""
	}
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case bool:
		return strconv.FormatBool(typed)
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprint(typed))
	}
}

func (c Config) Truthy(key string) bool {
	value, ok := c[key]
	if !ok || value == nil {
		return false
	}
	switch typed := value.(type) {
	case bool:
		return typed
	case float64:
		return typed != 0
	case string:
		switch strings.ToLower(strings.TrimSpace(typed)) {
		case "", "false", "0", "no", "off":
			return false
		}
		return true
	default:
		return true
	}
}

func (c Config) Int(key string) int {
	value := c.Str(key)
	if value == "" {
		return 0
	}
	if n, err := strconv.Atoi(value); err == nil {
		return n
	}
	if f, err := strconv.ParseFloat(value, 64); err == nil {
		return int(f)
	}
	return 0
}

// SignNotifyEnabled reports whether daily sign results join the push body.
func (c Config) SignNotifyEnabled() bool {
	if strings.EqualFold(os.Getenv("QUARK_SIGN_NOTIFY"), "false") {
		return false
	}
	value, ok := c["QUARK_SIGN_NOTIFY"]
	if !ok || value == nil {
		return true
	}
	return c.Truthy("QUARK_SIGN_NOTIFY")
}
