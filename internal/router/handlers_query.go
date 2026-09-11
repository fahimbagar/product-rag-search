package router

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/fahimbagar/product-rag-search/internal/pipeline"
)

func handleQuery(logger *slog.Logger, p *pipeline.Pipeline) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req queryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if req.Query == "" {
			http.Error(w, "query is required", http.StatusBadRequest)
			return
		}

		result, err := p.Query(r.Context(), req.Query)
		if err != nil {
			logger.Error("query failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, toQueryResponse(result))
	}
}
