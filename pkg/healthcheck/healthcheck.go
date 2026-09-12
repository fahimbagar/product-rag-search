// Package healthcheck exposes a health-check HTTP handler for a pingable
// dependency, so callers don't need to pass that dependency's concrete type
// around just to expose a health endpoint.
package healthcheck

import (
	"context"
	"encoding/json"
	"net/http"
)

// Pinger reports whether a dependency is reachable.
type Pinger interface {
	Ping(ctx context.Context) error
}

// HealthCheck reports liveness for a single Pinger.
type HealthCheck struct {
	pinger Pinger
}

// New builds a HealthCheck backed by the given Pinger.
func New(pinger Pinger) *HealthCheck {
	return &HealthCheck{pinger: pinger}
}

type response struct {
	Status string `json:"status"`
}

// Handler returns an http.HandlerFunc that pings the dependency and reports
// "ok" (200) or "unavailable" (503).
func (h *HealthCheck) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		status := http.StatusOK
		body := response{Status: "ok"}
		if err := h.pinger.Ping(r.Context()); err != nil {
			status = http.StatusServiceUnavailable
			body = response{Status: "unavailable"}
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}
