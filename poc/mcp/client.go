package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Client calls the qaspoc HTTP API on behalf of MCP tools.
//
// In handler mode it executes requests against the console's own mux in
// memory, so embedding the MCP endpoint needs no loopback connection or port
// plumbing; tokenFn is called for every request so credential changes take
// effect immediately. In network mode it dials a base URL with a fixed token
// (kept for tests).
type Client struct {
	baseURL string
	token   string
	http    *http.Client

	handler http.Handler
	tokenFn func() string
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// NewHandlerClient returns a client that routes API calls through the given
// http.Handler (the console's mux) in-process.
func NewHandlerClient(handler http.Handler, tokenFn func() string) *Client {
	return &Client{handler: handler, tokenFn: tokenFn}
}

func (c *Client) BaseURL() string { return c.baseURL }

type envelope struct {
	Success bool            `json:"success"`
	Message string          `json:"message"`
	Data    json.RawMessage `json:"data"`
}

// apiResponse carries what decodeEnvelope needs, so both transports share the
// response handling.
type apiResponse struct {
	status int
	raw    []byte
}

func (c *Client) do(ctx context.Context, method, path string, query url.Values, body, out any) error {
	rawURL := path
	if len(query) > 0 {
		rawURL += "?" + query.Encode()
	}
	var reader io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(encoded)
	}
	response, err := c.roundTrip(ctx, method, rawURL, reader, body != nil)
	if err != nil {
		return err
	}
	return decodeEnvelope(response, out)
}

func (c *Client) roundTrip(ctx context.Context, method, rawURL string, reader io.Reader, hasBody bool) (apiResponse, error) {
	if c.handler != nil {
		request, err := http.NewRequestWithContext(ctx, method, "http://qas.internal"+rawURL, reader)
		if err != nil {
			return apiResponse{}, err
		}
		request.Header.Set("Authorization", "Bearer "+c.tokenFn())
		if hasBody {
			request.Header.Set("Content-Type", "application/json")
		}
		recorder := &memoryResponseWriter{}
		c.handler.ServeHTTP(recorder, request)
		status := recorder.code
		if status == 0 {
			status = http.StatusOK
		}
		return apiResponse{status: status, raw: recorder.body.Bytes()}, nil
	}

	request, err := http.NewRequestWithContext(ctx, method, c.baseURL+rawURL, reader)
	if err != nil {
		return apiResponse{}, err
	}
	request.Header.Set("Authorization", "Bearer "+c.token)
	if hasBody {
		request.Header.Set("Content-Type", "application/json")
	}
	response, err := c.http.Do(request)
	if err != nil {
		return apiResponse{}, fmt.Errorf("连接 QAS 控制台失败: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 8<<20))
	if err != nil {
		return apiResponse{}, err
	}
	return apiResponse{status: response.StatusCode, raw: raw}, nil
}

func decodeEnvelope(response apiResponse, out any) error {
	var payload envelope
	if err := json.Unmarshal(response.raw, &payload); err != nil {
		if response.status >= 400 {
			return fmt.Errorf("HTTP %d: %s", response.status, snippet(response.raw))
		}
		return fmt.Errorf("响应解析失败: %s", snippet(response.raw))
	}
	if response.status >= 400 || payload.Success == false {
		message := payload.Message
		if message == "" {
			message = fmt.Sprintf("HTTP %d", response.status)
		}
		return errors.New(message)
	}
	if out != nil {
		// Some endpoints (/api/run, /api/sign) return fields at the top level
		// instead of wrapping them in data.
		source := []byte(payload.Data)
		if len(source) == 0 || string(source) == "null" {
			source = response.raw
		}
		if err := json.Unmarshal(source, out); err != nil {
			return fmt.Errorf("响应解析失败: %w", err)
		}
	}
	return nil
}

// memoryResponseWriter captures a handler response without touching the
// network. It only supports the plain JSON handlers the tools call.
type memoryResponseWriter struct {
	header http.Header
	code   int
	body   bytes.Buffer
}

func (w *memoryResponseWriter) Header() http.Header {
	if w.header == nil {
		w.header = http.Header{}
	}
	return w.header
}

func (w *memoryResponseWriter) WriteHeader(code int) {
	if w.code == 0 {
		w.code = code
	}
}

func (w *memoryResponseWriter) Write(p []byte) (int, error) {
	if w.code == 0 {
		w.code = http.StatusOK
	}
	return w.body.Write(p)
}

// Flush satisfies http.Flusher; streaming handlers are never used here.
func (w *memoryResponseWriter) Flush() {}

func (c *Client) get(ctx context.Context, path string, query url.Values, out any) error {
	return c.do(ctx, http.MethodGet, path, query, nil, out)
}

func (c *Client) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, nil, body, out)
}

func (c *Client) put(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPut, path, nil, body, out)
}

func (c *Client) delete(ctx context.Context, path string, out any) error {
	return c.do(ctx, http.MethodDelete, path, nil, nil, out)
}

func snippet(raw []byte) string {
	text := strings.TrimSpace(string(raw))
	if len(text) > 240 {
		return text[:240] + "..."
	}
	return text
}
