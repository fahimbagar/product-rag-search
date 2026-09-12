// Package app loads configuration and instantiates the concrete dependencies
// shared by the server and seed commands.
package app

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fahimbagar/product-rag-search/internal/config"
	embedgemini "github.com/fahimbagar/product-rag-search/internal/embeddings/gemini"
	"github.com/fahimbagar/product-rag-search/internal/healthcheck"
	"github.com/fahimbagar/product-rag-search/internal/ingestion"
	llmgemini "github.com/fahimbagar/product-rag-search/internal/llm/gemini"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
	"github.com/fahimbagar/product-rag-search/internal/rerank/rrf"
	"github.com/fahimbagar/product-rag-search/internal/retrieval"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/fulltext"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/graph"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/vector"
	"github.com/fahimbagar/product-rag-search/internal/store/postgres"
)

// Dependencies holds the concrete, already-wired components shared by the
// server and seed commands: a logger, Gemini clients, stores, and the query
// pipeline and ingester built from them.
type Dependencies struct {
	Config config.Config

	Logger      *zap.SugaredLogger
	HealthCheck *healthcheck.HealthCheck
	Embedder    *embedgemini.Client
	LLM         *llmgemini.Client
	Products    *postgres.ProductStore
	Graph       *graph.Store
	Pipeline    *pipeline.Pipeline
	Ingester    *ingestion.Ingester

	db *pgxpool.Pool
}

// Load reads configuration from the environment and instantiates all
// dependencies: logger, DB pool, embedder, LLM client, stores, retrieval
// sources, query pipeline, and ingester.
func Load(ctx context.Context) (*Dependencies, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}

	logger, err := newLogger(cfg.LogLevel)
	if err != nil {
		return nil, err
	}

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL, logger)
	if err != nil {
		return nil, err
	}

	embedder, err := embedgemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiEmbeddingModel)
	if err != nil {
		db.Close()
		return nil, err
	}

	llmClient, err := llmgemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiIntentModel, cfg.GeminiAnswerModel)
	if err != nil {
		db.Close()
		return nil, err
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

	return &Dependencies{
		Config:      cfg,
		Logger:      logger,
		HealthCheck: healthcheck.New(db),
		Embedder:    embedder,
		LLM:         llmClient,
		Products:    products,
		Graph:       graphStore,
		Pipeline:    p,
		Ingester:    ingester,
		db:          db,
	}, nil
}

// Close releases resources held by Dependencies.
func (d *Dependencies) Close() {
	d.db.Close()
	_ = d.Logger.Sync()
}

// newLogger builds the app's structured logger: JSON to stdout, filtered at
// level.
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
