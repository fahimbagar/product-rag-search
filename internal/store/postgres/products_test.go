package postgres

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/pashagolub/pgxmock/v4"
	"github.com/pgvector/pgvector-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fahimbagar/product-rag-search/internal/store"
)

// errorContains builds an assert.ErrorAssertionFunc that checks err's
// message contains substr.
func errorContains(substr string) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, err error, args ...interface{}) bool {
		return assert.ErrorContains(t, err, substr, args...)
	}
}

func TestProductStore_Insert(t *testing.T) {
	t.Parallel()

	productID := uuid.New()
	embedding := []float32{0.1, 0.2}
	embeddingText := pgvector.NewVector(embedding).String()

	tests := []struct {
		name        string
		product     store.Product
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
		expectedID  uuid.UUID
	}{
		{
			name:    "inserts with an embedding",
			product: store.Product{Title: "Trail Runner", Brand: "Nike", Price: 89.99, Embedding: embedding},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO products").
					WithArgs("Trail Runner", "", "", "Nike", 89.99, pgxmock.AnyArg(), &embeddingText).
					WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(productID))
			},
			expectedErr: assert.NoError,
			expectedID:  productID,
		},
		{
			name:    "inserts without an embedding",
			product: store.Product{Title: "Trail Socks", Brand: "Nike", Price: 9.99},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO products").
					WithArgs("Trail Socks", "", "", "Nike", 9.99, pgxmock.AnyArg(), (*string)(nil)).
					WillReturnRows(pgxmock.NewRows([]string{"id"}).AddRow(productID))
			},
			expectedErr: assert.NoError,
			expectedID:  productID,
		},
		{
			name:    "wraps a database error",
			product: store.Product{Title: "Trail Runner"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("INSERT INTO products").
					WithArgs(pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg(), pgxmock.AnyArg()).
					WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("insert product"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			id, err := NewProductStore(mock).Insert(context.Background(), tt.product)

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expectedID, id)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProductStore_GetByIDs(t *testing.T) {
	t.Parallel()

	productID := uuid.New()
	productSocksID := uuid.New()
	now := time.Now()
	embedding := []float32{0.1, 0.2}
	embeddingText := pgvector.NewVector(embedding).String()
	columns := []string{"id", "title", "description", "category", "brand", "price", "attributes", "embedding", "created_at", "updated_at"}

	tests := []struct {
		name        string
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
		expected    []store.Product
	}{
		{
			name: "maps rows and parses the embedding",
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).
					AddRow(productID, "Trail Runner", "A shoe", "Footwear", "Nike", 89.99, map[string]any{"waterproof": true}, &embeddingText, now, now).
					AddRow(productSocksID, "Trail Socks", "A sock", "Footwear", "Nike", 9.99, map[string]any{}, nil, now, now)
				mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
			},
			expectedErr: assert.NoError,
			expected: []store.Product{
				{ID: productID, Title: "Trail Runner", Description: "A shoe", Category: "Footwear", Brand: "Nike", Price: 89.99, Attributes: map[string]any{"waterproof": true}, Embedding: embedding, CreatedAt: now, UpdatedAt: now},
				{ID: productSocksID, Title: "Trail Socks", Description: "A sock", Category: "Footwear", Brand: "Nike", Price: 9.99, Attributes: map[string]any{}, CreatedAt: now, UpdatedAt: now},
			},
		},
		{
			name: "wraps a query error",
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg()).WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("get products by ids"),
		},
		{
			name: "wraps a scan error",
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).
					AddRow(42, "Trail Runner", "A shoe", "Footwear", "Nike", 89.99, map[string]any{}, nil, now, now)
				mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
			},
			expectedErr: errorContains("scan product"),
		},
		{
			name: "wraps a malformed embedding",
			setup: func(mock pgxmock.PgxPoolIface) {
				malformed := "not-a-vector"
				rows := pgxmock.NewRows(columns).
					AddRow(productID, "Trail Runner", "A shoe", "Footwear", "Nike", 89.99, map[string]any{}, &malformed, now, now)
				mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
			},
			expectedErr: errorContains("parse embedding"),
		},
		{
			name: "wraps a row iteration error",
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).
					AddRow(productID, "Trail Runner", "A shoe", "Footwear", "Nike", 89.99, map[string]any{}, nil, now, now).
					CloseError(errors.New("connection dropped mid-stream"))
				mock.ExpectQuery("SELECT (.+) FROM products").WithArgs(pgxmock.AnyArg()).WillReturnRows(rows)
			},
			expectedErr: errorContains("scan products"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			got, err := NewProductStore(mock).GetByIDs(context.Background(), []uuid.UUID{productID})

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expected, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestProductStore_List(t *testing.T) {
	t.Parallel()

	productID := uuid.New()
	now := time.Now()
	columns := []string{"id", "title", "description", "category", "brand", "price", "attributes", "embedding", "created_at", "updated_at"}

	tests := []struct {
		name        string
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
		expected    []store.Product
	}{
		{
			name: "lists every product ordered by creation",
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).
					AddRow(productID, "Trail Runner", "A shoe", "Footwear", "Nike", 89.99, map[string]any{}, nil, now, now)
				mock.ExpectQuery("SELECT (.+) FROM products ORDER BY created_at").WillReturnRows(rows)
			},
			expectedErr: assert.NoError,
			expected: []store.Product{
				{ID: productID, Title: "Trail Runner", Description: "A shoe", Category: "Footwear", Brand: "Nike", Price: 89.99, Attributes: map[string]any{}, CreatedAt: now, UpdatedAt: now},
			},
		},
		{
			name: "wraps a query error",
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("SELECT (.+) FROM products ORDER BY created_at").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("list products"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			got, err := NewProductStore(mock).List(context.Background())

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expected, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
