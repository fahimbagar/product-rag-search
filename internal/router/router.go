// Package router wires HTTP handlers for the product search API.
package router

import (
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/fahimbagar/product-rag-search/internal/ingestion"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
	"github.com/fahimbagar/product-rag-search/pkg/healthcheck"
)

// New builds the HTTP handler for the service.
func New(logger *slog.Logger, hc *healthcheck.HealthCheck, p *pipeline.Pipeline, ingester *ingestion.Ingester, adminToken string) http.Handler {
	r := chi.NewRouter()
	r.Use(withMiddleware(logger))

	r.Get("/health", hc.Handler())
	r.Post("/query", handleQuery(logger, p))
	r.Post("/ingest", handleIngest(logger, ingester, adminToken))
	r.Handle("/metrics", promhttp.Handler())

	return r
}
