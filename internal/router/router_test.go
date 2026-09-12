package router

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.uber.org/zap"

	"github.com/fahimbagar/product-rag-search/internal/healthcheck"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
)

type mockPinger struct{ mock.Mock }

func (m *mockPinger) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		method         string
		path           string
		body           string
		authHeader     string
		expectedStatus int
	}{
		{name: "health route is wired", method: http.MethodGet, path: "/health", expectedStatus: http.StatusOK},
		{name: "metrics route is wired", method: http.MethodGet, path: "/metrics", expectedStatus: http.StatusOK},
		{
			name:           "query route is wired",
			method:         http.MethodPost,
			path:           "/query",
			body:           `{"query":"running shoes"}`,
			expectedStatus: http.StatusOK,
		},
		{
			name:           "ingest route is wired and enforces the admin token",
			method:         http.MethodPost,
			path:           "/ingest",
			body:           `{"products":[{"title":"Trail Runner"}]}`,
			authHeader:     "Bearer wrong",
			expectedStatus: http.StatusUnauthorized,
		},
		{name: "unknown route 404s", method: http.MethodGet, path: "/nope", expectedStatus: http.StatusNotFound},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pinger := new(mockPinger)
			pinger.On("Ping", mock.Anything).Return(nil)

			q := new(mockQuerier)
			q.On("Query", mock.Anything, mock.Anything).Return(pipeline.Result{}, nil)

			handler := New(zap.NewNop().Sugar(), healthcheck.New(pinger), q, new(mockIngester), "secret")

			req := httptest.NewRequest(tt.method, tt.path, bytes.NewBufferString(tt.body))
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()

			handler.ServeHTTP(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
