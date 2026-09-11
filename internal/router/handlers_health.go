package router

import (
	"encoding/json"
	"net/http"

	"github.com/jackc/pgx/v5/pgxpool"
)

type healthResponse struct {
	Status string `json:"status"`
}

func handleHealth(db *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := db.Ping(r.Context()); err != nil {
			writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
			return
		}
		writeJSON(w, http.StatusOK, healthResponse{Status: "ok"})
	}
}

func writeJSON(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}
