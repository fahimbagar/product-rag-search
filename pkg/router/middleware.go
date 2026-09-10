package router

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/google/uuid"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withMiddleware(logger *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := uuid.NewString()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

		start := time.Now()
		defer func() {
			if rr := recover(); rr != nil {
				logger.Error("panic recovered", "request_id", requestID, "panic", rr)
				http.Error(rec, "internal server error", http.StatusInternalServerError)
			}
			logger.Info("request",
				"request_id", requestID,
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		}()

		next.ServeHTTP(rec, r)
	})
}
