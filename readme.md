commit 6:59 25/09/2025
feat(mcp): add MCP stdio server & HTTP proxy handler

- Added MCP stdio server (internal/mcp.go, internal/rpc.go) with basic ping/initialize methods for Cursor compatibility
- Updated cmd/main.go to support --mode=stdio and --mode=http
- Implemented HTTP proxy in internal/proxy.go with:
  - base URL override via ?base=
  - optional auth headers (Bearer/Basic)
  - configurable wrap_response (success/data)
  - detailed request/response logging
- Updated go.mod and go.sum for jsonrpc2 dependency