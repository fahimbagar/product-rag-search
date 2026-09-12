package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zapcore"
)

func Test_parseLogLevel(t *testing.T) {
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

// baseEnv sets every env var Load reads to an explicit value, so each test
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
		name    string
		env     map[string]string
		wantErr string
		wantCfg Config
	}{
		{
			name: "defaults",
			env:  nil,
			wantCfg: Config{
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
			wantCfg: Config{
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
			name:    "missing database url",
			env:     map[string]string{"DATABASE_URL": ""},
			wantErr: "DATABASE_URL is required",
		},
		{
			name:    "invalid log level",
			env:     map[string]string{"LOG_LEVEL": "verbose"},
			wantErr: "invalid LOG_LEVEL",
		},
		{
			name:    "invalid RRF_K",
			env:     map[string]string{"RRF_K": "not-a-number"},
			wantErr: "invalid int for RRF_K",
		},
		{
			name:    "invalid RETRIEVAL_TOP_K",
			env:     map[string]string{"RETRIEVAL_TOP_K": "not-a-number"},
			wantErr: "invalid int for RETRIEVAL_TOP_K",
		},
		{
			name:    "invalid FINAL_TOP_N",
			env:     map[string]string{"FINAL_TOP_N": "not-a-number"},
			wantErr: "invalid int for FINAL_TOP_N",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setEnv(t, tt.env)

			cfg, err := Load()

			if tt.wantErr != "" {
				assert.ErrorContains(t, err, tt.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.wantCfg, cfg)
		})
	}
}
