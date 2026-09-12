// Package gemini implements embeddings.Embedder using the Gemini API.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// EmbeddingDimension is the vector length Client requests from Gemini. It
// is a constant, not a runtime setting, because it must match the
// `vector(1536)` column in migrations/0001_init_products.up.sql: changing
// it means writing a new migration and re-embedding every product, not
// flipping a config value.
const EmbeddingDimension = 1536

// Client wraps the Gemini SDK to satisfy embeddings.Embedder.
type Client struct {
	sdk   *genai.Client
	model string
}

func NewClient(ctx context.Context, apiKey, model string) (*Client, error) {
	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("gemini embeddings client: %w", err)
	}
	return &Client{sdk: sdk, model: model}, nil
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	dim := int32(EmbeddingDimension)
	resp, err := c.sdk.Models.EmbedContent(ctx, c.model,
		[]*genai.Content{{Parts: []*genai.Part{{Text: text}}}},
		&genai.EmbedContentConfig{OutputDimensionality: &dim},
	)
	if err != nil {
		return nil, fmt.Errorf("gemini embed: %w", err)
	}
	if len(resp.Embeddings) == 0 {
		return nil, fmt.Errorf("gemini embed: empty response")
	}
	return resp.Embeddings[0].Values, nil
}
