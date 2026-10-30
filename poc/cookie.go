package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
)

func loadCookie(cookieFile, explicit string) (string, string, error) {
	if cookieFile == "" {
		cookieFile = os.Getenv("QUARK_COOKIE_FILE")
	}
	if cookieFile != "" {
		raw, err := os.ReadFile(cookieFile)
		if err != nil {
			return "", "", fmt.Errorf("读取 Cookie 文件失败: %w", err)
		}
		cookie, err := validateCookie(string(raw))
		return cookie, cookieFile, err
	}

	var candidates []string
	if explicit != "" {
		candidates = append(candidates, explicit)
	}
	if env := os.Getenv("CONFIG_PATH"); env != "" {
		candidates = append(candidates, env)
	}
	candidates = append(candidates,
		"./config/quark_config.json",
		"../config/quark_config.json",
		"./quark_config.json",
		"../quark_config.json",
	)
	var lastErr error
	for _, path := range candidates {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		cookie, err := readCookie(path)
		if err != nil {
			lastErr = fmt.Errorf("%s: %w", path, err)
			continue
		}
		return cookie, path, nil
	}
	if lastErr != nil {
		return "", "", lastErr
	}
	return "", "", errors.New("找不到配置文件；用 -config 指定，或设置 CONFIG_PATH")
}

func readCookie(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var doc struct {
		Cookie json.RawMessage `json:"cookie"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return "", fmt.Errorf("解析 JSON 失败: %w", err)
	}
	var list []string
	if err := json.Unmarshal(doc.Cookie, &list); err == nil {
		if len(list) > 0 {
			return validateCookie(list[0])
		}
	}
	var single string
	if err := json.Unmarshal(doc.Cookie, &single); err == nil {
		return validateCookie(single)
	}
	return "", errors.New("没有 cookie 字段")
}

func validateCookie(cookie string) (string, error) {
	cookie = strings.TrimSpace(cookie)
	if strings.Contains(cookie, "Your pan.quark.cn") ||
		(len(cookie) < 100 &&
			!strings.Contains(cookie, "__pus") &&
			!strings.Contains(cookie, "__kp") &&
			!strings.Contains(cookie, "kps=")) {
		return "", errors.New("cookie 是占位值（示例配置只有 59 字符），请换成真实 Cookie")
	}
	return cookie, nil
}
