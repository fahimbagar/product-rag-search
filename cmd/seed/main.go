// Command seed loads a JSON file of products (and curated related-product
// pairs) and runs them through the ingestion pipeline: embed -> insert ->
// graph edges.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"

	"github.com/fahimbagar/product-rag-search/pkg/config"
	"github.com/fahimbagar/product-rag-search/pkg/embeddings/openai"
	"github.com/fahimbagar/product-rag-search/pkg/ingestion"
	"github.com/fahimbagar/product-rag-search/pkg/retrieval/graph"
	"github.com/fahimbagar/product-rag-search/pkg/store/postgres"
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
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(logger); err != nil {
		logger.Error("seed failed", "error", err)
		os.Exit(1)
	}
}

func run(logger *slog.Logger) error {
	file := flag.String("file", "seed/products.json", "path to seed JSON file")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(*file)
	if err != nil {
		return fmt.Errorf("read seed file: %w", err)
	}
	var sf seedFile
	if err := json.Unmarshal(data, &sf); err != nil {
		return fmt.Errorf("parse seed file: %w", err)
	}

	ctx := context.Background()
	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer db.Close()

	embedder := openai.NewClient(cfg.OpenAIAPIKey, cfg.OpenAIEmbeddingModel)
	products := postgres.NewProductStore(db)
	graphStore := graph.NewStore(db)
	ingester := ingestion.NewIngester(embedder, products, graphStore)

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

	if err := ingester.Ingest(ctx, raw, related); err != nil {
		return fmt.Errorf("ingest: %w", err)
	}

	logger.Info("seed complete", "products", len(raw), "relations", len(related))
	return nil
}
