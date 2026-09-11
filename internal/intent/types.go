// Package intent defines the query-intent classification seam used to route
// incoming queries before retrieval runs.
package intent

import "context"

type Intent string

const (
	ProductSearch     Intent = "product_search"
	ProductComparison Intent = "product_comparison"
	AttributeQuestion Intent = "product_attribute_question"
	OutOfScope        Intent = "out_of_scope"
)

// Entities are structured hints extracted from the query alongside the
// intent label, used to seed graph traversal directly (not just via vector
// hits).
type Entities struct {
	Brand      string   `json:"brand"`
	Category   string   `json:"category"`
	Attributes []string `json:"attributes"`
}

type Result struct {
	Intent     Intent   `json:"intent"`
	Confidence float64  `json:"confidence"`
	Entities   Entities `json:"entities"`
}

// Classifier labels a raw user query. Implementations are swappable (Claude
// today, any structured-output LLM tomorrow).
type Classifier interface {
	Classify(ctx context.Context, query string) (Result, error)
}
