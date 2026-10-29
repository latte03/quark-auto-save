package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
	"sync"
)

const (
	serverName      = "qas"
	protocolVersion = "2025-06-18"
)

type rpcRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

type contentItem struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type toolResult struct {
	Content []contentItem `json:"content"`
	IsError bool          `json:"isError,omitempty"`
}

type Tool struct {
	Name        string
	Description string
	Schema      map[string]any
	Write       bool
	Destructive bool
	Handler     func(ctx context.Context, args map[string]any) (any, error)
}

type Options struct {
	Version  string
	ReadOnly bool
	Logger   *log.Logger
}

type Server struct {
	api      *Client
	version  string
	readOnly bool
	logger   *log.Logger
	out      io.Writer

	tools     []*Tool
	toolIndex map[string]*Tool
	outMu     sync.Mutex
	wg        sync.WaitGroup

	cancelMu sync.Mutex
	cancels  map[string]context.CancelFunc
}

func New(api *Client, opts Options) *Server {
	version := opts.Version
	if version == "" {
		version = "0.1.0-poc"
	}
	logger := opts.Logger
	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}
	server := &Server{
		api:       api,
		version:   version,
		readOnly:  opts.ReadOnly,
		logger:    logger,
		out:       io.Discard,
		toolIndex: map[string]*Tool{},
		cancels:   map[string]context.CancelFunc{},
	}
	server.tools = server.buildTools()
	for _, tool := range server.tools {
		server.toolIndex[tool.Name] = tool
	}
	return server
}

// Serve speaks newline-delimited JSON-RPC over the given reader and writes
// responses to out. Only protocol traffic goes to stdout; logs go to Logger.
func (s *Server) Serve(ctx context.Context, in io.Reader, out io.Writer) error {
	s.out = out
	decoder := json.NewDecoder(in)
	for {
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			if err == io.EOF {
				s.wg.Wait()
				return nil
			}
			return err
		}
		s.dispatch(ctx, raw)
	}
}

func (s *Server) dispatch(ctx context.Context, raw json.RawMessage) {
	trimmed := bytes.TrimSpace(raw)
	if len(trimmed) == 0 {
		return
	}
	if trimmed[0] == '[' {
		var batch []json.RawMessage
		if err := json.Unmarshal(trimmed, &batch); err != nil {
			s.write(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "解析失败"}})
			return
		}
		for _, item := range batch {
			s.dispatchOne(ctx, item)
		}
		return
	}
	s.dispatchOne(ctx, trimmed)
}

func (s *Server) dispatchOne(ctx context.Context, raw json.RawMessage) {
	var request rpcRequest
	if err := json.Unmarshal(raw, &request); err != nil {
		s.write(rpcResponse{JSONRPC: "2.0", Error: &rpcError{Code: -32700, Message: "解析失败"}})
		return
	}
	if request.Method == "" {
		s.write(rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{Code: -32600, Message: "无效请求"}})
		return
	}
	if len(request.ID) == 0 || string(request.ID) == "null" {
		s.handleNotification(request)
		return
	}

	requestCtx, cancel := context.WithCancel(ctx)
	key := string(request.ID)
	s.cancelMu.Lock()
	s.cancels[key] = cancel
	s.cancelMu.Unlock()

	s.wg.Add(1)
	go func() {
		defer func() {
			s.wg.Done()
			cancel()
			s.cancelMu.Lock()
			delete(s.cancels, key)
			s.cancelMu.Unlock()
		}()
		if response := s.Handle(requestCtx, request); response != nil {
			s.write(response)
		}
	}()
}

func (s *Server) handleNotification(request rpcRequest) {
	if request.Method != "notifications/cancelled" {
		return
	}
	var params struct {
		RequestID json.RawMessage `json:"requestId"`
	}
	if err := json.Unmarshal(request.Params, &params); err != nil {
		return
	}
	s.cancelMu.Lock()
	cancel := s.cancels[string(params.RequestID)]
	s.cancelMu.Unlock()
	if cancel != nil {
		cancel()
	}
}

// Handle executes one request synchronously; Serve calls it from a goroutine.
func (s *Server) Handle(ctx context.Context, request rpcRequest) *rpcResponse {
	response := func(result any) *rpcResponse {
		return &rpcResponse{JSONRPC: "2.0", ID: request.ID, Result: result}
	}
	fail := func(code int, message string) *rpcResponse {
		return &rpcResponse{JSONRPC: "2.0", ID: request.ID, Error: &rpcError{Code: code, Message: message}}
	}

	switch request.Method {
	case "initialize":
		return response(map[string]any{
			"protocolVersion": s.negotiate(request.Params),
			"capabilities":    map[string]any{"tools": map[string]any{}},
			"serverInfo":      map[string]any{"name": serverName, "version": s.version},
			"instructions": "QAS（夸克自动转存）控制台。读工具可直接调用；写入网盘（run/write=true）" +
				"与删除文件/任务需要显式传入 confirm=true。Cookie 与令牌永远不会返回。",
		})
	case "ping":
		return response(map[string]any{})
	case "tools/list":
		return response(map[string]any{"tools": s.toolList()})
	case "tools/call":
		var params struct {
			Name      string         `json:"name"`
			Arguments map[string]any `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil {
			return fail(-32602, "工具参数格式错误")
		}
		tool := s.toolIndex[params.Name]
		if tool == nil {
			return fail(-32602, "未知工具: "+params.Name)
		}
		if s.readOnly && tool.Write {
			return response(errorResult("当前 MCP 以只读模式运行，写工具已禁用"))
		}
		if params.Arguments == nil {
			params.Arguments = map[string]any{}
		}
		result, err := tool.Handler(ctx, params.Arguments)
		if err != nil {
			s.logger.Printf("工具 %s 失败: %v", tool.Name, err)
			return response(errorResult(err.Error()))
		}
		return response(successResult(result))
	case "resources/list":
		return response(map[string]any{"resources": []any{}})
	case "prompts/list":
		return response(map[string]any{"prompts": []any{}})
	case "logging/setLevel":
		return response(map[string]any{})
	default:
		return fail(-32601, "不支持的方法: "+request.Method)
	}
}

func (s *Server) negotiate(raw json.RawMessage) string {
	var params struct {
		ProtocolVersion string `json:"protocolVersion"`
	}
	if err := json.Unmarshal(raw, &params); err == nil && params.ProtocolVersion != "" {
		return params.ProtocolVersion
	}
	return protocolVersion
}

func (s *Server) toolList() []map[string]any {
	list := make([]map[string]any, 0, len(s.tools))
	for _, tool := range s.tools {
		if s.readOnly && tool.Write {
			continue
		}
		list = append(list, map[string]any{
			"name":        tool.Name,
			"description": tool.Description,
			"inputSchema": tool.Schema,
			"annotations": map[string]any{
				"readOnlyHint":    !tool.Write,
				"destructiveHint": tool.Destructive,
				"openWorldHint":   true,
			},
		})
	}
	return list
}

func successResult(value any) toolResult {
	text := ""
	switch typed := value.(type) {
	case nil:
		text = "OK"
	case string:
		text = typed
	default:
		encoded, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			text = fmt.Sprint(value)
		} else {
			text = string(encoded)
		}
	}
	return toolResult{Content: []contentItem{{Type: "text", Text: text}}}
}

func errorResult(message string) toolResult {
	message = strings.TrimSpace(message)
	if message == "" {
		message = "未知错误"
	}
	return toolResult{Content: []contentItem{{Type: "text", Text: message}}, IsError: true}
}

func (s *Server) write(response any) {
	s.outMu.Lock()
	defer s.outMu.Unlock()
	encoder := json.NewEncoder(s.out)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(response); err != nil {
		s.logger.Printf("写响应失败: %v", err)
	}
}
