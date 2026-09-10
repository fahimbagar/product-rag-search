// Package rrf implements rerank.Reranker via Reciprocal Rank Fusion:
// score(id) = sum over signals of 1 / (k + rank).
package rrf

import (
	"context"
	"sort"

	"github.com/fahimbagar/product-rag-search/pkg/retrieval"
)

const defaultK = 60

type Fuser struct {
	K int
}

func New(k int) *Fuser {
	if k <= 0 {
		k = defaultK
	}
	return &Fuser{K: k}
}

func (f *Fuser) Rerank(_ context.Context, _ string, signals ...[]retrieval.Candidate) ([]retrieval.Candidate, error) {
	scores := make(map[string]float64)
	for _, signal := range signals {
		for _, c := range signal {
			if c.Rank <= 0 {
				continue
			}
			scores[c.ProductID] += 1.0 / float64(f.K+c.Rank)
		}
	}

	ids := make([]string, 0, len(scores))
	for id := range scores {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool {
		if scores[ids[i]] != scores[ids[j]] {
			return scores[ids[i]] > scores[ids[j]]
		}
		return ids[i] < ids[j]
	})

	result := make([]retrieval.Candidate, len(ids))
	for i, id := range ids {
		result[i] = retrieval.Candidate{
			ProductID: id,
			Rank:      i + 1,
			Score:     scores[id],
			Source:    "rrf",
		}
	}
	return result, nil
}
