package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/sourcegraph/jsonrpc2"
)

// stdioReadWriteCloser wraps stdin/stdout into an io.ReadWriteCloser for jsonrpc2
type stdioReadWriteCloser struct {
	in  *os.File
	out *os.File
}

func (s *stdioReadWriteCloser) Read(p []byte) (int, error)  { return s.in.Read(p) }
func (s *stdioReadWriteCloser) Write(p []byte) (int, error) { return s.out.Write(p) }
func (s *stdioReadWriteCloser) Close() error                { return nil }

// MCPHandler implements jsonrpc2.Handler and holds config
type MCPHandler struct {
	cfg Config
}

// RunMCP starts the JSON-RPC (MCP-like) server on stdin/stdout
func RunMCP(cfg Config) {
    rwc := &stdioReadWriteCloser{in: os.Stdin, out: os.Stdout}
    stream := jsonrpc2.NewBufferedStream(rwc, jsonrpc2.VSCodeObjectCodec{})

    ctx := context.Background()
    conn := jsonrpc2.NewConn(ctx, stream, &MCPHandler{cfg: cfg})

    log.Println("🚀 MCP stdio server started (listening on stdin/stdout)")

    // Giữ kết nối, chờ cho đến khi có lỗi hoặc đóng
    <-conn.DisconnectNotify()
    log.Println("❌ MCP connection closed")
}



// Handle implements jsonrpc2.Handler
func (h *MCPHandler) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	method := req.Method

	switch method {
	case "ping":
		_ = conn.Reply(ctx, req.ID, map[string]string{"status": "ok"})
		return

	case "initialize":
		result := map[string]interface{}{
			"capabilities": map[string]bool{
				"resources": true,
				"tools":     true,
			},
			"serverInfo": map[string]string{
				"name":    "mcp-res-proxy",
				"version": "0.1.0",
			},
		}
		_ = conn.Reply(ctx, req.ID, result)
		return

	case "resource/list":
		// Simple example: report one resource if BaseURL present
		resources := []map[string]string{}
		if h.cfg.BaseURL != "" {
			resources = append(resources, map[string]string{
				"uri":  h.cfg.BaseURL,
				"name": "default",
			})
		}
		_ = conn.Reply(ctx, req.ID, resources)
		return

	case "tool/list":
		tools := []map[string]interface{}{
			{
				"name":        "api_get",
				"description": "Perform a GET request",
				"inputSchema": map[string]interface{}{
					"type": "object",
					"properties": map[string]interface{}{
						"base": map[string]string{"type": "string"},
						"path": map[string]string{"type": "string"},
						"headers": map[string]interface{}{
							"type": "object",
						},
					},
					"required": []string{"base", "path"},
				},
			},
			{
				"name":        "api_post",
				"description": "Perform a POST request",
			},
			{
				"name":        "api_put",
				"description": "Perform a PUT request",
			},
			{
				"name":        "api_delete",
				"description": "Perform a DELETE request",
			},
		}
		_ = conn.Reply(ctx, req.ID, map[string]interface{}{"tools": tools})
		return

	case "tool/run":
		// params expected like: {"name":"api_get","arguments": {...}}
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		
		if req.Params == nil {
    _ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
        Code:    jsonrpc2.CodeInvalidParams,
        Message: "missing params",
    })
    return
}

	if err := json.Unmarshal(*req.Params, &params); err != nil {
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeInvalidParams,
			Message: err.Error(),
		})
		return
	}

		res, err := h.runTool(params.Name, params.Arguments)
		if err != nil {
			_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{Code: jsonrpc2.CodeInternalError, Message: err.Error()})
			return
		}
		_ = conn.Reply(ctx, req.ID, res)
		return

	default:
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{Code: jsonrpc2.CodeMethodNotFound, Message: "Method not found"})
		return
	}
}

// runTool executes simple HTTP calls based on tool name + arguments
// Supported minimal arguments: base (string), path (string), headers (object), body (any)
func (h *MCPHandler) runTool(name string, args map[string]interface{}) (interface{}, error) {
	// Determine HTTP method from tool name
	method := "GET"
	switch strings.ToLower(name) {
	case "api_get":
		method = "GET"
	case "api_post":
		method = "POST"
	case "api_put":
		method = "PUT"
	case "api_delete":
		method = "DELETE"
	}

	// base fallback to config
	base := ""
	if b, ok := args["base"].(string); ok && b != "" {
		base = b
	} else {
		base = h.cfg.BaseURL
	}
	if base == "" {
		return nil, fmt.Errorf("missing base URL (provide in args.base or TARGET_BASE_URL)")
	}

	// path
	path := ""
	if p, ok := args["path"].(string); ok {
		path = p
	}
	// combine URL
	target := strings.TrimRight(base, "/") + "/" + strings.TrimLeft(path, "/")

	// body
	var bodyBytes []byte
	if b, ok := args["body"]; ok && b != nil {
		// if body is already a string, use it
		switch v := b.(type) {
		case string:
			bodyBytes = []byte(v)
		default:
			tmp, _ := json.Marshal(v)
			bodyBytes = tmp
		}
	}

	// build request
	req, err := http.NewRequest(method, target, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, err
	}

	// headers from args
	if hdrs, ok := args["headers"].(map[string]interface{}); ok {
		for hk, hv := range hdrs {
			req.Header.Set(hk, fmt.Sprintf("%v", hv))
		}
	}

	// attach auth from config if configured (but do not overwrite explicit header)
	switch strings.ToLower(h.cfg.AuthType) {
	case "bearer":
		if req.Header.Get("Authorization") == "" && h.cfg.Token != "" {
			req.Header.Set("Authorization", "Bearer "+h.cfg.Token)
		}
	case "basic":
		if req.Header.Get("Authorization") == "" && h.cfg.User != "" {
			token := base64StdEncode(h.cfg.User + ":" + h.cfg.Pass)
			req.Header.Set("Authorization", "Basic "+token)
		}
	}

	// default content-type if body present and not set
	if len(bodyBytes) > 0 && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return map[string]interface{}{"success": false, "error": err.Error()}, nil
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	// try parse JSON body
	var parsed interface{}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		// not JSON — return raw string
		parsed = string(respBody)
	}

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return ProxyResponse{Success: true, Data: parsed}, nil
	}
	// non-2xx
	return ProxyResponse{Success: false, Error: fmt.Sprintf("status %d: %s", resp.StatusCode, string(respBody))}, nil
}

// helper base64 encode (to avoid importing encoding/base64 at top if not present)
func base64StdEncode(s string) string {
	// simple wrapper
	return strings.TrimRight(strings.NewReplacer(
		"+", "-",
		"/", "_",
	).Replace(fmt.Sprintf("%s", s)), "=")
}
