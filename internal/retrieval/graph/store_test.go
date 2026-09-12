package graph

import (
	"context"
	"errors"
	"regexp"
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

func TestStore_Search(t *testing.T) {
	t.Parallel()

	const seedID = "11111111-1111-1111-1111-111111111111"
	const idA = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
	const idB = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"

	rowsFor := func(id string, count string) *pgxmock.Rows {
		return pgxmock.NewRows([]string{"product_id", "proximity"}).AddRow(`"`+id+`"`, count)
	}
	empty := func() *pgxmock.Rows {
		return pgxmock.NewRows([]string{"product_id", "proximity"})
	}

	tests := []struct {
		name        string
		query       retrieval.Query
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
		expected    []retrieval.Candidate
	}{
		{
			name:        "no signals issues no query",
			query:       retrieval.Query{},
			setup:       func(mock pgxmock.PgxPoolIface) {},
			expectedErr: assert.NoError,
			expected:    nil,
		},
		{
			name:        "invalid seed id is filtered out",
			query:       retrieval.Query{SeedIDs: []string{"not-a-uuid"}},
			setup:       func(mock pgxmock.PgxPoolIface) {},
			expectedErr: assert.NoError,
			expected:    nil,
		},
		{
			name:        "invalid category label is filtered out",
			query:       retrieval.Query{Category: "bad;label"},
			setup:       func(mock pgxmock.PgxPoolIface) {},
			expectedErr: assert.NoError,
			expected:    nil,
		},
		{
			name:  "aggregates proximity across seed ids category and brand",
			query: retrieval.Query{SeedIDs: []string{seedID}, Category: "Footwear", Brand: "Nike", TopK: 1},
			setup: func(mock pgxmock.PgxPoolIface) {
				// hopEdgeTypes order: IN_CATEGORY, BY_BRAND, HAS_ATTRIBUTE, then the
				// weighted RELATED_TO query, then the category and brand label queries.
				mock.ExpectQuery("cypher").WillReturnRows(rowsFor(idA, "2"))
				mock.ExpectQuery("cypher").WillReturnRows(empty())
				mock.ExpectQuery("cypher").WillReturnRows(empty())
				mock.ExpectQuery(regexp.QuoteMeta("sum(r.weight)")).WillReturnRows(rowsFor(idB, "0.75"))
				mock.ExpectQuery("cypher").WillReturnRows(rowsFor(idA, "1")) // category
				mock.ExpectQuery("cypher").WillReturnRows(rowsFor(idB, "5")) // brand
			},
			expectedErr: assert.NoError,
			expected: []retrieval.Candidate{
				{ProductID: idB, Rank: 1, Score: 5.75, Source: "graph"},
			},
		},
		{
			name:  "wraps a query error",
			query: retrieval.Query{SeedIDs: []string{seedID}},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("cypher").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("graph search"),
		},
		{
			name:  "wraps a category query error",
			query: retrieval.Query{Category: "Footwear"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("cypher").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("graph search"),
		},
		{
			name:  "wraps a brand query error",
			query: retrieval.Query{Brand: "Nike"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("cypher").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("graph search"),
		},
		{
			name:  "breaks proximity ties alphabetically",
			query: retrieval.Query{Category: "Footwear", Brand: "Nike"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectQuery("cypher").WillReturnRows(rowsFor(idB, "2"))
				mock.ExpectQuery("cypher").WillReturnRows(rowsFor(idA, "2"))
			},
			expectedErr: assert.NoError,
			expected: []retrieval.Candidate{
				{ProductID: idA, Rank: 1, Score: 2, Source: "graph"},
				{ProductID: idB, Rank: 2, Score: 2, Source: "graph"},
			},
		},
		{
			name:  "wraps a scan error",
			query: retrieval.Query{SeedIDs: []string{seedID}},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"product_id", "proximity"}).AddRow(true, "1")
				mock.ExpectQuery("cypher").WillReturnRows(rows)
			},
			expectedErr: errorContains("graph search: scan"),
		},
		{
			name:  "propagates a malformed product id",
			query: retrieval.Query{SeedIDs: []string{seedID}},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"product_id", "proximity"}).AddRow("not-json", "1")
				mock.ExpectQuery("cypher").WillReturnRows(rows)
			},
			expectedErr: errorContains("agtype: parse string"),
		},
		{
			name:  "propagates a malformed proximity count",
			query: retrieval.Query{SeedIDs: []string{seedID}},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := pgxmock.NewRows([]string{"product_id", "proximity"}).AddRow(`"`+idA+`"`, "not-a-number")
				mock.ExpectQuery("cypher").WillReturnRows(rows)
			},
			expectedErr: errorContains("agtype: parse float"),
		},
		{
			name:  "propagates a row iteration error",
			query: retrieval.Query{SeedIDs: []string{seedID}},
			setup: func(mock pgxmock.PgxPoolIface) {
				rows := rowsFor(idA, "1").CloseError(errors.New("connection dropped mid-stream"))
				mock.ExpectQuery("cypher").WillReturnRows(rows)
			},
			expectedErr: errorContains("connection dropped mid-stream"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			got, err := NewStore(mock).Search(context.Background(), tt.query)

			tt.expectedErr(t, err)
			if err == nil {
				assert.Equal(t, tt.expected, got)
			}
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestStore_UpsertProduct(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		product     ProductNode
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
	}{
		{
			name:    "upserts the bare product vertex",
			product: ProductNode{ProductID: "p1", Title: "Trail Runner"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnResult(pgxmock.NewResult("SELECT", 1))
			},
			expectedErr: assert.NoError,
		},
		{
			name:    "escapes quotes and backslashes before they reach cypher",
			product: ProductNode{ProductID: "p1", Title: `Runner's "Best" \ Pick`},
			setup: func(mock pgxmock.PgxPoolIface) {
				// cypherEscape doubles backslashes, then backslash-escapes quotes.
				escapedTitle := `Runner\'s "Best" \\ Pick`
				mock.ExpectExec(regexp.QuoteMeta(escapedTitle)).WillReturnResult(pgxmock.NewResult("SELECT", 1))
			},
			expectedErr: assert.NoError,
		},
		{
			name:    "links category and brand edges",
			product: ProductNode{ProductID: "p1", Title: "Trail Runner", Category: "Footwear", Brand: "Nike"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnResult(pgxmock.NewResult("SELECT", 1))
			},
			expectedErr: assert.NoError,
		},
		{
			name: "links attributes in sorted key order",
			product: ProductNode{
				ProductID:  "p1",
				Title:      "Trail Runner",
				Attributes: map[string]string{"waterproof": "true", "color": "red"},
			},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnResult(pgxmock.NewResult("SELECT", 1))
				mock.ExpectExec("color").WillReturnResult(pgxmock.NewResult("SELECT", 1))
				mock.ExpectExec("waterproof").WillReturnResult(pgxmock.NewResult("SELECT", 1))
			},
			expectedErr: assert.NoError,
		},
		{
			name:    "wraps an upsert error",
			product: ProductNode{ProductID: "p1", Title: "Trail Runner"},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("graph upsert product"),
		},
		{
			name: "wraps a link attribute error",
			product: ProductNode{
				ProductID:  "p1",
				Title:      "Trail Runner",
				Attributes: map[string]string{"waterproof": "true"},
			},
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnResult(pgxmock.NewResult("SELECT", 1))
				mock.ExpectExec("waterproof").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("graph link attribute"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			err = NewStore(mock).UpsertProduct(context.Background(), tt.product)

			tt.expectedErr(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestStore_LinkRelated(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		setup       func(mock pgxmock.PgxPoolIface)
		expectedErr assert.ErrorAssertionFunc
	}{
		{
			name: "links two products",
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnResult(pgxmock.NewResult("SELECT", 1))
			},
			expectedErr: assert.NoError,
		},
		{
			name: "wraps an error",
			setup: func(mock pgxmock.PgxPoolIface) {
				mock.ExpectExec("cypher").WillReturnError(errors.New("connection reset"))
			},
			expectedErr: errorContains("graph link related"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock, err := pgxmock.NewPool()
			require.NoError(t, err)
			defer mock.Close()
			tt.setup(mock)

			err = NewStore(mock).LinkRelated(context.Background(), "p1", "p2", 0.8)

			tt.expectedErr(t, err)
			assert.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestStore_Name(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "graph", new(Store).Name())
}
