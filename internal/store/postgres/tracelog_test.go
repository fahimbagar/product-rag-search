package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/tracelog"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestZapTraceLogger_Log(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name          string
		level         tracelog.LogLevel
		expectedLevel zapcore.Level
	}{
		{name: "error level logs as error", level: tracelog.LogLevelError, expectedLevel: zapcore.ErrorLevel},
		{name: "warn level logs as warn", level: tracelog.LogLevelWarn, expectedLevel: zapcore.WarnLevel},
		{name: "info level logs as debug", level: tracelog.LogLevelInfo, expectedLevel: zapcore.DebugLevel},
		{name: "debug level logs as debug", level: tracelog.LogLevelDebug, expectedLevel: zapcore.DebugLevel},
		{name: "trace level logs as debug", level: tracelog.LogLevelTrace, expectedLevel: zapcore.DebugLevel},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			core, logs := observer.New(zapcore.DebugLevel)
			logger := zapTraceLogger{logger: zap.New(core).Sugar()}

			logger.Log(context.Background(), tt.level, "Query", map[string]any{"sql": "SELECT 1"})

			entries := logs.All()
			if assert.Len(t, entries, 1) {
				assert.Equal(t, tt.expectedLevel, entries[0].Level)
				assert.Equal(t, "Query", entries[0].Message)
				assert.Equal(t, "SELECT 1", entries[0].ContextMap()["sql"])
			}
		})
	}
}
