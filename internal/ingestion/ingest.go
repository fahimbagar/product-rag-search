// Package ingestion turns raw product records into stored, embedded,
// graph-linked products: embed -> insert -> build graph edges.
package ingestion

import (
	"context"
	"fmt"

	"github.com/fahimbagar/product-rag-search/internal/embeddings"
	"github.com/fahimbagar/product-rag-search/internal/retrieval/graph"
	"github.com/fahimbagar/product-rag-search/internal/store"
)

// RawProduct is the input shape for a single product before embedding or
// persistence (e.g. as decoded from the seed JSON file).
type RawProduct struct {
	Title       string
	Description string
	Category    string
	Brand       string
	Price       float64
	Attributes  map[string]any
}

// RelatedPair curates an explicit RELATED_TO graph edge between two products
// identified by their index in the batch passed to Ingest.
type RelatedPair struct {
	FromIndex int
	ToIndex   int
	Weight    float64
}

type Ingester struct {
	embedder embeddings.Embedder
	products store.ProductRepository
	graph    graph.Writer
}

func NewIngester(embedder embeddings.Embedder, products store.ProductRepository, graphWriter graph.Writer) *Ingester {
	return &Ingester{embedder: embedder, products: products, graph: graphWriter}
}

// Ingest embeds and stores each product, links it into the graph, then wires
// up any explicitly curated related-product pairs.
func (in *Ingester) Ingest(ctx context.Context, raw []RawProduct, related []RelatedPair) error {
	ids := make([]string, len(raw))

	for i, p := range raw {
		embedding, err := in.embedder.Embed(ctx, embeddingText(p))
		if err != nil {
			return fmt.Errorf("ingest %q: embed: %w", p.Title, err)
		}

		id, err := in.products.Insert(ctx, store.Product{
			Title:       p.Title,
			Description: p.Description,
			Category:    p.Category,
			Brand:       p.Brand,
			Price:       p.Price,
			Attributes:  p.Attributes,
			Embedding:   embedding,
		})
		if err != nil {
			return fmt.Errorf("ingest %q: store: %w", p.Title, err)
		}
		ids[i] = id.String()

		if err := in.graph.UpsertProduct(ctx, graph.ProductNode{
			ProductID:  ids[i],
			Title:      p.Title,
			Category:   p.Category,
			Brand:      p.Brand,
			Attributes: stringAttributes(p.Attributes),
		}); err != nil {
			return fmt.Errorf("ingest %q: graph upsert: %w", p.Title, err)
		}
	}

	for _, r := range related {
		if r.FromIndex < 0 || r.FromIndex >= len(ids) || r.ToIndex < 0 || r.ToIndex >= len(ids) {
			return fmt.Errorf("ingest: related pair index out of range: %+v", r)
		}
		if err := in.graph.LinkRelated(ctx, ids[r.FromIndex], ids[r.ToIndex], r.Weight); err != nil {
			return fmt.Errorf("ingest: link related %d->%d: %w", r.FromIndex, r.ToIndex, err)
		}
	}

	return nil
}

func embeddingText(p RawProduct) string {
	return fmt.Sprintf("%s. %s. Category: %s. Brand: %s.", p.Title, p.Description, p.Category, p.Brand)
}

func stringAttributes(attrs map[string]any) map[string]string {
	out := make(map[string]string, len(attrs))
	for k, v := range attrs {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}
