package internal

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSON-RPC request/response struct
type RPCRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type RPCResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *RPCError   `json:"error,omitempty"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// Tool định nghĩa trong tools/list
type Tool struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}


func RunMCPA(cfg Config) {
	reader := bufio.NewReader(os.Stdin)
	encoder := json.NewEncoder(os.Stdout)

	for {
		line, err := reader.ReadBytes('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, "Read error:", err)
			continue
		}

		var req RPCRequest
		if err := json.Unmarshal(line, &req); err != nil {
			fmt.Fprintln(os.Stderr, "JSON parse error:", err)
			continue
		}

		var resp RPCResponse
		resp.JSONRPC = "2.0"
		resp.ID = req.ID

		switch req.Method {
		case "tools/list":
			resp.Result = map[string]interface{}{
				"tools": []Tool{
					{
						Name:        "api_get",
						Description: "GET data from target API",
						InputSchema: map[string]interface{}{
							"type": "object",
							"properties": map[string]interface{}{
								"base": map[string]string{"type": "string"},
								"path": map[string]string{"type": "string"},
							},
							"required": []string{"base", "path"},
						},
					},
				},
			}

		default:
			resp.Error = &RPCError{Code: -32601, Message: "Method not found"}
		}

		_ = encoder.Encode(resp)
	}
}
