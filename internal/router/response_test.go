package router

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriteJSON(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name         string
		status       int
		body         any
		expectedBody string
	}{
		{
			name:         "encodes a struct body",
			status:       http.StatusOK,
			body:         struct{ Status string }{Status: "ok"},
			expectedBody: `{"Status":"ok"}` + "\n",
		},
		{
			name:         "encodes a map body with a non-200 status",
			status:       http.StatusServiceUnavailable,
			body:         map[string]any{"status": "unavailable"},
			expectedBody: `{"status":"unavailable"}` + "\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := httptest.NewRecorder()

			writeJSON(rec, tt.status, tt.body)

			assert.Equal(t, tt.status, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.expectedBody, rec.Body.String())
		})
	}
}
