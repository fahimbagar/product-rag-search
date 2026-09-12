package healthcheck

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type mockPinger struct {
	mock.Mock
}

func (m *mockPinger) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func TestHandler(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name           string
		pingErr        error
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "healthy",
			pingErr:        nil,
			expectedStatus: http.StatusOK,
			expectedBody:   `{"status":"ok"}`,
		},
		{
			name:           "unavailable",
			pingErr:        errors.New("connection refused"),
			expectedStatus: http.StatusServiceUnavailable,
			expectedBody:   `{"status":"unavailable"}`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pinger := new(mockPinger)
			req := httptest.NewRequest(http.MethodGet, "/health", nil)
			pinger.On("Ping", req.Context()).Return(tt.pingErr)

			rec := httptest.NewRecorder()
			New(pinger).Handler()(rec, req)

			assert.Equal(t, tt.expectedStatus, rec.Code)
			assert.Equal(t, "application/json", rec.Header().Get("Content-Type"))
			assert.JSONEq(t, tt.expectedBody, rec.Body.String())
			pinger.AssertExpectations(t)
		})
	}
}
