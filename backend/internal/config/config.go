package config

import (
	"log"
	"os"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	AppPort            string
	IsProduction       bool
	AllowedOrigins     []string
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	DatabaseURL        string
	RedisAddr          string
	RedisPassword      string
	GeminiApiKey			 string
}

func Load() *Config {
	_ = godotenv.Load()

	rawOrigins := getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")
	var origins []string
	for _, o := range strings.Split(rawOrigins, ",") {
		if trimmed := strings.TrimSpace(o); trimmed != "" {
			origins = append(origins, trimmed)
		}
	}

	return &Config{
		AppPort:            getEnv("APP_PORT", "8080"),
		IsProduction:       getEnv("APP_ENV", "development") == "production",
		AllowedOrigins:     origins,
		GoogleClientID:     mustGetEnv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: mustGetEnv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  mustGetEnv("GOOGLE_REDIRECT_URL"),
		DatabaseURL:        mustGetEnv("DATABASE_URL"),
		RedisAddr:          getEnv("REDIS_ADDR", "localhost:6379"),
		RedisPassword:      getEnv("REDIS_PASSWORD", ""),
		GeminiApiKey: 			mustGetEnv("GEMINI_API_KEY"),
	}
}

func getEnv(key, fallback string) string {
	if val, ok := os.LookupEnv(key); ok && val != "" {
		return val
	}
	return fallback
}

func mustGetEnv(key string) string {
	val, ok := os.LookupEnv(key)
	if !ok || val == "" {
		log.Fatalf("Fatal: environment variable %s is required", key)
	}
	return val
}