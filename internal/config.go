package internal

import (
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port     string
	BaseURL  string // base mặc định, có thể override qua query param ?base=
	AuthType string
	Token    string
	User     string
	Pass     string
}

func LoadConfig() Config {
	_ = godotenv.Load()

	return Config{
		Port:     getEnv("PORT", "3000"),
		BaseURL:  getEnv("TARGET_BASE_URL", ""), // nếu rỗng thì user phải truyền query base
		AuthType: getEnv("AUTH_TYPE", "none"),
		Token:    os.Getenv("AUTH_TOKEN"),
		User:     os.Getenv("AUTH_USER"),
		Pass:     os.Getenv("AUTH_PASS"),
	}
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}
	return fallback
}
