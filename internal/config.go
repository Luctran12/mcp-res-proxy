package internal

import (
	"os"
)

type Config struct {
	Port         string
	BaseURL      string
	AuthType     string
	Token        string
	User         string
	Pass         string
	WrapResponse bool
}

func LoadConfig() Config {
	return Config{
		Port:         getEnv("PORT", "3000"),
		BaseURL:      getEnv("TARGET_BASE_URL", "https://jsonplaceholder.typicode.com"),
		AuthType:     getEnv("AUTH_TYPE", ""), // bearer | basic | none
		Token:        os.Getenv("AUTH_TOKEN"),
		User:         os.Getenv("AUTH_USER"),
		Pass:         os.Getenv("AUTH_PASS"),
		WrapResponse: getEnv("WRAP_RESPONSE", "true") == "true", // <-- thêm flag này
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
