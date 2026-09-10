// Package router wires HTTP handlers for the product search API.
package router

import (
	"log/slog"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/fahimbagar/product-rag-search/pkg/ingestion"
	"github.com/fahimbagar/product-rag-search/pkg/pipeline"
)

// New builds the HTTP handler for the service.
func New(logger *slog.Logger, db *pgxpool.Pool, p *pipeline.Pipeline, ingester *ingestion.Ingester, adminToken string) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", handleHealth(db))
	mux.HandleFunc("POST /query", handleQuery(p))
	mux.HandleFunc("POST /ingest", handleIngest(ingester, adminToken))

	return withMiddleware(logger, mux)
}
