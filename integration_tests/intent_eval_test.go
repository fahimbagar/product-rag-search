//go:build integration

package integration_tests

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fahimbagar/product-rag-search/internal/intent"
	"github.com/fahimbagar/product-rag-search/internal/llm/gemini"
)

// intentGoldenSet pairs a query with the intent label the classifier should
// assign it. Product-specific examples reference the seeded catalog so the
// classifier has real brands/categories to extract entities from, but the
// eval only checks the intent label, not extracted entities.
var intentGoldenSet = []struct {
	query  string
	intent intent.Intent
}{
	{"comfortable running shoes under $100", intent.ProductSearch},
	{"show me hiking boots", intent.ProductSearch},
	{"compare the Aero Runner and the TrailForge Ridge", intent.ProductComparison},
	{"which is better, Pulse Fit or Vantage Active", intent.ProductComparison},
	{"is the Pulse Watch waterproof", intent.AttributeQuestion},
	{"how much battery life does the Vantage Chrono have", intent.AttributeQuestion},
	{"what colors does the Vantage Time come in", intent.AttributeQuestion},
	{"hey how are you", intent.OutOfScope},
	{"what's the weather today", intent.OutOfScope},
	{"tell me a joke", intent.OutOfScope},
}

// minIntentAccuracy is the aggregate classification accuracy this eval must
// clear to pass.
const minIntentAccuracy = 0.7

func TestIntentClassificationGoldenSet(t *testing.T) {
	cfg := requireConfig(t)
	ctx := context.Background()

	classifier, err := gemini.NewClient(ctx, cfg.GeminiAPIKey, cfg.GeminiIntentModel, cfg.GeminiAnswerModel)
	if err != nil {
		t.Fatalf("gemini llm client: %v", err)
	}

	correct := 0
	for _, tc := range intentGoldenSet {
		t.Run(tc.query, func(t *testing.T) {
			result, err := classifier.Classify(ctx, tc.query)
			assert.NoError(t, err)

			t.Logf("query=%q expected=%s got=%s confidence=%.2f", tc.query, tc.intent, result.Intent, result.Confidence)
			if result.Intent == tc.intent {
				correct++
			}
		})
	}

	accuracy := float64(correct) / float64(len(intentGoldenSet))
	t.Logf("intent classification accuracy across %d queries: %.3f", len(intentGoldenSet), accuracy)
	assert.GreaterOrEqual(t, accuracy, minIntentAccuracy, "intent classification accuracy regressed below the eval threshold")
}
