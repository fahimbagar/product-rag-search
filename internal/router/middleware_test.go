package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"github.com/fahimbagar/product-rag-search/internal/ctxlog"
)

func TestWithMiddleware(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		next           http.HandlerFunc
		expectedStatus int
	}{
		{
			name: "passes through and records the handler's status",
			next: func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusTeapot)
			},
			expectedStatus: http.StatusTeapot,
		},
		{
			name: "defaults to 200 when the handler never calls WriteHeader",
			next: func(w http.ResponseWriter, r *http.Request) {
				_, _ = w.Write([]byte("ok"))
			},
			expectedStatus: http.StatusOK,
		},
		{
			name: "recovers a panic as a 500",
			next: func(w http.ResponseWriter, r *http.Request) {
				panic("boom")
			},
			expectedStatus: http.StatusInternalServerError,
		},
		{
			name: "attaches a request-scoped logger to the context",
			next: func(w http.ResponseWriter, r *http.Request) {
				logger := ctxlog.FromContext(r.Context())
				if logger == nil {
					w.WriteHeader(http.StatusInternalServerError)
					return
				}
				w.WriteHeader(http.StatusOK)
			},
			expectedStatus: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logger := zap.NewNop().Sugar()
			mw := withMiddleware(logger)(tt.next)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			rec := httptest.NewRecorder()

			assert.NotPanics(t, func() { mw.ServeHTTP(rec, req) })
			assert.Equal(t, tt.expectedStatus, rec.Code)
		})
	}
}
