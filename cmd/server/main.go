// Command server runs the product search HTTP API.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fahimbagar/product-rag-search/internal/app"
	"github.com/fahimbagar/product-rag-search/internal/router"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps, err := app.Load(ctx)
	if err != nil {
		slog.New(slog.NewJSONHandler(os.Stdout, nil)).Error("server exited with error", "error", err)
		os.Exit(1)
	}
	defer deps.Close()

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: deps.Config.LogLevel}))

	if err := run(ctx, logger, deps); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, logger *slog.Logger, deps *app.Dependencies) error {
	cfg := deps.Config

	handler := router.New(logger, deps.DB, deps.Pipeline, deps.Ingester, cfg.AdminToken)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.HTTPAddr)
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
