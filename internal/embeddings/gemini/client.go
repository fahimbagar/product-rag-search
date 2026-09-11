// Package gemini implements embeddings.Embedder using the Gemini API.
package gemini

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// Client wraps the Gemini SDK to satisfy embeddings.Embedder.
type Client struct {
	sdk       *genai.Client
	model     string
	dimension int32
}

func NewClient(ctx context.Context, apiKey, model string, dimension int) (*Client, error) {
	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("gemini embeddings client: %w", err)
	}
	return &Client{sdk: sdk, model: model, dimension: int32(dimension)}, nil
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	dim := c.dimension
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
