package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zapcore"
)

// errorContains builds an assert.ErrorAssertionFunc that checks err's
// message contains substr.
func errorContains(substr string) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, err error, args ...interface{}) bool {
		return assert.ErrorContains(t, err, substr, args...)
	}
}

func Test_parseLogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		in          string
		expected    zapcore.Level
		expectedErr assert.ErrorAssertionFunc
	}{
		{name: "debug", in: "debug", expected: zapcore.DebugLevel, expectedErr: assert.NoError},
		{name: "info", in: "info", expected: zapcore.InfoLevel, expectedErr: assert.NoError},
		{name: "warn", in: "warn", expected: zapcore.WarnLevel, expectedErr: assert.NoError},
		{name: "warning", in: "warning", expected: zapcore.WarnLevel, expectedErr: assert.NoError},
		{name: "error", in: "error", expected: zapcore.ErrorLevel, expectedErr: assert.NoError},
		{name: "uppercase is normalized", in: "DEBUG", expected: zapcore.DebugLevel, expectedErr: assert.NoError},
		{name: "invalid", in: "verbose", expectedErr: assert.Error},
		{name: "empty", in: "", expectedErr: assert.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseLogLevel(tt.in)

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}

// setEnv sets every env var Load reads to an explicit value, so each test
// case is deterministic regardless of what's in the surrounding
// environment. overrides are applied on top before Load runs.
func setEnv(t *testing.T, overrides map[string]string) {
	t.Helper()

	base := map[string]string{
		"DATABASE_URL":           "postgres://localhost/test",
		"GEMINI_API_KEY":         "",
		"GEMINI_INTENT_MODEL":    "",
		"GEMINI_ANSWER_MODEL":    "",
		"GEMINI_EMBEDDING_MODEL": "",
		"HTTP_ADDR":              "",
		"LOG_LEVEL":              "",
		"RRF_K":                  "",
		"RETRIEVAL_TOP_K":        "",
		"FINAL_TOP_N":            "",
		"ADMIN_TOKEN":            "",
	}
	for k, v := range overrides {
		base[k] = v
	}
	for k, v := range base {
		t.Setenv(k, v)
	}
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name        string
		env         map[string]string
		expectedErr assert.ErrorAssertionFunc
		expectedCfg Config
	}{
		{
			name:        "defaults",
			env:         nil,
			expectedErr: assert.NoError,
			expectedCfg: Config{
				DatabaseURL:          "postgres://localhost/test",
				GeminiIntentModel:    "gemini-3.5-flash-lite",
				GeminiAnswerModel:    "gemini-3.8-flash",
				GeminiEmbeddingModel: "gemini-embedding-001",
				HTTPAddr:             ":8080",
				LogLevel:             zapcore.InfoLevel,
				RRFK:                 60,
				RetrievalTopK:        10,
				FinalTopN:            5,
			},
		},
		{
			name: "overrides",
			env: map[string]string{
				"GEMINI_API_KEY":         "test-key",
				"GEMINI_INTENT_MODEL":    "custom-intent",
				"GEMINI_ANSWER_MODEL":    "custom-answer",
				"GEMINI_EMBEDDING_MODEL": "custom-embedding",
				"HTTP_ADDR":              ":9090",
				"LOG_LEVEL":              "debug",
				"RRF_K":                  "30",
				"RETRIEVAL_TOP_K":        "20",
				"FINAL_TOP_N":            "3",
				"ADMIN_TOKEN":            "secret",
			},
			expectedErr: assert.NoError,
			expectedCfg: Config{
				DatabaseURL:          "postgres://localhost/test",
				GeminiAPIKey:         "test-key",
				GeminiIntentModel:    "custom-intent",
				GeminiAnswerModel:    "custom-answer",
				GeminiEmbeddingModel: "custom-embedding",
				HTTPAddr:             ":9090",
				LogLevel:             zapcore.DebugLevel,
				RRFK:                 30,
				RetrievalTopK:        20,
				FinalTopN:            3,
				AdminToken:           "secret",
			},
		},
		{
			name:        "missing database url",
			env:         map[string]string{"DATABASE_URL": ""},
			expectedErr: errorContains("DATABASE_URL is required"),
		},
		{
			name:        "invalid log level",
			env:         map[string]string{"LOG_LEVEL": "verbose"},
			expectedErr: errorContains("invalid LOG_LEVEL"),
		},
		{
			name:        "invalid RRF_K",
			env:         map[string]string{"RRF_K": "not-a-number"},
			expectedErr: errorContains("invalid int for RRF_K"),
		},
		{
			name:        "invalid RETRIEVAL_TOP_K",
			env:         map[string]string{"RETRIEVAL_TOP_K": "not-a-number"},
			expectedErr: errorContains("invalid int for RETRIEVAL_TOP_K"),
		},
		{
			name:        "invalid FINAL_TOP_N",
			env:         map[string]string{"FINAL_TOP_N": "not-a-number"},
			expectedErr: errorContains("invalid int for FINAL_TOP_N"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := Load()

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expectedCfg, cfg)
			}
		})
	}
}
