// Package claude implements intent.Classifier and llm.Generator on top of
// the Anthropic Claude API.
package claude

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"

	"github.com/fahimbagar/product-rag-search/pkg/intent"
	"github.com/fahimbagar/product-rag-search/pkg/llm"
)

// Client wraps the Anthropic SDK. It satisfies both intent.Classifier and
// llm.Generator, using a cheaper model for classification and a stronger
// model for grounded generation.
type Client struct {
	sdk         anthropic.Client
	intentModel string
	answerModel string
}

func NewClient(apiKey, intentModel, answerModel string) *Client {
	return &Client{
		sdk:         anthropic.NewClient(option.WithAPIKey(apiKey)),
		intentModel: intentModel,
		answerModel: answerModel,
	}
}

const intentSystemPrompt = `You classify product-search queries for an e-commerce catalog.
Respond only with the requested JSON. Valid intents:
- product_search: the user wants products matching some description
- product_comparison: the user wants to compare two or more specific products
- product_attribute_question: the user asks about a specific attribute of a product or category
- out_of_scope: anything unrelated to the product catalog (greetings, chit-chat, unrelated topics)
Extract brand/category/attributes only when explicitly mentioned; leave them empty otherwise.`

var intentSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"intent": map[string]any{
			"type": "string",
			"enum": []string{
				string(intent.ProductSearch),
				string(intent.ProductComparison),
				string(intent.AttributeQuestion),
				string(intent.OutOfScope),
			},
		},
		"confidence": map[string]any{"type": "number"},
		"entities": map[string]any{
			"type": "object",
			"properties": map[string]any{
				"brand":      map[string]any{"type": "string"},
				"category":   map[string]any{"type": "string"},
				"attributes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}},
			},
			"required": []string{"brand", "category", "attributes"},
		},
	},
	"required": []string{"intent", "confidence", "entities"},
}

func (c *Client) Classify(ctx context.Context, query string) (intent.Result, error) {
	resp, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.intentModel),
		MaxTokens: 512,
		System:    []anthropic.TextBlockParam{{Text: intentSystemPrompt}},
		OutputConfig: anthropic.OutputConfigParam{
			Format: anthropic.JSONOutputFormatParam{Schema: intentSchema},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(query)),
		},
	})
	if err != nil {
		return intent.Result{}, fmt.Errorf("claude classify: %w", err)
	}

	text, err := firstText(resp)
	if err != nil {
		return intent.Result{}, fmt.Errorf("claude classify: %w", err)
	}

	var result intent.Result
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		return intent.Result{}, fmt.Errorf("claude classify: parse response: %w", err)
	}
	return result, nil
}

const answerSystemPrompt = `You are a product search assistant. Answer the user's question using only
the product documents provided. Cite the products you rely on. If none of the
documents answer the question, say so plainly instead of guessing.`

func (c *Client) GenerateAnswer(ctx context.Context, query string, docs []llm.ProductDoc) (llm.Answer, error) {
	blocks := make([]anthropic.ContentBlockParamUnion, 0, len(docs)+1)
	for _, d := range docs {
		doc := anthropic.DocumentBlockParam{
			Title:     anthropic.String(d.Title),
			Citations: anthropic.CitationsConfigParam{Enabled: anthropic.Bool(true)},
			Source: anthropic.DocumentBlockParamSourceUnion{
				OfText: &anthropic.PlainTextSourceParam{Data: formatProductDoc(d)},
			},
		}
		blocks = append(blocks, anthropic.ContentBlockParamUnion{OfDocument: &doc})
	}
	blocks = append(blocks, anthropic.NewTextBlock(query))

	resp, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.answerModel),
		MaxTokens: 1024,
		System:    []anthropic.TextBlockParam{{Text: answerSystemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(blocks...),
		},
	})
	if err != nil {
		return llm.Answer{}, fmt.Errorf("claude generate answer: %w", err)
	}

	var text strings.Builder
	citedIndexes := map[int64]struct{}{}
	for _, block := range resp.Content {
		tb, ok := block.AsAny().(anthropic.TextBlock)
		if !ok {
			continue
		}
		text.WriteString(tb.Text)
		for _, citation := range tb.Citations {
			if loc, ok := citation.AsAny().(anthropic.CitationCharLocation); ok {
				citedIndexes[loc.DocumentIndex] = struct{}{}
			}
		}
	}

	citedIDs := make([]string, 0, len(citedIndexes))
	for idx := range citedIndexes {
		if idx >= 0 && int(idx) < len(docs) {
			citedIDs = append(citedIDs, docs[idx].ID)
		}
	}

	return llm.Answer{Text: text.String(), CitedProductIDs: citedIDs}, nil
}

const deflectSystemPrompt = `You are a product search assistant. The user's message is unrelated to the
product catalog. Reply with one short, friendly sentence redirecting them to
ask about products. Do not answer the off-topic question.`

func (c *Client) Deflect(ctx context.Context, query string) (string, error) {
	resp, err := c.sdk.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     anthropic.Model(c.intentModel),
		MaxTokens: 128,
		System:    []anthropic.TextBlockParam{{Text: deflectSystemPrompt}},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(query)),
		},
	})
	if err != nil {
		return "", fmt.Errorf("claude deflect: %w", err)
	}
	return firstText(resp)
}

func firstText(resp *anthropic.Message) (string, error) {
	for _, block := range resp.Content {
		if tb, ok := block.AsAny().(anthropic.TextBlock); ok {
			return tb.Text, nil
		}
	}
	return "", fmt.Errorf("no text block in response")
}

func formatProductDoc(d llm.ProductDoc) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Title: %s\n", d.Title)
	fmt.Fprintf(&b, "Brand: %s\n", d.Brand)
	fmt.Fprintf(&b, "Category: %s\n", d.Category)
	fmt.Fprintf(&b, "Price: %.2f\n", d.Price)
	fmt.Fprintf(&b, "Description: %s\n", d.Description)
	if len(d.Attributes) > 0 {
		attrJSON, _ := json.Marshal(d.Attributes)
		fmt.Fprintf(&b, "Attributes: %s\n", attrJSON)
	}
	return b.String()
}
