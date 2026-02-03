package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

type Config struct {
	DBPath            string
	JWTSecret         []byte
	ServerPort        string
	GoogleOAuthConfig *oauth2.Config
}

func LoadConfig() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("Warning: .env file not found, using environment variables")
	}

	config := &Config{
		DBPath:     getEnv("DB_PATH", "./sqlite.db"),
		JWTSecret:  []byte(getEnv("JWT_SECRET", "CHANGE_THIS_SECRET")),
		ServerPort: getEnv("SERVER_PORT", "8080"),
	}

	config.GoogleOAuthConfig = &oauth2.Config{
		ClientID:     getEnv("GOOGLE_CLIENT_ID", ""),
		ClientSecret: getEnv("GOOGLE_CLIENT_SECRET", ""),
		RedirectURL:  getEnv("GOOGLE_REDIRECT_URL", "http://localhost:8080/callback"),
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			"https://www.googleapis.com/auth/userinfo.profile",
		},
		Endpoint: google.Endpoint,
	}

	return config
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}