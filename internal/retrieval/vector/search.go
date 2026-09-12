// Package vector implements retrieval.Source via pgvector cosine similarity.
package vector

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"github.com/fahimbagar/product-rag-search/internal/retrieval"
)

// pgxIface is the subset of *pgxpool.Pool that Searcher needs, so tests can
// substitute a pgxmock pool instead of a real database.
type pgxIface interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

// Searcher implements retrieval.Source via pgvector cosine similarity.
type Searcher struct {
	pool pgxIface
}

func NewSearcher(pool pgxIface) *Searcher {
	return &Searcher{pool: pool}
}

func (s *Searcher) Name() string { return "vector" }

func (s *Searcher) Search(ctx context.Context, q retrieval.Query) ([]retrieval.Candidate, error) {
	if len(q.Embedding) == 0 {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, embedding <=> $1 AS distance
		FROM products
		WHERE embedding IS NOT NULL
		ORDER BY distance
		LIMIT $2
	`, pgvector.NewVector(q.Embedding), q.TopK)
	if err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	defer rows.Close()

	var candidates []retrieval.Candidate
	rank := 1
	for rows.Next() {
		var id string
		var distance float64
		if err := rows.Scan(&id, &distance); err != nil {
			return nil, fmt.Errorf("vector search: scan: %w", err)
		}
		candidates = append(candidates, retrieval.Candidate{
			ProductID: id,
			Rank:      rank,
			Score:     1 - distance, // cosine similarity
			Source:    s.Name(),
		})
		rank++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("vector search: %w", err)
	}
	return candidates, nil
}
