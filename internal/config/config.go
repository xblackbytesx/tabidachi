package config

import (
	"fmt"
	"os"
)

type Config struct {
	DatabaseURL    string
	SessionSecret  string
	CSRFAuthKey    string
	AppBaseURL     string
	Port           string
	SecureCookies  bool
	PexelsAPIKey   string
	UnsplashKey    string
	UploadsDir     string
}

func Load() (*Config, error) {
	uploadsDir := os.Getenv("UPLOADS_DIR")
	if uploadsDir == "" {
		uploadsDir = "data/uploads"
	}
	cfg := &Config{
		DatabaseURL:   os.Getenv("DATABASE_URL"),
		SessionSecret: os.Getenv("SESSION_SECRET"),
		CSRFAuthKey:   os.Getenv("CSRF_AUTH_KEY"),
		AppBaseURL:    os.Getenv("APP_BASE_URL"),
		Port:          os.Getenv("PORT"),
		SecureCookies: os.Getenv("SECURE_COOKIES") != "false",
		PexelsAPIKey:  os.Getenv("PEXELS_API_KEY"),
		UnsplashKey:   os.Getenv("UNSPLASH_ACCESS_KEY"),
		UploadsDir:    uploadsDir,
	}
	if cfg.DatabaseURL == "" {
		return nil, fmt.Errorf("DATABASE_URL is required")
	}
	if cfg.SessionSecret == "" {
		return nil, fmt.Errorf("SESSION_SECRET is required")
	}
	if len(cfg.SessionSecret) < 32 {
		return nil, fmt.Errorf("SESSION_SECRET must be at least 32 characters")
	}
	if cfg.CSRFAuthKey == "" {
		return nil, fmt.Errorf("CSRF_AUTH_KEY is required")
	}
	if len(cfg.CSRFAuthKey) < 32 {
		return nil, fmt.Errorf("CSRF_AUTH_KEY must be at least 32 characters")
	}
	if cfg.AppBaseURL == "" {
		cfg.AppBaseURL = "http://localhost:8080"
	}
	if cfg.Port == "" {
		cfg.Port = "8080"
	}
	return cfg, nil
}
