// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string

	GeminiAPIKey             string
	GeminiIntentModel        string
	GeminiAnswerModel        string
	GeminiEmbeddingModel     string
	GeminiEmbeddingDimension int

	HTTPAddr string
	LogLevel string

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
		LogLevel: getEnvDefault("LOG_LEVEL", "info"),

		AdminToken: os.Getenv("ADMIN_TOKEN"),
	}

	var err error
	if cfg.RRFK, err = getEnvIntDefault("RRF_K", 60); err != nil {
		return Config{}, err
	}
	if cfg.RetrievalTopK, err = getEnvIntDefault("RETRIEVAL_TOP_K", 10); err != nil {
		return Config{}, err
	}
	if cfg.FinalTopN, err = getEnvIntDefault("FINAL_TOP_N", 5); err != nil {
		return Config{}, err
	}
	if cfg.GeminiEmbeddingDimension, err = getEnvIntDefault("GEMINI_EMBEDDING_DIMENSION", 1536); err != nil {
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
