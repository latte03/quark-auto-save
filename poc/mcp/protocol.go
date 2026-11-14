package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strings"
)

const (
	serverName      = "qas"
	protocolVersion = "2025-06-18"
)

// supportedVersions lists the protocol revisions this server can speak;
// negotiate falls back to protocolVersion for anything else.
var supportedVersions = map[string]bool{
	"2025-06-18": true,
	"2025-03-26": true,
	"2024-11-05": true,
}

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

	tools     []*Tool
	toolIndex map[string]*Tool
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
		toolIndex: map[string]*Tool{},
	}
	server.tools = server.buildTools()
	for _, tool := range server.tools {
		server.toolIndex[tool.Name] = tool
	}
	return server
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
	if err := json.Unmarshal(raw, &params); err == nil && supportedVersions[params.ProtocolVersion] {
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
