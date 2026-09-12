package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func TestParseLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		in      string
		want    zapcore.Level
		wantErr bool
	}{
		{name: "debug", in: "debug", want: zapcore.DebugLevel},
		{name: "info", in: "info", want: zapcore.InfoLevel},
		{name: "warn", in: "warn", want: zapcore.WarnLevel},
		{name: "warning", in: "warning", want: zapcore.WarnLevel},
		{name: "error", in: "error", want: zapcore.ErrorLevel},
		{name: "uppercase is normalized", in: "DEBUG", want: zapcore.DebugLevel},
		{name: "invalid", in: "verbose", wantErr: true},
		{name: "empty", in: "", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseLogLevel(tt.in)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnvDefault(t *testing.T) {
	t.Run("returns env value when set", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_KEY", "custom")
		assert.Equal(t, "custom", getEnvDefault("CONFIG_TEST_KEY", "fallback"))
	})

	t.Run("returns default when unset", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_KEY", "")
		assert.Equal(t, "fallback", getEnvDefault("CONFIG_TEST_KEY", "fallback"))
	})
}

func TestGetEnvIntDefault(t *testing.T) {
	t.Run("returns default when unset", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_INT", "")
		got, err := getEnvIntDefault("CONFIG_TEST_INT", 42)
		require.NoError(t, err)
		assert.Equal(t, 42, got)
	})

	t.Run("parses a valid int", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_INT", "7")
		got, err := getEnvIntDefault("CONFIG_TEST_INT", 42)
		require.NoError(t, err)
		assert.Equal(t, 7, got)
	})

	t.Run("errors on a non-numeric value", func(t *testing.T) {
		t.Setenv("CONFIG_TEST_INT", "not-a-number")
		_, err := getEnvIntDefault("CONFIG_TEST_INT", 42)
		assert.Error(t, err)
	})
}

// setBaseEnv sets every env var Load reads to an explicit value, so the test
// is deterministic regardless of what's in the surrounding environment.
func setBaseEnv(t *testing.T) {
	t.Helper()
	t.Setenv("DATABASE_URL", "postgres://localhost/test")
	t.Setenv("GEMINI_API_KEY", "")
	t.Setenv("GEMINI_INTENT_MODEL", "")
	t.Setenv("GEMINI_ANSWER_MODEL", "")
	t.Setenv("GEMINI_EMBEDDING_MODEL", "")
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("LOG_LEVEL", "")
	t.Setenv("RRF_K", "")
	t.Setenv("RETRIEVAL_TOP_K", "")
	t.Setenv("FINAL_TOP_N", "")
	t.Setenv("ADMIN_TOKEN", "")
}

func TestLoad_Defaults(t *testing.T) {
	setBaseEnv(t)

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "postgres://localhost/test", cfg.DatabaseURL)
	assert.Equal(t, "gemini-3.5-flash-lite", cfg.GeminiIntentModel)
	assert.Equal(t, "gemini-3.8-flash", cfg.GeminiAnswerModel)
	assert.Equal(t, "gemini-embedding-001", cfg.GeminiEmbeddingModel)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, zapcore.InfoLevel, cfg.LogLevel)
	assert.Equal(t, 60, cfg.RRFK)
	assert.Equal(t, 10, cfg.RetrievalTopK)
	assert.Equal(t, 5, cfg.FinalTopN)
	assert.Empty(t, cfg.AdminToken)
}

func TestLoad_Overrides(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("GEMINI_API_KEY", "test-key")
	t.Setenv("GEMINI_INTENT_MODEL", "custom-intent")
	t.Setenv("GEMINI_ANSWER_MODEL", "custom-answer")
	t.Setenv("GEMINI_EMBEDDING_MODEL", "custom-embedding")
	t.Setenv("HTTP_ADDR", ":9090")
	t.Setenv("LOG_LEVEL", "debug")
	t.Setenv("RRF_K", "30")
	t.Setenv("RETRIEVAL_TOP_K", "20")
	t.Setenv("FINAL_TOP_N", "3")
	t.Setenv("ADMIN_TOKEN", "secret")

	cfg, err := Load()
	require.NoError(t, err)

	assert.Equal(t, "test-key", cfg.GeminiAPIKey)
	assert.Equal(t, "custom-intent", cfg.GeminiIntentModel)
	assert.Equal(t, "custom-answer", cfg.GeminiAnswerModel)
	assert.Equal(t, "custom-embedding", cfg.GeminiEmbeddingModel)
	assert.Equal(t, ":9090", cfg.HTTPAddr)
	assert.Equal(t, zapcore.DebugLevel, cfg.LogLevel)
	assert.Equal(t, 30, cfg.RRFK)
	assert.Equal(t, 20, cfg.RetrievalTopK)
	assert.Equal(t, 3, cfg.FinalTopN)
	assert.Equal(t, "secret", cfg.AdminToken)
}

func TestLoad_MissingDatabaseURL(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("DATABASE_URL", "")

	_, err := Load()
	assert.ErrorContains(t, err, "DATABASE_URL is required")
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	setBaseEnv(t)
	t.Setenv("LOG_LEVEL", "verbose")

	_, err := Load()
	assert.ErrorContains(t, err, "invalid LOG_LEVEL")
}

func TestLoad_InvalidInts(t *testing.T) {
	tests := []struct {
		name   string
		envKey string
	}{
		{name: "RRF_K", envKey: "RRF_K"},
		{name: "RETRIEVAL_TOP_K", envKey: "RETRIEVAL_TOP_K"},
		{name: "FINAL_TOP_N", envKey: "FINAL_TOP_N"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setBaseEnv(t)
			t.Setenv(tt.envKey, "not-a-number")

			_, err := Load()
			assert.ErrorContains(t, err, "invalid int for "+tt.envKey)
		})
	}
}
