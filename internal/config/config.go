// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"log/slog"
	"os"
	"strconv"
	"strings"
)

// Config holds the service's environment-derived settings: database and
// Gemini connection info, HTTP/logging options, and retrieval/rerank tuning.
type Config struct {
	DatabaseURL string

	GeminiAPIKey         string
	GeminiIntentModel    string
	GeminiAnswerModel    string
	GeminiEmbeddingModel string

	HTTPAddr string
	LogLevel slog.Level

	RRFK          int
	RetrievalTopK int
	FinalTopN     int
	AdminToken    string
}

func Load() (Config, error) {
	cfg := Config{
		DatabaseURL: os.Getenv("DATABASE_URL"),

		GeminiAPIKey:         os.Getenv("GEMINI_API_KEY"),
		GeminiIntentModel:    getEnvDefault("GEMINI_INTENT_MODEL", "gemini-3.5-flash-lite"),
		GeminiAnswerModel:    getEnvDefault("GEMINI_ANSWER_MODEL", "gemini-3.8-flash"),
		GeminiEmbeddingModel: getEnvDefault("GEMINI_EMBEDDING_MODEL", "gemini-embedding-001"),

		HTTPAddr: getEnvDefault("HTTP_ADDR", ":8080"),

		AdminToken: os.Getenv("ADMIN_TOKEN"),
	}

	var err error
	if cfg.LogLevel, err = parseLogLevel(getEnvDefault("LOG_LEVEL", "info")); err != nil {
		return Config{}, err
	}
	if cfg.RRFK, err = getEnvIntDefault("RRF_K", 60); err != nil {
		return Config{}, err
	}
	if cfg.RetrievalTopK, err = getEnvIntDefault("RETRIEVAL_TOP_K", 10); err != nil {
		return Config{}, err
	}
	if cfg.FinalTopN, err = getEnvIntDefault("FINAL_TOP_N", 5); err != nil {
		return Config{}, err
	}

	if cfg.DatabaseURL == "" {
		return Config{}, fmt.Errorf("config: DATABASE_URL is required")
	}

	return cfg, nil
}

func getEnvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func parseLogLevel(v string) (slog.Level, error) {
	switch strings.ToLower(v) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("config: invalid LOG_LEVEL %q (want debug, info, warn, or error)", v)
	}
}

func getEnvIntDefault(key string, def int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return def, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("config: invalid int for %s: %w", key, err)
	}
	return n, nil
}
