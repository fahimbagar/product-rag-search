package postgres

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/pgvector/pgvector-go"

	"github.com/fahimbagar/product-rag-search/internal/store"
)

// ProductStore implements store.ProductRepository against Postgres.
type ProductStore struct {
	pool store.DB
}

func NewProductStore(pool store.DB) *ProductStore {
	return &ProductStore{pool: pool}
}

func (s *ProductStore) Insert(ctx context.Context, p store.Product) (uuid.UUID, error) {
	var embeddingText *string
	if len(p.Embedding) > 0 {
		v := pgvector.NewVector(p.Embedding).String()
		embeddingText = &v
	}

	var id uuid.UUID
	err := s.pool.QueryRow(ctx, `
		INSERT INTO products (title, description, category, brand, price, attributes, embedding)
		VALUES ($1, $2, $3, $4, $5, $6, $7::vector)
		RETURNING id
	`, p.Title, p.Description, p.Category, p.Brand, p.Price, p.Attributes, embeddingText).Scan(&id)
	if err != nil {
		return uuid.Nil, fmt.Errorf("insert product: %w", err)
	}
	return id, nil
}

func (s *ProductStore) GetByIDs(ctx context.Context, ids []uuid.UUID) ([]store.Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, description, category, brand, price, attributes, embedding::text, created_at, updated_at
		FROM products
		WHERE id = ANY($1)
	`, ids)
	if err != nil {
		return nil, fmt.Errorf("get products by ids: %w", err)
	}
	defer rows.Close()

	return scanProducts(rows)
}

func (s *ProductStore) List(ctx context.Context) ([]store.Product, error) {
	rows, err := s.pool.Query(ctx, `
		SELECT id, title, description, category, brand, price, attributes, embedding::text, created_at, updated_at
		FROM products
		ORDER BY created_at
	`)
	if err != nil {
		return nil, fmt.Errorf("list products: %w", err)
	}
	defer rows.Close()

	return scanProducts(rows)
}

func scanProducts(rows pgx.Rows) ([]store.Product, error) {
	var products []store.Product
	for rows.Next() {
		var p store.Product
		var embeddingText *string
		if err := rows.Scan(&p.ID, &p.Title, &p.Description, &p.Category, &p.Brand, &p.Price,
			&p.Attributes, &embeddingText, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan product: %w", err)
		}
		if embeddingText != nil {
			var v pgvector.Vector
			if err := v.Parse(*embeddingText); err != nil {
				return nil, fmt.Errorf("parse embedding: %w", err)
			}
			p.Embedding = v.Slice()
		}
		products = append(products, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("scan products: %w", err)
	}
	return products, nil
}
