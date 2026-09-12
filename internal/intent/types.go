// Package intent defines the query-intent classification seam used to route
// incoming queries before retrieval runs.
package intent

import "context"

// Intent labels what kind of request a query is, so the pipeline can route
// it (answer via retrieval, or deflect without one).
type Intent string

const (
	// ProductSearch means the user wants products matching some description.
	ProductSearch Intent = "product_search"
	// ProductComparison means the user wants to compare two or more specific products.
	ProductComparison Intent = "product_comparison"
	// AttributeQuestion means the user asks about a specific attribute of a product or category.
	AttributeQuestion Intent = "product_attribute_question"
	// OutOfScope means the query is unrelated to the product catalog.
	OutOfScope Intent = "out_of_scope"
)

// Entities are structured hints extracted from the query alongside the
// intent label. The pipeline uses them to seed graph traversal directly,
// in addition to seeding it from vector hits.
type Entities struct {
	Brand      string   `json:"brand"`
	Category   string   `json:"category"`
	Attributes []string `json:"attributes"`
}

// Result is a classified query: its intent label, the classifier's
// confidence, and any entities extracted alongside it.
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
