package main

import (
	"log"
	"mcp-res-proxy/internal"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	cfg := internal.LoadConfig()

	r := mux.NewRouter()
	r.Use(internal.LoggingMiddleware)

	// Truyền cfg vào ProxyHandler
	r.PathPrefix("/mcp/").Handler(internal.ProxyHandler(cfg))

	log.Printf("🚀 MCP server running at http://localhost:%s", cfg.Port)
	log.Fatal(http.ListenAndServe(":"+cfg.Port, r))
}
