package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

type pingerFunc func(ctx context.Context) error

func (f pingerFunc) Ping(ctx context.Context) error { return f(ctx) }

func TestHandler_Healthy(t *testing.T) {
	t.Parallel()

	hc := New(pingerFunc(func(ctx context.Context) error { return nil }))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	hc.Handler()(rec, req)

	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"ok"}`, rec.Body.String())
}

func TestHandler_Unavailable(t *testing.T) {
	t.Parallel()

	hc := New(pingerFunc(func(ctx context.Context) error { return errors.New("connection refused") }))

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	hc.Handler()(rec, req)

	assert.Equal(t, http.StatusServiceUnavailable, rec.Code)
	assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
	assert.JSONEq(t, `{"status":"unavailable"}`, rec.Body.String())
}

func TestHandler_UsesRequestContext(t *testing.T) {
	t.Parallel()

	type ctxKey struct{}
	var gotCtx context.Context
	hc := New(pingerFunc(func(ctx context.Context) error {
		gotCtx = ctx
		return nil
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req = req.WithContext(context.WithValue(req.Context(), ctxKey{}, "marker"))
	hc.Handler()(httptest.NewRecorder(), req)

	assert.Equal(t, "marker", gotCtx.Value(ctxKey{}))
}
