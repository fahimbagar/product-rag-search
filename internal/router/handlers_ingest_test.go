package router

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fahimbagar/product-rag-search/internal/ingestion"
)

type mockIngester struct{ mock.Mock }

func (m *mockIngester) Ingest(ctx context.Context, raw []ingestion.RawProduct, related []ingestion.RelatedPair) error {
	args := m.Called(ctx, raw, related)
	return args.Error(0)
}

func TestHandler_Ingest(t *testing.T) {
	t.Parallel()

	validBody := `{"products":[{"title":"Trail Runner","brand":"Nike","price":89.99}],` +
		`"relations":[{"from_index":0,"to_index":0,"weight":1}]}`

	tests := []struct {
		name           string
		adminToken     string
		authHeader     string
		body           string
		setup          func(m *mockIngester)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "missing admin token configured",
			adminToken:     "",
			authHeader:     "Bearer secret",
			body:           validBody,
			setup:          func(m *mockIngester) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized\n",
		},
		{
			name:           "wrong bearer token",
			adminToken:     "secret",
			authHeader:     "Bearer wrong",
			body:           validBody,
			setup:          func(m *mockIngester) {},
			expectedStatus: http.StatusUnauthorized,
			expectedBody:   "unauthorized\n",
		},
		{
			name:           "invalid JSON body",
			adminToken:     "secret",
			authHeader:     "Bearer secret",
			body:           `{`,
			setup:          func(m *mockIngester) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid request body\n",
		},
		{
			name:           "no products",
			adminToken:     "secret",
			authHeader:     "Bearer secret",
			body:           `{"products":[]}`,
			setup:          func(m *mockIngester) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "products is required\n",
		},
		{
			name:       "ingester error",
			adminToken: "secret",
			authHeader: "Bearer secret",
			body:       validBody,
			setup: func(m *mockIngester) {
				m.On("Ingest", mock.Anything, mock.Anything, mock.Anything).
					Return(errors.New("ingest unavailable"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error\n",
		},
		{
			name:       "success maps products and relations",
			adminToken: "secret",
			authHeader: "Bearer secret",
			body:       validBody,
			setup: func(m *mockIngester) {
				m.On("Ingest", mock.Anything,
					[]ingestion.RawProduct{{Title: "Trail Runner", Brand: "Nike", Price: 89.99}},
					[]ingestion.RelatedPair{{FromIndex: 0, ToIndex: 0, Weight: 1}},
				).Return(nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody:   `{"ingested":1}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ing := new(mockIngester)
			tt.setup(ing)
			h := handler{ingester: ing, adminToken: tt.adminToken}

			req := httptest.NewRequest(http.MethodPost, "/ingest", bytes.NewBufferString(tt.body))
			req.Header.Set("Authorization", tt.authHeader)
			rec := httptest.NewRecorder()

			h.ingest()(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, tt.expectedBody, rec.Body.String())
			ing.AssertExpectations(t)
		})
	}
}
