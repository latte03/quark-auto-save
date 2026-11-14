package mcp

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
)

const maxBodyBytes = 2 << 20

// HTTPHandler serves the MCP Streamable HTTP transport (protocol 2025-06-18)
// in JSON response mode. The server is stateless: it never assigns a
// Mcp-Session-Id and offers no server-initiated SSE stream, so GET and DELETE
// answer 405. Tool calls run synchronously; a client disconnect cancels the
// request context, which stops wait-style tools.
func (s *Server) HTTPHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			w.Header().Set("Allow", http.MethodPost)
			http.Error(w, "MCP 端点仅支持 POST", http.StatusMethodNotAllowed)
			return
		}
		s.serveHTTP(w, r)
	})
}

func (s *Server) serveHTTP(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, maxBodyBytes))
	if err != nil {
		writeRPCHTTPError(w, http.StatusBadRequest, -32700, "读取请求失败")
		return
	}
	body := bytes.TrimSpace(raw)
	if len(body) == 0 {
		writeRPCHTTPError(w, http.StatusBadRequest, -32700, "请求体为空")
		return
	}

	// Batches were removed from the 2025-06-18 protocol but answering them
	// keeps older clients working, mirroring the old stdio behavior.
	if body[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(body, &batch); err != nil {
			writeRPCHTTPError(w, http.StatusBadRequest, -32700, "解析失败")
			return
		}
		responses := []json.RawMessage{}
		for _, item := range batch {
			request, err := parseRequest(item)
			if err != nil {
				responses = append(responses, encodeRPC(&rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "解析失败"}}))
				continue
			}
			if isNotification(request) {
				continue
			}
			if response := s.Handle(r.Context(), *request); response != nil {
				responses = append(responses, encodeRPC(response))
			}
		}
		if len(responses) == 0 {
			w.WriteHeader(http.StatusAccepted)
			return
		}
		writeJSONHTTP(w, http.StatusOK, responses)
		return
	}

	request, err := parseRequest(body)
	if err != nil {
		writeRPCHTTPError(w, http.StatusBadRequest, -32700, "解析失败")
		return
	}
	if isNotification(request) {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	response := s.Handle(r.Context(), *request)
	if response == nil {
		w.WriteHeader(http.StatusAccepted)
		return
	}
	writeJSONHTTP(w, http.StatusOK, response)
}

func parseRequest(raw json.RawMessage) (*rpcRequest, error) {
	var request rpcRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		return nil, err
	}
	return &request, nil
}

func isNotification(request *rpcRequest) bool {
	return len(request.ID) == 0 || string(request.ID) == "null"
}

func encodeRPC(response *rpcResponse) json.RawMessage {
	encoded, err := json.Marshal(response)
	if err != nil {
		return json.RawMessage(`{"jsonrpc":"2.0","error":{"code":-32603,"message":"响应序列化失败"}}`)
	}
	return encoded
}

func writeJSONHTTP(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeRPCHTTPError(w http.ResponseWriter, status, code int, message string) {
	writeJSONHTTP(w, status, rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: code, Message: message}})
}
