// Package openai implements embeddings.Embedder using the OpenAI API.
package openai

import (
	"context"
	"fmt"

	"github.com/openai/openai-go/v2"
	"github.com/openai/openai-go/v2/option"
)

// Client wraps the OpenAI SDK to satisfy embeddings.Embedder.
type Client struct {
	sdk   openai.Client
	model string
}

func NewClient(apiKey, model string) *Client {
	return &Client{
		sdk:   openai.NewClient(option.WithAPIKey(apiKey)),
		model: model,
	}
}

func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	resp, err := c.sdk.Embeddings.New(ctx, openai.EmbeddingNewParams{
		Input: openai.EmbeddingNewParamsInputUnion{OfString: openai.String(text)},
		Model: openai.EmbeddingModel(c.model),
	})
	if err != nil {
		return nil, fmt.Errorf("openai embed: %w", err)
	}
	if len(resp.Data) == 0 {
		return nil, fmt.Errorf("openai embed: empty response")
	}

	vec64 := resp.Data[0].Embedding
	vec32 := make([]float32, len(vec64))
	for i, v := range vec64 {
		vec32[i] = float32(v)
	}
	return vec32, nil
}
