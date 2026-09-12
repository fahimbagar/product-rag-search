package router

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fahimbagar/product-rag-search/internal/intent"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
)

func TestToQueryResponse(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		in       pipeline.Result
		expected queryResponse
	}{
		{
			name:     "no products",
			in:       pipeline.Result{Intent: intent.OutOfScope, Answer: "I can only help with product questions."},
			expected: queryResponse{Intent: "out_of_scope", Answer: "I can only help with product questions.", Products: []productResponse{}},
		},
		{
			name: "maps every product field and preserves order",
			in: pipeline.Result{
				Intent: intent.ProductSearch,
				Answer: "The Trail Runner is a great fit.",
				Products: []pipeline.ProductResult{
					{ID: "1", Title: "Trail Runner", Brand: "Nike", Category: "Footwear", Price: 89.99, Cited: true},
					{ID: "2", Title: "Trail Socks", Brand: "Nike", Category: "Footwear", Price: 9.99, Cited: false},
				},
			},
			expected: queryResponse{
				Intent: "product_search",
				Answer: "The Trail Runner is a great fit.",
				Products: []productResponse{
					{ID: "1", Title: "Trail Runner", Brand: "Nike", Category: "Footwear", Price: 89.99, Cited: true},
					{ID: "2", Title: "Trail Socks", Brand: "Nike", Category: "Footwear", Price: 9.99, Cited: false},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := toQueryResponse(tt.in)

			assert.Equal(t, tt.expected, got)
		})
	}
}
