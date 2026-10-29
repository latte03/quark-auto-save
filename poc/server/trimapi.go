package server

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"
)

// The fnOS open-platform gateway is only reachable over this Unix socket, and
// only from inside the app runtime (the system injects TRIM_API_TOKEN).
const trimGatewaySocket = "/var/run/trim_open_gateway_apiscope.socket"

func trimAppName() string {
	if name := strings.TrimSpace(os.Getenv("TRIM_APPNAME")); name != "" {
		return name
	}
	return "qas"
}

func trimCall(ctx context.Context, req string, data any) (json.RawMessage, error) {
	token := strings.TrimSpace(os.Getenv("TRIM_API_TOKEN"))
	if token == "" {
		return nil, fmt.Errorf("未运行在飞牛应用中，无法查询授权目录")
	}
	payload := map[string]any{
		"reqId":   fmt.Sprintf("%d", time.Now().UnixNano()),
		"req":     req,
		"appName": trimAppName(),
		"data":    data,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	request, err := http.NewRequestWithContext(
		ctx, http.MethodPost, "http://localhost/api/v1/trimapp", bytes.NewReader(body),
	)
	if err != nil {
		return nil, err
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{
		Timeout: 5 * time.Second,
		Transport: &http.Transport{
			DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
				var dialer net.Dialer
				return dialer.DialContext(ctx, "unix", trimGatewaySocket)
			},
		},
	}
	response, err := client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("调用飞牛网关失败: %w", err)
	}
	defer response.Body.Close()

	var envelope struct {
		Code int             `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		return nil, fmt.Errorf("解析飞牛响应失败: %w", err)
	}
	if envelope.Code != 0 {
		message := envelope.Msg
		if message == "" {
			message = "未知错误"
		}
		return nil, fmt.Errorf("飞牛返回错误 %d: %s", envelope.Code, message)
	}
	return envelope.Data, nil
}

// handleLocalAuthorized lists directories the administrator granted to QAS.
func (s *Server) handleLocalAuthorized(w http.ResponseWriter, r *http.Request) {
	data, err := trimCall(r.Context(), "trim.file.getSharedAccessibleFolders", map[string]any{})
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	payload := struct {
		Paths []string `json:"paths"`
	}{Paths: []string{}}
	if err := json.Unmarshal(data, &payload); err != nil {
		writeError(w, http.StatusBadGateway, "解析授权目录失败")
		return
	}
	if payload.Paths == nil {
		payload.Paths = []string{}
	}
	writeJSON(w, http.StatusOK, map[string]any{"success": true, "data": payload})
}
