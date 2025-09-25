package internal

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"strings"

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

		// 1️⃣ Lấy base URL từ query param hoặc config
		base := r.URL.Query().Get("base")
		if base == "" {
			base = cfg.BaseURL
		}
		if base == "" {
			http.Error(w, "missing ?base=API_URL parameter or TARGET_BASE_URL config", http.StatusBadRequest)
			return
		}
		_, err := url.ParseRequestURI(base)
		if err != nil {
			http.Error(w, "invalid base URL", http.StatusBadRequest)
			return
		}

		// 2️⃣ Xây target URL (bỏ prefix /mcp)
		target := fmt.Sprintf("%s%s", base, r.URL.Path[len("/mcp"):])
		q := r.URL.Query()
		q.Del("base")
		if len(q) > 0 {
			target += "?" + q.Encode()
		}

		// 3️⃣ Tạo request mới tới API gốc
		req, err := http.NewRequest(r.Method, target, r.Body)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}
		req.Header = r.Header.Clone()

		// 4️⃣ Auto attach Auth nếu có config
		switch cfg.AuthType {
		case "bearer":
			req.Header.Set("Authorization", "Bearer "+cfg.Token)
		case "basic":
			auth := base64.StdEncoding.EncodeToString([]byte(cfg.User + ":" + cfg.Pass))
			req.Header.Set("Authorization", "Basic "+auth)
		}

		// 5️⃣ Gửi request tới API gốc
		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			respondError(w, http.StatusBadGateway, err.Error())
			log.Printf("[MCP] %s %s -> ERROR: %s", r.Method, target, err.Error())
			return
		}
		defer resp.Body.Close()

		// 6️⃣ Nếu Content-Encoding = gzip thì giải nén
		var reader io.Reader = resp.Body
		if resp.Header.Get("Content-Encoding") == "gzip" {
            log.Printf("mode: gzip")
			gz, err := gzip.NewReader(resp.Body)
			if err != nil {
				respondError(w, http.StatusInternalServerError, "failed to decode gzip: "+err.Error())
				return
			}
			defer gz.Close()
			reader = gz

            // ❌ Đừng forward Content-Encoding vì body đã được giải nén
            resp.Header.Del("Content-Encoding")
            resp.Header.Del("Content-Length")
		}

		// Đọc toàn bộ body
		body, err := io.ReadAll(reader)
		if err != nil {
			respondError(w, http.StatusInternalServerError, err.Error())
			return
		}

		// 7️⃣ Log request trace
		duration := time.Since(start)
		log.Printf("[MCP] %s %s -> %d (%v)", r.Method, target, resp.StatusCode, duration)

		// 8️⃣ Forward headers từ API gốc (bỏ Content-Length vì body đã đọc lại)
		for k, v := range resp.Header {
			if strings.ToLower(k) == "content-length" {
				continue
			}
			for _, vv := range v {
				w.Header().Add(k, vv)
			}
		}

		// 9️⃣ Trả response (wrap hoặc raw)
		if cfg.WrapResponse {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")

			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				var data interface{}
				if err := json.Unmarshal(body, &data); err != nil {
					respondError(w, http.StatusInternalServerError, "JSON unmarshal error: "+err.Error())
					log.Printf("❌ JSON unmarshal error: %v", err)
					return
				}

				// Log preview thay vì full body
				preview := string(body)
				if len(preview) > 200 {
					preview = preview[:200] + "..."
				}
				log.Printf("response preview: %s", preview)

				json.NewEncoder(w).Encode(ProxyResponse{Success: true, Data: data})
			} else {
				json.NewEncoder(w).Encode(ProxyResponse{Success: false, Error: string(body)})
			}
		} else {
			// giữ nguyên body gốc
			w.WriteHeader(resp.StatusCode)
			_, err = w.Write(body)
			if err != nil {
				log.Printf("[MCP] error writing response: %v", err)
			}
		}
	})
}




// Helper để trả lỗi chuẩn
func respondError(w http.ResponseWriter, status int, msg string) {
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ProxyResponse{Success: false, Error: msg})
}
