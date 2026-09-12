// Package ctxlog carries a logger on a context.Context, so a request-scoped
// logger can reach handlers without being threaded through every function
// signature in the call chain.
package ctxlog

import (
	"context"
	"log/slog"
)

type contextKey struct{}

// WithLogger returns a copy of ctx carrying logger.
func WithLogger(ctx context.Context, logger *slog.Logger) context.Context {
	return context.WithValue(ctx, contextKey{}, logger)
}

// FromContext returns the logger carried by ctx, or slog.Default() if ctx
// carries none.
func FromContext(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(contextKey{}).(*slog.Logger); ok {
		return logger
	}
	return slog.Default()
}
