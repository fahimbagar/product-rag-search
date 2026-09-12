package ingestion

import (
	"context"
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fahimbagar/product-rag-search/internal/retrieval/graph"
	"github.com/fahimbagar/product-rag-search/internal/store"
)

// errorContains builds an assert.ErrorAssertionFunc that checks err's
// message contains substr.
func errorContains(substr string) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, err error, args ...interface{}) bool {
		return assert.ErrorContains(t, err, substr, args...)
	}
}

type mockEmbedder struct{ mock.Mock }

func (m *mockEmbedder) Embed(ctx context.Context, text string) ([]float32, error) {
	args := m.Called(ctx, text)
	emb, _ := args.Get(0).([]float32)
	return emb, args.Error(1)
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

type mockGraphWriter struct{ mock.Mock }

func (m *mockGraphWriter) UpsertProduct(ctx context.Context, p graph.ProductNode) error {
	args := m.Called(ctx, p)
	return args.Error(0)
}

func (m *mockGraphWriter) LinkRelated(ctx context.Context, fromProductID, toProductID string, weight float64) error {
	args := m.Called(ctx, fromProductID, toProductID, weight)
	return args.Error(0)
}

type ingestMocks struct {
	embedder *mockEmbedder
	products *mockProductRepository
	graph    *mockGraphWriter
}

func newIngestMocks() *ingestMocks {
	return &ingestMocks{
		embedder: new(mockEmbedder),
		products: new(mockProductRepository),
		graph:    new(mockGraphWriter),
	}
}

func (m *ingestMocks) build() *Ingester {
	return NewIngester(m.embedder, m.products, m.graph)
}

func TestIngester_Ingest(t *testing.T) {
	t.Parallel()

	productA := uuid.New()
	productB := uuid.New()
	embeddingA := []float32{0.1, 0.2}
	embeddingB := []float32{0.3, 0.4}

	rawSingle := []RawProduct{
		{Title: "Trail Runner", Description: "A shoe", Category: "Footwear", Brand: "Nike", Price: 89.99},
	}
	rawPair := []RawProduct{
		{Title: "Trail Runner", Description: "A shoe", Category: "Footwear", Brand: "Nike", Price: 89.99, Attributes: map[string]any{"waterproof": true}},
		{Title: "Trail Socks", Description: "A sock", Category: "Footwear", Brand: "Nike", Price: 9.99},
	}

	tests := []struct {
		name        string
		raw         []RawProduct
		related     []RelatedPair
		setup       func(m *ingestMocks)
		expectedErr assert.ErrorAssertionFunc
		verify      func(t *testing.T, m *ingestMocks)
	}{
		{
			name: "embed error",
			raw:  rawSingle,
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, mock.Anything).
					Return([]float32(nil), errors.New("embedder unavailable"))
			},
			expectedErr: errorContains(`ingest "Trail Runner": embed`),
		},
		{
			name: "store error",
			raw:  rawSingle,
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, mock.Anything).Return(embeddingA, nil)
				m.products.On("Insert", mock.Anything, mock.Anything).
					Return(uuid.Nil, errors.New("db unavailable"))
			},
			expectedErr: errorContains(`ingest "Trail Runner": store`),
		},
		{
			name: "graph upsert error",
			raw:  rawSingle,
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, mock.Anything).Return(embeddingA, nil)
				m.products.On("Insert", mock.Anything, mock.Anything).Return(productA, nil)
				m.graph.On("UpsertProduct", mock.Anything, mock.Anything).
					Return(errors.New("graph unavailable"))
			},
			expectedErr: errorContains(`ingest "Trail Runner": graph upsert`),
		},
		{
			name:    "related pair index out of range",
			raw:     rawSingle,
			related: []RelatedPair{{FromIndex: 0, ToIndex: 1, Weight: 0.5}},
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, mock.Anything).Return(embeddingA, nil)
				m.products.On("Insert", mock.Anything, mock.Anything).Return(productA, nil)
				m.graph.On("UpsertProduct", mock.Anything, mock.Anything).Return(nil)
			},
			expectedErr: errorContains("related pair index out of range"),
		},
		{
			name:    "link related error",
			raw:     rawPair,
			related: []RelatedPair{{FromIndex: 0, ToIndex: 1, Weight: 0.5}},
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, mock.Anything).Return(embeddingA, nil).Once()
				m.embedder.On("Embed", mock.Anything, mock.Anything).Return(embeddingB, nil).Once()
				m.products.On("Insert", mock.Anything, mock.Anything).Return(productA, nil).Once()
				m.products.On("Insert", mock.Anything, mock.Anything).Return(productB, nil).Once()
				m.graph.On("UpsertProduct", mock.Anything, mock.Anything).Return(nil)
				m.graph.On("LinkRelated", mock.Anything, productA.String(), productB.String(), 0.5).
					Return(errors.New("graph unavailable"))
			},
			expectedErr: errorContains("ingest: link related 0->1"),
		},
		{
			name:    "happy path embeds inserts links products and related pairs",
			raw:     rawPair,
			related: []RelatedPair{{FromIndex: 0, ToIndex: 1, Weight: 0.5}},
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, "Trail Runner. A shoe. Category: Footwear. Brand: Nike.").
					Return(embeddingA, nil)
				m.embedder.On("Embed", mock.Anything, "Trail Socks. A sock. Category: Footwear. Brand: Nike.").
					Return(embeddingB, nil)
				m.products.On("Insert", mock.Anything, mock.MatchedBy(func(p store.Product) bool {
					return p.Title == "Trail Runner" && assert.ObjectsAreEqual(embeddingA, p.Embedding)
				})).Return(productA, nil)
				m.products.On("Insert", mock.Anything, mock.MatchedBy(func(p store.Product) bool {
					return p.Title == "Trail Socks" && assert.ObjectsAreEqual(embeddingB, p.Embedding)
				})).Return(productB, nil)
				m.graph.On("UpsertProduct", mock.Anything, graph.ProductNode{
					ProductID:  productA.String(),
					Title:      "Trail Runner",
					Category:   "Footwear",
					Brand:      "Nike",
					Attributes: map[string]string{"waterproof": "true"},
				}).Return(nil)
				m.graph.On("UpsertProduct", mock.Anything, graph.ProductNode{
					ProductID:  productB.String(),
					Title:      "Trail Socks",
					Category:   "Footwear",
					Brand:      "Nike",
					Attributes: map[string]string{},
				}).Return(nil)
				m.graph.On("LinkRelated", mock.Anything, productA.String(), productB.String(), 0.5).
					Return(nil)
			},
			expectedErr: assert.NoError,
		},
		{
			name:        "no related pairs never calls LinkRelated",
			raw:         rawSingle,
			related:     nil,
			expectedErr: assert.NoError,
			setup: func(m *ingestMocks) {
				m.embedder.On("Embed", mock.Anything, mock.Anything).Return(embeddingA, nil)
				m.products.On("Insert", mock.Anything, mock.Anything).Return(productA, nil)
				m.graph.On("UpsertProduct", mock.Anything, mock.Anything).Return(nil)
			},
			verify: func(t *testing.T, m *ingestMocks) {
				m.graph.AssertNotCalled(t, "LinkRelated", mock.Anything, mock.Anything, mock.Anything, mock.Anything)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newIngestMocks()
			tt.setup(m)

			err := m.build().Ingest(context.Background(), tt.raw, tt.related)

			tt.expectedErr(t, err)
			if tt.verify != nil {
				tt.verify(t, m)
			}
		})
	}
}
