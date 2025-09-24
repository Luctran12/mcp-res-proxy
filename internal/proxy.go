package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"
)

type ProxyResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

func ProxyHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 1. Xác định base URL (ưu tiên query param, fallback .env)
		base := r.URL.Query().Get("base")
		if base == "" {
			base = cfg.BaseURL
		}
		if base == "" {
			http.Error(w, "missing ?base=API_URL parameter or TARGET_BASE_URL config", http.StatusBadRequest)
			return
		}

		// Validate base URL
		_, err := url.ParseRequestURI(base)
		if err != nil {
			http.Error(w, "invalid base URL", http.StatusBadRequest)
			return
		}

		// 2. Tạo target URL (bỏ prefix /mcp)
		target := fmt.Sprintf("%s%s", base, r.URL.Path[len("/mcp"):])

		// Bỏ query param `base` để không forward sang API gốc
		q := r.URL.Query()
		q.Del("base")
		if len(q) > 0 {
			target += "?" + q.Encode()
		}

		// 3. Tạo request mới
		req, err := http.NewRequest(r.Method, target, r.Body)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		req.Header = r.Header.Clone()

		// 4. Auto-attach Auth nếu có config
		switch cfg.AuthType {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+cfg.Token)
		case "basic":
			auth := base64.StdEncoding.EncodeToString([]byte(cfg.User + ":" + cfg.Pass))
			req.Header.Set("Authorization", "Basic "+auth)
		}

		// 5. Forward request
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			respondError(w, http.StatusBadGateway, err.Error())
			log.Printf("[MCP] %s %s -> ERROR: %s", r.Method, target, err.Error())
			return
		}
		defer resp.Body.Close()

		// 6. Đọc body trả về
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 7. Log trace rõ ràng
		duration := time.Since(start)
		log.Printf("[MCP] %s %s -> %d (%v)", r.Method, target, resp.StatusCode, duration)

		// 8. Chuẩn hoá output JSON
		w.Header().Set("Content-Type", "application/json")
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			var data interface{}
			_ = json.Unmarshal(body, &data)
			json.NewEncoder(w).Encode(ProxyResponse{Success: true, Data: data})
		} else {
			json.NewEncoder(w).Encode(ProxyResponse{Success: false, Error: string(body)})
		}
	})
}

// Helper để trả lỗi chuẩn
func respondError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProxyResponse{Success: false, Error: msg})
}
