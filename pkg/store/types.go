// Package store defines the product persistence seam shared by ingestion
// and the query pipeline.
package store

import (
	"context"
	"time"

	"github.com/google/uuid"
)

type Product struct {
	ID          uuid.UUID
	Title       string
	Description string
	Category    string
	Brand       string
	Price       float64
	Attributes  map[string]any
	Embedding   []float32
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// ProductRepository persists and retrieves products. Implementations are
// swappable (Postgres today); callers depend on this interface rather than
// a concrete database package.
type ProductRepository interface {
	Insert(ctx context.Context, p Product) (uuid.UUID, error)
	GetByIDs(ctx context.Context, ids []uuid.UUID) ([]Product, error)
	List(ctx context.Context) ([]Product, error)
}
