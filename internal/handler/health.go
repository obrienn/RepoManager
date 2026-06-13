package handler

import (
	"encoding/json"
	"net/http"

	"repomanager/internal/store"
)

func Health(s *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]string{"status": "ok", "database": "ok"}
		if err := s.Ping(r.Context()); err != nil {
			resp["database"] = "error: " + err.Error()
			w.WriteHeader(http.StatusServiceUnavailable)
		}
		json.NewEncoder(w).Encode(resp)
	}
}
