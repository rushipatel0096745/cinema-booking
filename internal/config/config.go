package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port               string
	GinMode            string
	AppName            string
	DbUrl              string
	DatabaseURL        string
	JWTSecret          string
	JWTExpiry          int // minutes
	RefreshExpiry      int // days
	GoogleClientID     string
	GoogleClientSecret string
	GoogleRedirectURL  string
	AppBaseURL         string
}

func Load() *Config {
	err := godotenv.Load()
	if err != nil {
		log.Println("No .env file found, using environment variables")
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:               ":" + port,
		GinMode:            getEnv("GIN_MODE", "debug"),
		AppName:            getEnv("APP_NAME", "cinemabooking"),
		DbUrl:              getEnv("DB_URL", ""),
		DatabaseURL:        os.Getenv("DATABASE_URL"),
		JWTSecret:          os.Getenv("JWT_SECRET"),
		JWTExpiry:          180, // 180(3 hr)-minute access tokens
		RefreshExpiry:      30,  // 30-day refresh tokens
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURL:  os.Getenv("GOOGLE_REDIRECT_URL"),
		AppBaseURL:         os.Getenv("APP_BASE_URL"),
	}
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
