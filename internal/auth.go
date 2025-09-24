package internal

import "encoding/base64"

func GetAuthHeader(cfg Config) map[string]string {
	headers := map[string]string{}
	switch cfg.AuthType {
	case "bearer":
		headers["Authorization"] = "Bearer " + cfg.Token
	case "basic":
		token := base64.StdEncoding.EncodeToString([]byte(cfg.User + ":" + cfg.Pass))
		headers["Authorization"] = "Basic " + token
	}
	return headers
}
