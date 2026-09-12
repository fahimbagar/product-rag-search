// Package router wires HTTP handlers for the product search API.
package router

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/fahimbagar/product-rag-search/internal/healthcheck"
	"github.com/fahimbagar/product-rag-search/internal/ingestion"
	"github.com/fahimbagar/product-rag-search/internal/pipeline"
)

// querier is the query-serving seam handler.query() depends on. Satisfied
// by *pipeline.Pipeline; a local interface so tests can fake it without
// building a real Pipeline.
type querier interface {
	Query(ctx context.Context, query string) (pipeline.Result, error)
}

// productIngester is the ingest-serving seam handler.ingest() depends on.
// Satisfied by *ingestion.Ingester; a local interface for the same reason
// as querier.
type productIngester interface {
	Ingest(ctx context.Context, raw []ingestion.RawProduct, related []ingestion.RelatedPair) error
}

// handler holds the dependencies shared by the query and ingest routes.
type handler struct {
	pipeline   querier
	ingester   productIngester
	adminToken string
}

// New builds the HTTP handler for the service.
func New(logger *zap.SugaredLogger, hc *healthcheck.HealthCheck, p querier, ingester productIngester, adminToken string) http.Handler {
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
