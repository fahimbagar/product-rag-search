// Package pipeline orchestrates a query end to end: classify intent, embed,
// fan out to retrieval sources, fuse with rerank, then generate a grounded
// answer.
package pipeline

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/fahimbagar/product-rag-search/pkg/embeddings"
	"github.com/fahimbagar/product-rag-search/pkg/intent"
	"github.com/fahimbagar/product-rag-search/pkg/llm"
	"github.com/fahimbagar/product-rag-search/pkg/rerank"
	"github.com/fahimbagar/product-rag-search/pkg/retrieval"
	"github.com/fahimbagar/product-rag-search/pkg/store"
)

type Config struct {
	RetrievalTopK int
	FinalTopN     int
}

type Pipeline struct {
	classifier intent.Classifier
	embedder   embeddings.Embedder
	sources    []retrieval.Source
	reranker   rerank.Reranker
	generator  llm.Generator
	products   store.ProductRepository
	cfg        Config
}

func New(
	classifier intent.Classifier,
	embedder embeddings.Embedder,
	sources []retrieval.Source,
	reranker rerank.Reranker,
	generator llm.Generator,
	products store.ProductRepository,
	cfg Config,
) *Pipeline {
	return &Pipeline{
		classifier: classifier,
		embedder:   embedder,
		sources:    sources,
		reranker:   reranker,
		generator:  generator,
		products:   products,
		cfg:        cfg,
	}
}

type ProductResult struct {
	ID       string
	Title    string
	Brand    string
	Category string
	Price    float64
	Cited    bool
}

type Result struct {
	Intent   intent.Intent
	Answer   string
	Products []ProductResult
}

func (p *Pipeline) Query(ctx context.Context, query string) (Result, error) {
	classification, err := p.classifier.Classify(ctx, query)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: classify: %w", err)
	}

	if classification.Intent == intent.OutOfScope {
		answer, err := p.generator.Deflect(ctx, query)
		if err != nil {
			return Result{}, fmt.Errorf("pipeline: deflect: %w", err)
		}
		return Result{Intent: classification.Intent, Answer: answer}, nil
	}

	embedding, err := p.embedder.Embed(ctx, query)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: embed: %w", err)
	}

	retrievalQuery := retrieval.Query{
		Text:      query,
		Embedding: embedding,
		Category:  classification.Entities.Category,
		Brand:     classification.Entities.Brand,
		TopK:      p.cfg.RetrievalTopK,
	}

	signals, err := p.searchAll(ctx, retrievalQuery)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: retrieve: %w", err)
	}

	fused, err := p.reranker.Rerank(ctx, query, signals...)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: rerank: %w", err)
	}
	if len(fused) > p.cfg.FinalTopN {
		fused = fused[:p.cfg.FinalTopN]
	}

	products, err := p.hydrate(ctx, fused)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: hydrate: %w", err)
	}
	if len(products) == 0 {
		return Result{Intent: classification.Intent, Answer: "I couldn't find any matching products."}, nil
	}

	docs := make([]llm.ProductDoc, len(products))
	for i, prod := range products {
		docs[i] = llm.ProductDoc{
			ID:          prod.ID.String(),
			Title:       prod.Title,
			Description: prod.Description,
			Brand:       prod.Brand,
			Category:    prod.Category,
			Price:       prod.Price,
			Attributes:  prod.Attributes,
		}
	}

	answer, err := p.generator.GenerateAnswer(ctx, query, docs)
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: generate answer: %w", err)
	}

	cited := make(map[string]bool, len(answer.CitedProductIDs))
	for _, id := range answer.CitedProductIDs {
		cited[id] = true
	}

	results := make([]ProductResult, len(products))
	for i, prod := range products {
		id := prod.ID.String()
		results[i] = ProductResult{
			ID:       id,
			Title:    prod.Title,
			Brand:    prod.Brand,
			Category: prod.Category,
			Price:    prod.Price,
			Cited:    cited[id],
		}
	}

	return Result{Intent: classification.Intent, Answer: answer.Text, Products: results}, nil
}

func (p *Pipeline) searchAll(ctx context.Context, q retrieval.Query) ([][]retrieval.Candidate, error) {
	signals := make([][]retrieval.Candidate, len(p.sources))

	g, gctx := errgroup.WithContext(ctx)
	for i, source := range p.sources {
		g.Go(func() error {
			candidates, err := source.Search(gctx, q)
			if err != nil {
				return fmt.Errorf("%s: %w", source.Name(), err)
			}
			signals[i] = candidates
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return signals, nil
}

func (p *Pipeline) hydrate(ctx context.Context, candidates []retrieval.Candidate) ([]store.Product, error) {
	ids := make([]uuid.UUID, 0, len(candidates))
	for _, c := range candidates {
		id, err := uuid.Parse(c.ProductID)
		if err != nil {
			continue
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, nil
	}

	products, err := p.products.GetByIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	byID := make(map[uuid.UUID]store.Product, len(products))
	for _, prod := range products {
		byID[prod.ID] = prod
	}

	ordered := make([]store.Product, 0, len(ids))
	for _, id := range ids {
		if prod, ok := byID[id]; ok {
			ordered = append(ordered, prod)
		}
	}
	return ordered, nil
}
