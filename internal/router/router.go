// Package router wires HTTP handlers for the product search API.
package router

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/fahimbagar/product-rag-search/internal/healthcheck"
	"github.com/fahimbagar/product-rag-search/internal/ingestion"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
)

// handler holds the dependencies shared by the query and ingest routes.
type handler struct {
	pipeline   *pipeline.Pipeline
	ingester   *ingestion.Ingester
	adminToken string
}

// New builds the HTTP handler for the service.
func New(logger *zap.SugaredLogger, hc *healthcheck.HealthCheck, p *pipeline.Pipeline, ingester *ingestion.Ingester, adminToken string) http.Handler {
	h := handler{
		pipeline:   p,
		ingester:   ingester,
		adminToken: adminToken,
	}

	r := chi.NewRouter()
	r.Use(withMiddleware(logger))

	r.Get("/health", hc.Handler())
	r.Post("/query", h.query())
	r.Post("/ingest", h.ingest())
	r.Handle("/metrics", promhttp.Handler())

	return r
}
