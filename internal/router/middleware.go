package router

import (
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"

	"github.com/fahimbagar/product-rag-search/internal/metrics"
)

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func withMiddleware(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requestID := uuid.NewString()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}

			start := time.Now()
			defer func() {
				if rr := recover(); rr != nil {
					logger.Error("panic recovered", "request_id", requestID, "panic", rr)
					http.Error(rec, "internal server error", http.StatusInternalServerError)
				}
				duration := time.Since(start)
				logger.Info("request",
					"request_id", requestID,
					"method", r.Method,
					"path", r.URL.Path,
					"status", rec.status,
					"duration_ms", duration.Milliseconds(),
				)
				metrics.HTTPRequestsTotal.WithLabelValues(r.Method, r.URL.Path, strconv.Itoa(rec.status)).Inc()
				metrics.HTTPRequestDuration.WithLabelValues(r.Method, r.URL.Path).Observe(duration.Seconds())
			}()

			next.ServeHTTP(rec, r)
		})
	}
}
