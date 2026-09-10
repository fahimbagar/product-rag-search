// Package llm defines the generation seam: turning a query plus retrieved
// product candidates into a grounded, cited answer.
package llm

import "context"

// ProductDoc is the minimal product view handed to the generator as a
// grounding document.
type ProductDoc struct {
	ID          string
	Title       string
	Description string
	Brand       string
	Category    string
	Price       float64
	Attributes  map[string]any
}

// Answer is a generated response plus the subset of product IDs it actually
// cited, so callers can mark which candidates were used.
type Answer struct {
	Text            string
	CitedProductIDs []string
}

// Generator produces grounded answers from retrieved product documents, and
// short deflections for out-of-scope queries. Implementations are
// swappable (Claude today, any citation-capable LLM tomorrow).
type Generator interface {
	GenerateAnswer(ctx context.Context, query string, docs []ProductDoc) (Answer, error)
	Deflect(ctx context.Context, query string) (string, error)
}
