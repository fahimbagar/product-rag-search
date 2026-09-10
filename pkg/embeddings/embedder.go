// Package embeddings defines the seam between the pipeline and whichever
// embedding provider produces vector representations of text.
package embeddings

import "context"

// Embedder turns text into a fixed-size vector. Implementations are
// interchangeable (OpenAI today, a local model tomorrow) behind this
// interface.
type Embedder interface {
	Embed(ctx context.Context, text string) ([]float32, error)
}
