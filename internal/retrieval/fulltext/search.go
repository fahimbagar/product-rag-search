// Package fulltext implements retrieval.Source via Postgres tsvector/ts_rank
// keyword search.
package fulltext

import (
	"context"
	"fmt"

	"github.com/fahimbagar/product-rag-search/internal/retrieval"
	"github.com/fahimbagar/product-rag-search/internal/store"
)

// Searcher implements retrieval.Source via Postgres full-text search.
type Searcher struct {
	pool store.DB
}

func NewSearcher(pool store.DB) *Searcher {
	return &Searcher{pool: pool}
}

func (s *Searcher) Name() string { return "fulltext" }

func (s *Searcher) Search(ctx context.Context, q retrieval.Query) ([]retrieval.Candidate, error) {
	if q.Text == "" {
		return nil, nil
	}

	rows, err := s.pool.Query(ctx, `
		SELECT id::text, ts_rank(search_tsv, websearch_to_tsquery('english', $1)) AS rank
		FROM products
		WHERE search_tsv @@ websearch_to_tsquery('english', $1)
		ORDER BY rank DESC
		LIMIT $2
	`, q.Text, q.TopK)
	if err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}
	defer rows.Close()

	var candidates []retrieval.Candidate
	rank := 1
	for rows.Next() {
		var id string
		var score float64
		if err := rows.Scan(&id, &score); err != nil {
			return nil, fmt.Errorf("fulltext search: scan: %w", err)
		}
		candidates = append(candidates, retrieval.Candidate{
			ProductID: id,
			Rank:      rank,
			Score:     score,
			Source:    s.Name(),
		})
		rank++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("fulltext search: %w", err)
	}
	return candidates, nil
}
