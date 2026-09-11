// Command server runs the product search HTTP API.
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fahimbagar/product-rag-search/pkg/config"
	embedgemini "github.com/fahimbagar/product-rag-search/pkg/embeddings/gemini"
	"github.com/fahimbagar/product-rag-search/pkg/ingestion"
	llmgemini "github.com/fahimbagar/product-rag-search/pkg/llm/gemini"
	"github.com/fahimbagar/product-rag-search/pkg/pipeline"
	"github.com/fahimbagar/product-rag-search/pkg/rerank/rrf"
	"github.com/fahimbagar/product-rag-search/pkg/retrieval"
	"github.com/fahimbagar/product-rag-search/pkg/retrieval/fulltext"
	"github.com/fahimbagar/product-rag-search/pkg/retrieval/graph"
	"github.com/fahimbagar/product-rag-search/pkg/retrieval/vector"
	"github.com/fahimbagar/product-rag-search/pkg/router"
	"github.com/fahimbagar/product-rag-search/pkg/store/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	if err := run(logger); err != nil {
		logger.Error("server exited with error", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	embedder, err := embedgemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiEmbeddingModel, cfg.GeminiEmbeddingDimension)
	if err != nil {
		return err
	}
	llmClient, err := llmgemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiIntentModel, cfg.GeminiAnswerModel)
	if err != nil {
		return err
	}
	products := postgres.NewProductStore(db)
	graphStore := graph.NewStore(db)

	sources := []retrieval.Source{
		vector.NewSearcher(db),
		fulltext.NewSearcher(db),
		graphStore,
	}

	p := pipeline.New(
		llmClient,
		embedder,
		sources,
		rrf.New(cfg.RRFK),
		llmClient,
		products,
		pipeline.Config{RetrievalTopK: cfg.RetrievalTopK, FinalTopN: cfg.FinalTopN},
	)

	ingester := ingestion.NewIngester(embedder, products, graphStore)

	handler := router.New(logger, db, p, ingester, cfg.AdminToken)
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("server listening", "addr", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
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
