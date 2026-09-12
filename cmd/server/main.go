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

	"github.com/fahimbagar/product-rag-search/internal/app"
	"github.com/fahimbagar/product-rag-search/internal/router"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	deps, err := app.Load(ctx)
	if err != nil {
		bootstrap, _ := zap.NewProduction()
		bootstrap.Sugar().Errorw("server exited with error", "error", err)
		os.Exit(1)
	}
	defer deps.Close()

	if err := run(ctx, deps.Logger, deps); err != nil {
		deps.Logger.Errorw("server exited with error", "error", err)
		os.Exit(1)
	}
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
