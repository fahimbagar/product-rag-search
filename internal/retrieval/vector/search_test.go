package vector

import (
	"context"
	"errors"
	"testing"

	"github.com/pashagolub/pgxmock/v4"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fahimbagar/product-rag-search/internal/retrieval"
)

// errorContains builds an assert.ErrorAssertionFunc that checks err's
// message contains substr.
func errorContains(substr string) assert.ErrorAssertionFunc {
	return func(t assert.TestingT, err error, args ...interface{}) bool {
		return assert.ErrorContains(t, err, substr, args...)
	}
}

func TestSearcher_Search(t *testing.T) {
	t.Parallel()

	columns := []string{"id", "distance"}

	tests := []struct {
		name        string
		query       retrieval.Query
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
		expected    []retrieval.Candidate
	}{
		{
			name:        "empty embedding skips the query entirely",
			query:       retrieval.Query{TopK: 10},
			setup:       func(mock pgxmock.PgxPoolIface) {},
			expectedErr: assert.NoError,
			expected:    nil,
		},
		{
			name:  "ranks candidates by ascending distance",
			query: retrieval.Query{Embedding: []float32{0.1, 0.2}, TopK: 10},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).
					AddRow("11111111-1111-1111-1111-111111111111", 0.1).
					AddRow("22222222-2222-2222-2222-222222222222", 0.4)
				mock.ExpectQuery("FROM products").WithArgs(pgxmock.AnyArg(), 10).WillReturnRows(rows)
			},
			expectedErr: assert.NoError,
			expected: []retrieval.Candidate{
				{ProductID: "11111111-1111-1111-1111-111111111111", Rank: 1, Score: 0.9, Source: "vector"},
				{ProductID: "22222222-2222-2222-2222-222222222222", Rank: 2, Score: 0.6, Source: "vector"},
			},
		},
		{
			name:  "wraps a query error",
			query: retrieval.Query{Embedding: []float32{0.1, 0.2}, TopK: 10},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("FROM products").WithArgs(pgxmock.AnyArg(), 10).
					WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("vector search"),
		},
		{
			name:  "wraps a scan error",
			query: retrieval.Query{Embedding: []float32{0.1, 0.2}, TopK: 10},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).AddRow(true, 0.1)
				mock.ExpectQuery("FROM products").WithArgs(pgxmock.AnyArg(), 10).WillReturnRows(rows)
			},
			expectedErr: errorContains("vector search: scan"),
		},
		{
			name:  "wraps a row iteration error",
			query: retrieval.Query{Embedding: []float32{0.1, 0.2}, TopK: 10},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows(columns).
					AddRow("11111111-1111-1111-1111-111111111111", 0.1).
					CloseError(errors.New("connection dropped mid-stream"))
				mock.ExpectQuery("FROM products").WithArgs(pgxmock.AnyArg(), 10).WillReturnRows(rows)
			},
			expectedErr: errorContains("vector search"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			got, err := NewSearcher(mock).Search(context.Background(), tt.query)

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expected, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestSearcher_Name(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "vector", new(Searcher).Name())
}
