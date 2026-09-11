// Package retrieval defines the multi-signal search seam: vector similarity,
// graph proximity, and full-text keyword search each implement Source so the
// pipeline can fan out over them uniformly and fuse the results.
package retrieval

import "context"

// Candidate is one product's rank/score from a single retrieval signal.
type Candidate struct {
	ProductID string
	Rank      int
	Score     float64
	Source    string
}

// Query carries everything a Source might need: the raw text for full-text
// search, the embedding for vector search, and classifier-extracted entities
// plus vector-hit seed IDs for graph traversal.
type Query struct {
	Text      string
	Embedding []float32
	Category  string
	Brand     string
	SeedIDs   []string
	TopK      int
}

// Source is one retrieval signal. Implementations: vector (pgvector cosine
// similarity), fulltext (Postgres tsvector/ts_rank), graph (Apache AGE
// traversal).
type Source interface {
	Name() string
	Search(ctx context.Context, q Query) ([]Candidate, error)
}
