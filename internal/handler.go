package internal

import (
	"context"

	"github.com/sourcegraph/jsonrpc2"
)

// MCPHandler implements jsonrpc2.Handler
type MCPHandlerA struct{}

func (h *MCPHandlerA) Handle(ctx context.Context, conn *jsonrpc2.Conn, req *jsonrpc2.Request) {
	switch req.Method {
	case "ping":
		_ = conn.Reply(ctx, req.ID, map[string]string{"status": "ok"})

	case "initialize":
		result := map[string]any{
			"capabilities": map[string]any{
				"resources": true,
				"tools":     true,
			},
			"serverInfo": map[string]string{
				"name":    "mcp-res-proxy",
				"version": "0.1.0",
			},
		}
		_ = conn.Reply(ctx, req.ID, result)

	case "resource/list":
		// TODO: lấy danh sách từ config/proxy.go
		resources := []map[string]string{
			{"uri": "https://jsonplaceholder.typicode.com/posts", "name": "posts"},
		}
		_ = conn.Reply(ctx, req.ID, resources)

	case "tool/list":
		tools := []map[string]string{
			{"name": "getPosts", "description": "Fetch posts from jsonplaceholder"},
		}
		_ = conn.Reply(ctx, req.ID, tools)

	case "tool/run":
		// gọi sang proxy.go để fetch dữ liệu API thật
		posts, err := FetchPosts()
		if err != nil {
			_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
				Code:    jsonrpc2.CodeInternalError,
				Message: err.Error(),
			})
			return
		}
		_ = conn.Reply(ctx, req.ID, posts)

	default:
		_ = conn.ReplyWithError(ctx, req.ID, &jsonrpc2.Error{
			Code:    jsonrpc2.CodeMethodNotFound,
			Message: "Method not found",
		})
	}
}
