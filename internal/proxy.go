package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
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

type Post struct {
	ID    int    `json:"id"`
	Title string `json:"title"`
	Body  string `json:"body"`
}

func FetchPosts() ([]Post, error) {
	resp, err := http.Get("https://jsonplaceholder.typicode.com/posts")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, _ := ioutil.ReadAll(resp.Body)
	var posts []Post
	err = json.Unmarshal(body, &posts)
	if err != nil {
		return nil, err
	}
	return posts, nil
}

func ProxyHandler(cfg Config) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// 1️⃣ Xác định base URL (ưu tiên query param `?base=`, fallback sang config)
		base := r.URL.Query().Get("base")
		if base == "" {
			base = cfg.BaseURL
		}
		if base == "" {
			http.Error(w, "missing ?base=API_URL parameter or TARGET_BASE_URL config", http.StatusBadRequest)
			return
		}

		// 2️⃣ Validate base URL
		_, err := url.ParseRequestURI(base)
		if err != nil {
			http.Error(w, "invalid base URL", http.StatusBadRequest)
			return
		}

		// 3️⃣ Ghép target URL (bỏ prefix `/mcp`)
		target := fmt.Sprintf("%s%s", base, r.URL.Path[len("/mcp"):])

		// Loại bỏ query param `base` để không forward sang API gốc
		q := r.URL.Query()
		q.Del("base")
		if len(q) > 0 {
			target += "?" + q.Encode()
		}

		// 4️⃣ Tạo request mới tới API gốc
		req, err := http.NewRequest(r.Method, target, r.Body)
        
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		// Copy toàn bộ headers từ request gốc
		req.Header = r.Header.Clone()

		// 5️⃣ Nếu có cấu hình Auth thì auto-attach vào header
		switch cfg.AuthType {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+cfg.Token)
		case "basic":
			auth := base64.StdEncoding.EncodeToString([]byte(cfg.User + ":" + cfg.Pass))
			req.Header.Set("Authorization", "Basic "+auth)
		}

		// 6️⃣ Gửi request tới API gốc
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			respondError(w, http.StatusBadGateway, err.Error())
			log.Printf("[MCP] %s %s -> ERROR: %s", r.Method, target, err.Error())
			return
		}
		defer resp.Body.Close()

		// Đọc toàn bộ body trả về
		body, err := io.ReadAll(resp.Body)
        log.Printf("body: %v", body)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 7️⃣ Log thông tin chi tiết request/response
		duration := time.Since(start)
		log.Printf("[MCP] %s %s -> %d (%v)", r.Method, target, resp.StatusCode, duration)

		// 8️⃣ Tuỳ chế độ: bọc JSON hay giữ nguyên
		if cfg.WrapResponse {
            log.Print("Wrapping response")
			// --- Chế độ bọc JSON chuẩn hoá ---
			w.Header().Set("Content-Type", "application/json")

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				// Nếu status thành công → parse JSON để wrap
				var data interface{}
				if err := json.Unmarshal(body, &data); err != nil {
                log.Printf("❌ JSON unmarshal error: %v", err)
                log.Printf("raw body: %s", string(body))
            } else {
                log.Printf("✅ Parsed data: %#v", data)
            }
				json.NewEncoder(w).Encode(ProxyResponse{Success: true, Data: data})
			} else {
				// Nếu lỗi → wrap thành success=false + error
				json.NewEncoder(w).Encode(ProxyResponse{Success: false, Error: string(body)})
			}
			return
		}

		// --- Chế độ giữ nguyên response gốc ---
		// Forward headers y nguyên
		for k, v := range resp.Header {
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}
		// Forward status code
		w.WriteHeader(resp.StatusCode)
		// Forward body y nguyên
		_, err = w.Write(body)
		if err != nil {
			log.Printf("[MCP] error writing response: %v", err)
		}
	})
}



// Helper để trả lỗi chuẩn
func respondError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProxyResponse{Success: false, Error: msg})
}
