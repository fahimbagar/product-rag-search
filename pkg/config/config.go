// Package config loads application configuration from environment variables.
package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	DatabaseURL string

	AnthropicAPIKey   string
	ClaudeIntentModel string
	ClaudeAnswerModel string

	OpenAIAPIKey         string
	OpenAIEmbeddingModel string

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

		AnthropicAPIKey:   os.Getenv("ANTHROPIC_API_KEY"),
		ClaudeIntentModel: getEnvDefault("CLAUDE_INTENT_MODEL", "claude-haiku-4-5"),
		ClaudeAnswerModel: getEnvDefault("CLAUDE_ANSWER_MODEL", "claude-sonnet-5"),

		OpenAIAPIKey:         os.Getenv("OPENAI_API_KEY"),
		OpenAIEmbeddingModel: getEnvDefault("OPENAI_EMBEDDING_MODEL", "text-embedding-3-small"),

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
