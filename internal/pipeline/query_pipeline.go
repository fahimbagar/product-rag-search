// Package pipeline orchestrates a query end to end: classify intent, embed,
// fan out to retrieval sources, fuse with rerank, then generate a grounded
// answer.
package pipeline

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"golang.org/x/sync/errgroup"

	"github.com/fahimbagar/product-rag-search/internal/embeddings"
	"github.com/fahimbagar/product-rag-search/internal/intent"
	"github.com/fahimbagar/product-rag-search/internal/llm"
	"github.com/fahimbagar/product-rag-search/internal/metrics"
	"github.com/fahimbagar/product-rag-search/internal/rerank"
	"github.com/fahimbagar/product-rag-search/internal/retrieval"
	"github.com/fahimbagar/product-rag-search/internal/store"
	"github.com/fahimbagar/product-rag-search/pkg/ctxlog"
)

// Config tunes how many candidates each retrieval source returns and how
// many fused results survive into the final answer.
type Config struct {
	RetrievalTopK int
	FinalTopN     int
}

// Pipeline wires an intent classifier, embedder, retrieval sources,
// reranker, generator, and product store into the end-to-end query flow.
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

// ProductResult is one product in a query response, with whether the
// generated answer cited it.
type ProductResult struct {
	ID       string
	Title    string
	Brand    string
	Category string
	Price    float64
	Cited    bool
}

// Result is the full outcome of a Pipeline.Query call: the classified
// intent, the generated answer text, and any cited products.
type Result struct {
	Intent   intent.Intent
	Answer   string
	Products []ProductResult
}

func (p *Pipeline) Query(ctx context.Context, query string) (Result, error) {
	logger := ctxlog.FromContext(ctx)

	classifyStart := time.Now()
	classification, err := p.classifier.Classify(ctx, query)
	metrics.PipelineStageDuration.WithLabelValues("classify").Observe(time.Since(classifyStart).Seconds())
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: classify: %w", err)
	}
	metrics.IntentClassificationsTotal.WithLabelValues(string(classification.Intent)).Inc()
	logger.Debugw("intent classified", "intent", classification.Intent, "confidence", classification.Confidence)

	if classification.Intent == intent.OutOfScope {
		deflectStart := time.Now()
		answer, err := p.generator.Deflect(ctx, query)
		metrics.PipelineStageDuration.WithLabelValues("deflect").Observe(time.Since(deflectStart).Seconds())
		if err != nil {
			return Result{}, fmt.Errorf("pipeline: deflect: %w", err)
		}
		logger.Debugw("deflected out-of-scope query")
		return Result{Intent: classification.Intent, Answer: answer}, nil
	}

	embedStart := time.Now()
	embedding, err := p.embedder.Embed(ctx, query)
	metrics.PipelineStageDuration.WithLabelValues("embed").Observe(time.Since(embedStart).Seconds())
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: embed: %w", err)
	}
	logger.Debugw("query embedded", "dimensions", len(embedding))

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

	rerankStart := time.Now()
	fused, err := p.reranker.Rerank(ctx, query, signals...)
	metrics.PipelineStageDuration.WithLabelValues("rerank").Observe(time.Since(rerankStart).Seconds())
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: rerank: %w", err)
	}
	if len(fused) > p.cfg.FinalTopN {
		fused = fused[:p.cfg.FinalTopN]
	}
	recordSignalContribution(signals, fused)
	logger.Debugw("candidates reranked", "fused", len(fused))

	hydrateStart := time.Now()
	products, err := p.hydrate(ctx, fused)
	metrics.PipelineStageDuration.WithLabelValues("hydrate").Observe(time.Since(hydrateStart).Seconds())
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: hydrate: %w", err)
	}
	logger.Debugw("products hydrated", "count", len(products))
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

	generateStart := time.Now()
	answer, err := p.generator.GenerateAnswer(ctx, query, docs)
	metrics.PipelineStageDuration.WithLabelValues("generate").Observe(time.Since(generateStart).Seconds())
	if err != nil {
		return Result{}, fmt.Errorf("pipeline: generate answer: %w", err)
	}
	logger.Debugw("answer generated", "cited_products", len(answer.CitedProductIDs))

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
	logger := ctxlog.FromContext(ctx)
	signals := make([][]retrieval.Candidate, len(p.sources))

	g, gctx := errgroup.WithContext(ctx)
	for i, source := range p.sources {
		g.Go(func() error {
			start := time.Now()
			candidates, err := source.Search(gctx, q)
			metrics.PipelineStageDuration.WithLabelValues(source.Name() + "_search").Observe(time.Since(start).Seconds())
			if err != nil {
				return fmt.Errorf("%s: %w", source.Name(), err)
			}
			logger.Debugw("retrieval source completed", "source", source.Name(), "candidates", len(candidates))
			signals[i] = candidates
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return signals, nil
}

// recordSignalContribution counts, for each final top-N result, which
// retrieval signal(s) surfaced it. A result found by multiple signals
// increments each of them.
func recordSignalContribution(signals [][]retrieval.Candidate, final []retrieval.Candidate) {
	foundBy := make(map[string]map[string]bool, len(final))
	for _, signal := range signals {
		for _, c := range signal {
			if foundBy[c.ProductID] == nil {
				foundBy[c.ProductID] = make(map[string]bool)
			}
			foundBy[c.ProductID][c.Source] = true
		}
	}
	for _, c := range final {
		for source := range foundBy[c.ProductID] {
			metrics.RetrievalSignalContribution.WithLabelValues(source).Inc()
		}
	}
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
