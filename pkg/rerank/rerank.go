// Package rerank defines the fusion seam that merges multiple retrieval
// signals into one ranked candidate list.
package rerank

import (
	"context"

	"github.com/fahimbagar/product-rag-search/pkg/retrieval"
)

// Reranker fuses one ranked candidate list per retrieval signal into a
// single ranked list. Implementations are swappable — RRF today, an
// LLM/cross-encoder reranker later — without changing callers.
type Reranker interface {
	Rerank(ctx context.Context, query string, signals ...[]retrieval.Candidate) ([]retrieval.Candidate, error)
}
