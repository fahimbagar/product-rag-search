//go:build integration

// Package integration_tests holds offline evaluation harnesses that exercise
// the real retrieval/LLM stack (live Postgres + a real GEMINI_API_KEY), run
// via `go test -tags=integration ./eval/...` against a seeded database
// (docker compose --profile seed run --rm seed).
package integration_tests

import (
	"context"
	"os"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/fahimbagar/product-rag-search/internal/config"
	"github.com/fahimbagar/product-rag-search/internal/embeddings/gemini"
	"github.com/fahimbagar/product-rag-search/internal/rerank/rrf"
	"github.com/fahimbagar/product-rag-search/internal/retrieval"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/fulltext"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/graph"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/vector"
	"github.com/fahimbagar/product-rag-search/internal/store/postgres"
)

// retrievalGoldenSet pairs a query against seed/products.json's catalog with
// the titles of products it should surface. Kept small (<=3 expected titles
// per query) so a perfect score is achievable within the default
// FINAL_TOP_N=5. Product IDs are regenerated on every seed run, so matching
// is done by title, not ID.
var retrievalGoldenSet = []struct {
	query          string
	expectedTitles []string
}{
	{"Aero brand running shoes", []string{"Aero Runner", "Aero Sprint"}},
	{"TrailForge hiking boots", []string{"TrailForge Trekker", "TrailForge Summit Boot"}},
	{"Vantage headphones", []string{"Vantage Sound", "Vantage Studio", "Vantage Mini"}},
	{"smartwatch with GPS", []string{"Pulse Watch", "Vantage Time", "Vantage Active"}},
	{"noise cancelling headphones", []string{"Pulse Air", "Vantage Sound", "Vantage Studio"}},
	{"budget running shoes", []string{"Summit Stride", "Summit Dash", "Aero Runner"}},
	{"insulated winter hiking boots", []string{"Aero Alpine"}},
	{"luxury hybrid smartwatch", []string{"Vantage Chrono"}},
	{"lightweight trail running shoe", []string{"TrailForge Ridge", "TrailForge Peak"}},
	{"compact on-ear headphones for commuting", []string{"Vantage Mini"}},
}

// minRetrievalRecall is the aggregate recall@FinalTopN this eval must clear
// to pass; a regression in retrieval quality (bad embeddings, a broken
// signal, a bad RRF constant) should fail CI, not just look worse on a
// dashboard.
const minRetrievalRecall = 0.6

func TestRetrievalGoldenSet(t *testing.T) {
	cfg := requireConfig(t)
	ctx := context.Background()

	db, err := postgres.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		t.Fatalf("connect postgres: %v", err)
	}
	defer db.Close()

	embedder, err := gemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiEmbeddingModel, cfg.GeminiEmbeddingDimension)
	if err != nil {
		t.Fatalf("gemini embeddings client: %v", err)
	}

	sources := []retrieval.Source{
		vector.NewSearcher(db),
		fulltext.NewSearcher(db),
		graph.NewStore(db),
	}
	fuser := rrf.New(cfg.RRFK)
	products := postgres.NewProductStore(db)

	var totalRecall float64
	for _, tc := range retrievalGoldenSet {
		t.Run(tc.query, func(t *testing.T) {
			embedding, err := embedder.Embed(ctx, tc.query)
			assert.NoError(t, err)

			q := retrieval.Query{Text: tc.query, Embedding: embedding, TopK: cfg.RetrievalTopK}

			signals := make([][]retrieval.Candidate, 0, len(sources))
			for _, source := range sources {
				candidates, err := source.Search(ctx, q)
				assert.NoError(t, err)
				signals = append(signals, candidates)
			}

			fused, err := fuser.Rerank(ctx, tc.query, signals...)
			assert.NoError(t, err)
			if len(fused) > cfg.FinalTopN {
				fused = fused[:cfg.FinalTopN]
			}

			ids := make([]uuid.UUID, 0, len(fused))
			for _, c := range fused {
				if id, err := uuid.Parse(c.ProductID); err == nil {
					ids = append(ids, id)
				}
			}
			rows, err := products.GetByIDs(ctx, ids)
			assert.NoError(t, err)

			titles := make([]string, len(rows))
			for i, p := range rows {
				titles[i] = p.Title
			}

			recall := recallAt(tc.expectedTitles, titles)
			totalRecall += recall
			t.Logf("query=%q expected=%q got=%q recall=%.2f", tc.query, tc.expectedTitles, titles, recall)
		})
	}

	meanRecall := totalRecall / float64(len(retrievalGoldenSet))
	t.Logf("mean recall@FinalTopN across %d queries: %.3f", len(retrievalGoldenSet), meanRecall)
	assert.GreaterOrEqual(t, meanRecall, minRetrievalRecall, "retrieval quality regressed below the eval threshold")
}

// recallAt is the fraction of expected titles present anywhere in got.
func recallAt(expected, got []string) float64 {
	if len(expected) == 0 {
		return 1
	}
	gotSet := make(map[string]bool, len(got))
	for _, title := range got {
		gotSet[title] = true
	}
	hits := 0
	for _, title := range expected {
		if gotSet[title] {
			hits++
		}
	}
	return float64(hits) / float64(len(expected))
}

func requireConfig(t *testing.T) config.Config {
	t.Helper()
	if os.Getenv("DATABASE_URL") == "" || os.Getenv("GEMINI_API_KEY") == "" {
		t.Skip("DATABASE_URL and GEMINI_API_KEY must be set to run this eval against a live, seeded stack")
	}
	cfg, err := config.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	return cfg
}
