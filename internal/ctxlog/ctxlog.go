// Package ctxlog carries a logger on a context.Context, so a request-scoped
// logger can reach handlers without being threaded through every function
// signature in the call chain.
package ctxlog

import (
	"context"

	"go.uber.org/zap"
)

type contextKey struct{}

// WithLogger returns a copy of ctx carrying logger.
func WithLogger(ctx context.Context, logger *zap.SugaredLogger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger carried by ctx, or a no-op logger if ctx
// carries none.
func FromContext(ctx context.Context) *zap.SugaredLogger {
	if logger, ok := ctx.Value(contextKey{}).(*zap.SugaredLogger); ok {
		return logger
	}
	return zap.NewNop().Sugar()
}
