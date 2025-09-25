package main

import (
	"flag"
	"log"
	"mcp-res-proxy/internal"
	"net/http"

	"github.com/gorilla/mux"
)

func main() {
	mode := flag.String("mode", "http", "server mode: http|stdio")
	flag.Parse()

	cfg := internal.LoadConfig()

	switch *mode {
	case "stdio":
		log.Println("🚀 MCP server running in stdio mode")
		internal.RunMCP(cfg)

	case "http":
		// HTTP mode (debug/test)
		r := mux.NewRouter()
		r.Use(internal.LoggingMiddleware)
		r.PathPrefix("/mcp/").Handler(internal.ProxyHandler(cfg))

		log.Printf("🚀 MCP server running at http://localhost:%s", cfg.Port)
		log.Printf("mode: %v",cfg.WrapResponse)
		log.Fatal(http.ListenAndServe(":"+cfg.Port, r))

	default:
		log.Fatalf("❌ Unknown mode: %s (expected http|stdio)", *mode)
	}
}
