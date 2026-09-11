// Package gemini implements intent.Classifier and llm.Generator using the
// Gemini API.
//
// Gemini has no equivalent of Claude's citations API (which mechanically
// verifies which source document backed each span of generated text) for
// documents supplied inline in a request — its closest feature, File Search,
// requires pre-indexing a whole corpus and lets Gemini do its own retrieval,
// which doesn't fit a pipeline that already does its own hybrid retrieval.
// So GenerateAnswer asks the model to self-report cited_product_ids via a
// JSON schema instead: the shape is enforced, but nothing verifies the
// model's claim the way Claude's citations do. See
// docs/future-improvements.md for the full tradeoff.
package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"google.golang.org/genai"

	"github.com/fahimbagar/product-rag-search/internal/intent"
	"github.com/fahimbagar/product-rag-search/internal/llm"
)

// Client wraps the Gemini SDK. It satisfies both intent.Classifier and
// llm.Generator, using a cheaper model for classification and a stronger
// model for generation.
type Client struct {
	sdk         *genai.Client
	intentModel string
	answerModel string
}

func NewClient(ctx context.Context, apiKey, intentModel, answerModel string) (*Client, error) {
	sdk, err := genai.NewClient(ctx, &genai.ClientConfig{APIKey: apiKey})
	if err != nil {
		return nil, fmt.Errorf("gemini llm client: %w", err)
	}
	return &Client{sdk: sdk, intentModel: intentModel, answerModel: answerModel}, nil
}

const intentSystemPrompt = `You classify product-search queries for an e-commerce catalog.
Respond only with the requested JSON. Valid intents:
- product_search: the user wants products matching some description
- product_comparison: the user wants to compare two or more specific products
- product_attribute_question: the user asks about a specific attribute of a product or category
- out_of_scope: anything unrelated to the product catalog (greetings, chit-chat, unrelated topics)
Extract brand/category/attributes only when explicitly mentioned; leave them empty otherwise.`

var intentSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"intent": {
			Type: genai.TypeString,
			Enum: []string{
				string(intent.ProductSearch),
				string(intent.ProductComparison),
				string(intent.AttributeQuestion),
				string(intent.OutOfScope),
			},
		},
		"confidence": {Type: genai.TypeNumber},
		"entities": {
			Type: genai.TypeObject,
			Properties: map[string]*genai.Schema{
				"brand":      {Type: genai.TypeString},
				"category":   {Type: genai.TypeString},
				"attributes": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
			},
			Required: []string{"brand", "category", "attributes"},
		},
	},
	Required: []string{"intent", "confidence", "entities"},
}

func (c *Client) Classify(ctx context.Context, query string) (intent.Result, error) {
	resp, err := c.sdk.Models.GenerateContent(ctx, c.intentModel,
		[]*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: query}}}},
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: intentSystemPrompt}}},
			ResponseMIMEType:  "application/json",
			ResponseSchema:    intentSchema,
		},
	)
	if err != nil {
		return intent.Result{}, fmt.Errorf("gemini classify: %w", err)
	}

	var result intent.Result
	if err := json.Unmarshal([]byte(resp.Text()), &result); err != nil {
		return intent.Result{}, fmt.Errorf("gemini classify: parse response: %w", err)
	}
	return result, nil
}

const answerSystemPrompt = `You are a product search assistant. Answer the user's question using only
the product documents provided. Set cited_product_ids to the ids of the
products you actually relied on to answer -- omit any you did not use. If none
of the documents answer the question, say so plainly instead of guessing.`

var answerSchema = &genai.Schema{
	Type: genai.TypeObject,
	Properties: map[string]*genai.Schema{
		"answer":            {Type: genai.TypeString},
		"cited_product_ids": {Type: genai.TypeArray, Items: &genai.Schema{Type: genai.TypeString}},
	},
	Required: []string{"answer", "cited_product_ids"},
}

type answerPayload struct {
	Answer          string   `json:"answer"`
	CitedProductIDs []string `json:"cited_product_ids"`
}

func (c *Client) GenerateAnswer(ctx context.Context, query string, docs []llm.ProductDoc) (llm.Answer, error) {
	var prompt strings.Builder
	for _, d := range docs {
		fmt.Fprintf(&prompt, "Product id=%s\n%s\n\n", d.ID, formatProductDoc(d))
	}
	fmt.Fprintf(&prompt, "Question: %s\n", query)

	resp, err := c.sdk.Models.GenerateContent(ctx, c.answerModel,
		[]*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: prompt.String()}}}},
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: answerSystemPrompt}}},
			ResponseMIMEType:  "application/json",
			ResponseSchema:    answerSchema,
		},
	)
	if err != nil {
		return llm.Answer{}, fmt.Errorf("gemini generate answer: %w", err)
	}

	var payload answerPayload
	if err := json.Unmarshal([]byte(resp.Text()), &payload); err != nil {
		return llm.Answer{}, fmt.Errorf("gemini generate answer: parse response: %w", err)
	}

	// The schema only enforces shape, not truthfulness -- the model could
	// self-report an id we never gave it. Drop anything not in the actual
	// candidate set; we still can't verify it truly informed the answer text.
	validIDs := make(map[string]bool, len(docs))
	for _, d := range docs {
		validIDs[d.ID] = true
	}
	citedIDs := make([]string, 0, len(payload.CitedProductIDs))
	for _, id := range payload.CitedProductIDs {
		if validIDs[id] {
			citedIDs = append(citedIDs, id)
		}
	}

	return llm.Answer{Text: payload.Answer, CitedProductIDs: citedIDs}, nil
}

const deflectSystemPrompt = `You are a product search assistant. The user's message is unrelated to the
product catalog. Reply with one short, friendly sentence redirecting them to
ask about products. Do not answer the off-topic question.`

func (c *Client) Deflect(ctx context.Context, query string) (string, error) {
	resp, err := c.sdk.Models.GenerateContent(ctx, c.intentModel,
		[]*genai.Content{{Role: "user", Parts: []*genai.Part{{Text: query}}}},
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{Parts: []*genai.Part{{Text: deflectSystemPrompt}}},
		},
	)
	if err != nil {
		return "", fmt.Errorf("gemini deflect: %w", err)
	}
	return resp.Text(), nil
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
