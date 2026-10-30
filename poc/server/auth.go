package server

import (
	"crypto/hmac"
	"crypto/md5"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// ---------- auth ----------

// APIKey derives the bearer token from the WebUI credentials.
func APIKey(username, password string) string {
	sum := md5.Sum([]byte(fmt.Sprintf("token%s%s+-*/", username, password)))
	return hex.EncodeToString(sum[:])[8:24]
}

func (s *Server) apiToken(username, password string) string {
	return APIKey(username, password)
}

func (s *Server) sessionValue(username, password string) string {
	expiry := time.Now().Add(31 * 24 * time.Hour).Unix()
	payload := strconv.FormatInt(expiry, 10)
	mac := hmac.New(sha256.New, []byte("qas-session-v1|"+username+"|"+password))
	mac.Write([]byte(payload))
	return payload + "." + hex.EncodeToString(mac.Sum(nil))
}

func (s *Server) validSession(value, username, password string) bool {
	parts := strings.Split(value, ".")
	if len(parts) != 2 {
		return false
	}
	expiry, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || time.Now().Unix() > expiry {
		return false
	}
	mac := hmac.New(sha256.New, []byte("qas-session-v1|"+username+"|"+password))
	mac.Write([]byte(parts[0]))
	expected := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(expected), []byte(parts[1]))
}

func (s *Server) authenticated(r *http.Request) bool {
	username, password := s.store.WebUI()
	if username == "" || password == "" {
		return false
	}
	if token := r.URL.Query().Get("token"); token != "" {
		return subtle.ConstantTimeCompare([]byte(token), []byte(s.apiToken(username, password))) == 1
	}
	if header := r.Header.Get("Authorization"); strings.HasPrefix(header, "Bearer ") {
		if subtle.ConstantTimeCompare([]byte(strings.TrimPrefix(header, "Bearer ")), []byte(s.apiToken(username, password))) == 1 {
			return true
		}
	}
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return false
	}
	return s.validSession(cookie.Value, username, password)
}

func (s *Server) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.authenticated(r) {
			writeError(w, http.StatusUnauthorized, "未登录")
			return
		}
		next(w, r)
	}
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	username, _ := s.store.WebUI()
	auth := s.authenticated(r)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":       true,
		"authenticated": auth,
		"username":      username,
		"version":       s.opts.Version,
	})
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := decodeJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	username, password := s.store.WebUI()
	userOK := subtle.ConstantTimeCompare([]byte(request.Username), []byte(username)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(request.Password), []byte(password)) == 1
	if username == "" || password == "" || !userOK || !passOK {
		s.logger.Printf("WebUI 登录失败: 用户=%s 来自=%s", request.Username, r.RemoteAddr)
		writeError(w, http.StatusUnauthorized, "用户名或密码错误")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    s.sessionValue(username, password),
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   r.TLS != nil,
		MaxAge:   31 * 24 * 3600,
	})
	s.logger.Printf("WebUI 登录成功: 用户=%s 来自=%s", username, r.RemoteAddr)
	writeJSON(w, http.StatusOK, map[string]any{
		"success":  true,
		"token":    s.apiToken(username, password),
		"username": username,
	})
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookie,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

func (s *Server) handleVersion(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "version": s.opts.Version})
}
