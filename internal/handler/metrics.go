package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"repomanager/internal/store"
)

func Metrics(s *store.Store, nextUpdate *time.Time) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		nu := time.Time{}
		if nextUpdate != nil {
			nu = *nextUpdate
		}
		m, err := s.GetMetrics(r.Context(), nu)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(m)
	}
}
