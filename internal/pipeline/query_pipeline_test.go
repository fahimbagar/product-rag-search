package pipeline

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fahimbagar/product-rag-search/internal/intent"
	"github.com/fahimbagar/product-rag-search/internal/llm"
	"github.com/fahimbagar/product-rag-search/internal/retrieval"
	"github.com/fahimbagar/product-rag-search/internal/store"
)

// errorContains builds an assert.ErrorAssertionFunc that checks err's
// message contains substr.
func errorContains(substr string) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, err error, args ...interface{}) bool {
		return assert.ErrorContains(t, err, substr, args...)
	}
}

type mockClassifier struct{ mock.Mock }

func (m *mockClassifier) Classify(ctx context.Context, query string) (intent.Result, error) {
	args := m.Called(ctx, query)
	res, _ := args.Get(0).(intent.Result)
	return res, args.Error(1)
}

type mockEmbedder struct{ mock.Mock }

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	emb, _ := args.Get(0).([]float32)
	return emb, args.Error(1)
}

type mockSource struct {
	mock.Mock
	name string
}

func (m *mockSource) Name() string { return m.name }

func (m *mockSource) Search(ctx context.Context, q retrieval.Query) ([]retrieval.Candidate, error) {
	args := m.Called(ctx, q)
	candidates, _ := args.Get(0).([]retrieval.Candidate)
	return candidates, args.Error(1)
}

type mockReranker struct{ mock.Mock }

func (m *mockReranker) Rerank(ctx context.Context, query string, signals ...[]retrieval.Candidate) ([]retrieval.Candidate, error) {
	args := m.Called(ctx, query, signals)
	fused, _ := args.Get(0).([]retrieval.Candidate)
	return fused, args.Error(1)
}

type mockGenerator struct{ mock.Mock }

func (m *mockGenerator) GenerateAnswer(ctx context.Context, query string, docs []llm.ProductDoc) (llm.Answer, error) {
	args := m.Called(ctx, query, docs)
	answer, _ := args.Get(0).(llm.Answer)
	return answer, args.Error(1)
}

func (m *mockGenerator) Deflect(ctx context.Context, query string) (string, error) {
	args := m.Called(ctx, query)
	return args.String(0), args.Error(1)
}

type mockProductRepository struct{ mock.Mock }

func (m *mockProductRepository) Insert(ctx context.Context, p store.Product) (uuid.UUID, error) {
	args := m.Called(ctx, p)
	id, _ := args.Get(0).(uuid.UUID)
	return id, args.Error(1)
}

func (m *mockProductRepository) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]store.Product, error) {
	args := m.Called(ctx, ids)
	products, _ := args.Get(0).([]store.Product)
	return products, args.Error(1)
}

func (m *mockProductRepository) List(ctx context.Context) ([]store.Product, error) {
	args := m.Called(ctx)
	products, _ := args.Get(0).([]store.Product)
	return products, args.Error(1)
}

// pipelineMocks bundles one mock per Pipeline dependency, defaulting to a
// single retrieval source named "vector". Each test case configures only
// the mocks its scenario exercises.
type pipelineMocks struct {
	classifier *mockClassifier
	embedder   *mockEmbedder
	sources    []*mockSource
	reranker   *mockReranker
	generator  *mockGenerator
	products   *mockProductRepository
}

func newPipelineMocks(sourceNames ...string) *pipelineMocks {
	if len(sourceNames) == 0 {
		sourceNames = []string{"vector"}
	}
	sources := make([]*mockSource, len(sourceNames))
	for i, name := range sourceNames {
		sources[i] = &mockSource{name: name}
	}
	return &pipelineMocks{
		classifier: new(mockClassifier),
		embedder:   new(mockEmbedder),
		sources:    sources,
		reranker:   new(mockReranker),
		generator:  new(mockGenerator),
		products:   new(mockProductRepository),
	}
}

func (m *pipelineMocks) build(cfg Config) *Pipeline {
	sources := make([]retrieval.Source, len(m.sources))
	for i, s := range m.sources {
		sources[i] = s
	}
	return New(m.classifier, m.embedder, sources, m.reranker, m.generator, m.products, cfg)
}

func TestPipeline_Query(t *testing.T) {
	t.Parallel()

	productA := uuid.New()
	productB := uuid.New()

	tests := []struct {
		name        string
		query       string
		cfg         Config
		sourceNames []string
		setup       func(m *pipelineMocks)
		expectedErr assert.ErrorAssertionFunc
		expected    Result
	}{
		{
			name:  "out of scope query deflects without retrieval",
			query: "what's the weather today",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, "what's the weather today").
					Return(intent.Result{Intent: intent.OutOfScope}, nil)
				m.generator.On("Deflect", mock.Anything, "what's the weather today").
					Return("I can only help with product questions.", nil)
			},
			expectedErr: assert.NoError,
			expected: Result{
				Intent: intent.OutOfScope,
				Answer: "I can only help with product questions.",
			},
		},
		{
			name:  "classify error",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{}, errors.New("classifier unavailable"))
			},
			expectedErr: errorContains("pipeline: classify"),
		},
		{
			name:  "deflect error",
			query: "what's the weather today",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.OutOfScope}, nil)
				m.generator.On("Deflect", mock.Anything, mock.Anything).
					Return("", errors.New("generator unavailable"))
			},
			expectedErr: errorContains("pipeline: deflect"),
		},
		{
			name:  "embed error",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32(nil), errors.New("embedder unavailable"))
			},
			expectedErr: errorContains("pipeline: embed"),
		},
		{
			name:  "retrieval source error",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32{0.1, 0.2}, nil)
				m.sources[0].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate(nil), errors.New("vector store down"))
			},
			expectedErr: errorContains("pipeline: retrieve"),
		},
		{
			name:  "rerank error",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32{0.1, 0.2}, nil)
				m.sources[0].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productA.String(), Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.reranker.On("Rerank", mock.Anything, mock.Anything, mock.Anything).
					Return([]retrieval.Candidate(nil), errors.New("rerank failed"))
			},
			expectedErr: errorContains("pipeline: rerank"),
		},
		{
			name:  "hydrate error",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32{0.1, 0.2}, nil)
				m.sources[0].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productA.String(), Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.reranker.On("Rerank", mock.Anything, mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productA.String(), Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.products.On("GetByIDs", mock.Anything, mock.Anything).
					Return([]store.Product(nil), errors.New("db unavailable"))
			},
			expectedErr: errorContains("pipeline: hydrate"),
		},
		{
			name:  "no matching products after hydrate",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32{0.1, 0.2}, nil)
				// Not a valid UUID, so hydrate drops it before ever calling GetByIDs.
				m.sources[0].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: "not-a-uuid", Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.reranker.On("Rerank", mock.Anything, mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: "not-a-uuid", Rank: 1, Score: 1, Source: "vector"}}, nil)
			},
			expectedErr: assert.NoError,
			expected: Result{
				Intent: intent.ProductSearch,
				Answer: "I couldn't find any matching products.",
			},
		},
		{
			name:  "generate answer error",
			query: "running shoes",
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32{0.1, 0.2}, nil)
				m.sources[0].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productA.String(), Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.reranker.On("Rerank", mock.Anything, mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productA.String(), Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.products.On("GetByIDs", mock.Anything, mock.Anything).
					Return([]store.Product{{ID: productA, Title: "Trail Runner"}}, nil)
				m.generator.On("GenerateAnswer", mock.Anything, mock.Anything, mock.Anything).
					Return(llm.Answer{}, errors.New("generator unavailable"))
			},
			expectedErr: errorContains("pipeline: generate answer"),
		},
		{
			name:        "happy path fuses multiple sources and marks citations",
			query:       "running shoes",
			cfg:         Config{RetrievalTopK: 10, FinalTopN: 1},
			sourceNames: []string{"vector", "fulltext"},
			setup: func(m *pipelineMocks) {
				m.classifier.On("Classify", mock.Anything, mock.Anything).
					Return(intent.Result{Intent: intent.ProductSearch, Confidence: 0.9}, nil)
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32{0.1, 0.2}, nil)
				m.sources[0].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productA.String(), Rank: 1, Score: 1, Source: "vector"}}, nil)
				m.sources[1].On("Search", mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{{ProductID: productB.String(), Rank: 1, Score: 1, Source: "fulltext"}}, nil)
				// FinalTopN: 1 truncates this to just productA.
				m.reranker.On("Rerank", mock.Anything, mock.Anything, mock.Anything).
					Return([]retrieval.Candidate{
						{ProductID: productA.String(), Rank: 1, Score: 2, Source: "vector"},
						{ProductID: productB.String(), Rank: 2, Score: 1, Source: "fulltext"},
					}, nil)
				m.products.On("GetByIDs", mock.Anything, mock.Anything).
					Return([]store.Product{{ID: productA, Title: "Trail Runner", Brand: "Nike", Category: "Footwear", Price: 89.99}}, nil)
				m.generator.On("GenerateAnswer", mock.Anything, mock.Anything, mock.Anything).
					Return(llm.Answer{Text: "The Trail Runner is a great fit.", CitedProductIDs: []string{productA.String()}}, nil)
			},
			expectedErr: assert.NoError,
			expected: Result{
				Intent: intent.ProductSearch,
				Answer: "The Trail Runner is a great fit.",
				Products: []ProductResult{
					{ID: productA.String(), Title: "Trail Runner", Brand: "Nike", Category: "Footwear", Price: 89.99, Cited: true},
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newPipelineMocks(tt.sourceNames...)
			tt.setup(m)

			cfg := tt.cfg
			if cfg == (Config{}) {
				cfg = Config{RetrievalTopK: 10, FinalTopN: 5}
			}

			got, err := m.build(cfg).Query(context.Background(), tt.query)

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expected, got)
			}
		})
	}
}
