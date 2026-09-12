// Command server runs the product search HTTP API.
package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fahimbagar/product-rag-search/internal/app"
	"github.com/fahimbagar/product-rag-search/internal/router"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps, err := app.Load(ctx)
	if err != nil {
		bootstrap, _ := newLogger(zapcore.InfoLevel)
		bootstrap.Errorw("server exited with error", "error", err)
		os.Exit(1)
	}
	defer deps.Close()

	logger, err := newLogger(deps.Config.LogLevel)
	if err != nil {
		os.Exit(1)
	}
	defer logger.Sync()

	if err := run(ctx, logger, deps); err != nil {
		logger.Errorw("server exited with error", "error", err)
		os.Exit(1)
	}
}

func newLogger(level zapcore.Level) (*zap.SugaredLogger, error) {
	cfg := zap.NewProductionConfig()
	cfg.OutputPaths = []string{"stdout"}
	cfg.Level = zap.NewAtomicLevelAt(level)

	logger, err := cfg.Build()
	if err != nil {
		return nil, err
	}
	return logger.Sugar(), nil
}

func run(ctx context.Context, logger *zap.SugaredLogger, deps *app.Dependencies) error {
	cfg := deps.Config

	handler := router.New(logger, deps.HealthCheck, deps.Pipeline, deps.Ingester, cfg.AdminToken)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Infow("server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		logger.Info("shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	case err := <-errCh:
		return err
	}
}
