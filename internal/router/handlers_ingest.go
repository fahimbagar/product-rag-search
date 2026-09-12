package router

import (
	"encoding/json"
	"net/http"

	"github.com/fahimbagar/product-rag-search/internal/ingestion"
	"github.com/fahimbagar/product-rag-search/pkg/ctxlog"
)

type ingestProduct struct {
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Category    string         `json:"category"`
	Brand       string         `json:"brand"`
	Price       float64        `json:"price"`
	Attributes  map[string]any `json:"attributes"`
}

type ingestRelation struct {
	FromIndex int     `json:"from_index"`
	ToIndex   int     `json:"to_index"`
	Weight    float64 `json:"weight"`
}

type ingestRequest struct {
	Products  []ingestProduct  `json:"products"`
	Relations []ingestRelation `json:"relations"`
}

func (h handler) ingest() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if h.adminToken == "" || r.Header.Get("Authorization") != "Bearer "+h.adminToken {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		var req ingestRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}
		if len(req.Products) == 0 {
			http.Error(w, "products is required", http.StatusBadRequest)
			return
		}

		raw := make([]ingestion.RawProduct, len(req.Products))
		for i, p := range req.Products {
			raw[i] = ingestion.RawProduct{
				Title:       p.Title,
				Description: p.Description,
				Category:    p.Category,
				Brand:       p.Brand,
				Price:       p.Price,
				Attributes:  p.Attributes,
			}
		}
		related := make([]ingestion.RelatedPair, len(req.Relations))
		for i, rel := range req.Relations {
			related[i] = ingestion.RelatedPair{FromIndex: rel.FromIndex, ToIndex: rel.ToIndex, Weight: rel.Weight}
		}

		if err := h.ingester.Ingest(r.Context(), raw, related); err != nil {
			ctxlog.FromContext(r.Context()).Errorw("ingest failed", "error", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ingested": len(raw)})
	}
}
