package postgres

import (
	"context"

	"github.com/jackc/pgx/v5/tracelog"
	"go.uber.org/zap"
)

// zapTraceLogger adapts a *zap.SugaredLogger to pgx's tracelog.Logger, so
// pgx's own query tracing (SQL, args, duration) flows through the app's
// structured logger. Routine query events log at debug, so they're silent
// unless LOG_LEVEL=debug; warnings and errors always come through.
type zapTraceLogger struct {
	logger *zap.SugaredLogger
}

func (l zapTraceLogger) Log(_ context.Context, level tracelog.LogLevel, msg string, data map[string]any) {
	args := make([]any, 0, len(data)*2)
	for k, v := range data {
		args = append(args, k, v)
	}

	switch level {
	case tracelog.LogLevelError:
		l.logger.Errorw(msg, args...)
	case tracelog.LogLevelWarn:
		l.logger.Warnw(msg, args...)
	default:
		l.logger.Debugw(msg, args...)
	}
}
