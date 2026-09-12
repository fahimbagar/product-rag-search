package router

import (
	"bytes"
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"github.com/fahimbagar/product-rag-search/internal/intent"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
)

type mockQuerier struct{ mock.Mock }

func (m *mockQuerier) Query(ctx context.Context, query string) (pipeline.Result, error) {
	args := m.Called(ctx, query)
	res, _ := args.Get(0).(pipeline.Result)
	return res, args.Error(1)
}

func TestHandler_Query(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		body           string
		setup          func(m *mockQuerier)
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "invalid JSON body",
			body:           `{`,
			setup:          func(m *mockQuerier) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "invalid request body\n",
		},
		{
			name:           "empty query",
			body:           `{"query":""}`,
			setup:          func(m *mockQuerier) {},
			expectedStatus: http.StatusBadRequest,
			expectedBody:   "query is required\n",
		},
		{
			name: "pipeline error",
			body: `{"query":"running shoes"}`,
			setup: func(m *mockQuerier) {
				m.On("Query", mock.Anything, "running shoes").
					Return(pipeline.Result{}, errors.New("pipeline unavailable"))
			},
			expectedStatus: http.StatusInternalServerError,
			expectedBody:   "internal server error\n",
		},
		{
			name: "success",
			body: `{"query":"running shoes"}`,
			setup: func(m *mockQuerier) {
				m.On("Query", mock.Anything, "running shoes").Return(pipeline.Result{
					Intent: intent.ProductSearch,
					Answer: "The Trail Runner is a great fit.",
					Products: []pipeline.ProductResult{
						{ID: "1", Title: "Trail Runner", Brand: "Nike", Category: "Footwear", Price: 89.99, Cited: true},
					},
				}, nil)
			},
			expectedStatus: http.StatusOK,
			expectedBody: `{"intent":"product_search","answer":"The Trail Runner is a great fit.",` +
				`"products":[{"id":"1","title":"Trail Runner","brand":"Nike","category":"Footwear","price":89.99,"cited":true}]}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := new(mockQuerier)
			tt.setup(q)
			h := handler{pipeline: q}

			req := httptest.NewRequest(http.MethodPost, "/query", bytes.NewBufferString(tt.body))
			rec := httptest.NewRecorder()

			h.query()(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			if strings.HasPrefix(rec.Header().Get("Content-Type"), "application/json") {
				assert.JSONEq(t, tt.expectedBody, rec.Body.String())
			} else {
				assert.Equal(t, tt.expectedBody, rec.Body.String())
			}
			q.AssertExpectations(t)
		})
	}
}
