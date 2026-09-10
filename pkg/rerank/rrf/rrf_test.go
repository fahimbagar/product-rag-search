package rrf

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/fahimbagar/product-rag-search/pkg/retrieval"
)

func TestFuserRerank(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		k          int
		signals    [][]retrieval.Candidate
		wantOrder  []string
		wantLength int
	}{
		{
			name:       "no signals returns empty result",
			k:          60,
			signals:    nil,
			wantOrder:  []string{},
			wantLength: 0,
		},
		{
			name: "single signal preserves rank order",
			k:    60,
			signals: [][]retrieval.Candidate{
				{
					{ProductID: "a", Rank: 1},
					{ProductID: "b", Rank: 2},
					{ProductID: "c", Rank: 3},
				},
			},
			wantOrder:  []string{"a", "b", "c"},
			wantLength: 3,
		},
		{
			name: "agreement across signals outranks single-signal top hit",
			k:    60,
			signals: [][]retrieval.Candidate{
				{
					{ProductID: "a", Rank: 1},
					{ProductID: "b", Rank: 2},
				},
				{
					{ProductID: "b", Rank: 1},
					{ProductID: "a", Rank: 3},
				},
			},
			// b: 1/61 + 1/61 = 2/61 ≈ 0.0328
			// a: 1/61 + 1/63 ≈ 0.0164 + 0.0159 = 0.0322
			wantOrder:  []string{"b", "a"},
			wantLength: 2,
		},
		{
			name: "zero rank candidates are ignored",
			k:    60,
			signals: [][]retrieval.Candidate{
				{
					{ProductID: "a", Rank: 0},
					{ProductID: "b", Rank: 1},
				},
			},
			wantOrder:  []string{"b"},
			wantLength: 1,
		},
		{
			name: "tie breaks by product id ascending",
			k:    60,
			signals: [][]retrieval.Candidate{
				{
					{ProductID: "z", Rank: 1},
				},
				{
					{ProductID: "a", Rank: 1},
				},
			},
			wantOrder:  []string{"a", "z"},
			wantLength: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fuser := New(tt.k)
			result, err := fuser.Rerank(context.Background(), "irrelevant query", tt.signals...)

			assert.NoError(t, err)
			assert.Len(t, result, tt.wantLength)

			gotOrder := make([]string, len(result))
			for i, c := range result {
				gotOrder[i] = c.ProductID
				assert.Equal(t, i+1, c.Rank)
				assert.Equal(t, "rrf", c.Source)
			}
			assert.Equal(t, tt.wantOrder, gotOrder)
		})
	}
}

func TestNewDefaultsK(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input int
		wantK int
	}{
		{name: "positive k is kept", input: 30, wantK: 30},
		{name: "zero k falls back to default", input: 0, wantK: defaultK},
		{name: "negative k falls back to default", input: -5, wantK: defaultK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			fuser := New(tt.input)
			assert.Equal(t, tt.wantK, fuser.K)
		})
	}
}
