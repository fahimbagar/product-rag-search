// Command seed loads a JSON file of products (and curated related-product
// pairs) and runs them through the ingestion pipeline: embed -> insert ->
// graph edges.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/fahimbagar/product-rag-search/internal/app"
	"github.com/fahimbagar/product-rag-search/internal/ctxlog"
	"github.com/fahimbagar/product-rag-search/internal/ingestion"
)

type seedProduct struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Brand       string         `json:"brand"`
	Price       float64        `json:"price"`
	Attributes  map[string]any `json:"attributes"`
}

type seedRelation struct {
	FromIndex int     `json:"from_index"`
	ToIndex   int     `json:"to_index"`
	Weight    float64 `json:"weight"`
}

type seedFile struct {
	Products  []seedProduct  `json:"products"`
	Relations []seedRelation `json:"relations"`
}

func main() {
	ctx := context.Background()

	deps, err := app.Load(ctx)
	if err != nil {
		bootstrap, _ := newLogger(zapcore.InfoLevel)
		bootstrap.Errorw("seed failed", "error", err)
		os.Exit(1)
	}
	defer deps.Close()

	logger, err := newLogger(deps.Config.LogLevel)
	if err != nil {
		os.Exit(1)
	}
	defer logger.Sync()
	ctx = ctxlog.WithLogger(ctx, logger)

	if err := run(ctx, logger, deps); err != nil {
		logger.Errorw("seed failed", "error", err)
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
	file := flag.String("file", "seed/products.json", "path to seed JSON file")
	flag.Parse()

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	var sf seedFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("parse seed file: %w", err)
	}

	raw := make([]ingestion.RawProduct, len(sf.Products))
	for i, p := range sf.Products {
		raw[i] = ingestion.RawProduct{
			Title:       p.Title,
			Description: p.Description,
			Category:    p.Category,
			Brand:       p.Brand,
			Price:       p.Price,
			Attributes:  p.Attributes,
		}
	}
	related := make([]ingestion.RelatedPair, len(sf.Relations))
	for i, r := range sf.Relations {
		related[i] = ingestion.RelatedPair{FromIndex: r.FromIndex, ToIndex: r.ToIndex, Weight: r.Weight}
	}

	if err := deps.Ingester.Ingest(ctx, raw, related); err != nil {
		return fmt.Errorf("ingest: %w", err)
	}

	logger.Infow("seed complete", "products", len(raw), "relations", len(related))
	return nil
}
