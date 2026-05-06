package httpapi

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func (s *Server) getTeamActivity(w http.ResponseWriter, r *http.Request) {
	teamSlug := chi.URLParam(r, "team_slug")
	round, err := s.roundFromQuery(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	limit := 25
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
			return
		}
		limit = parsed
	}
	activity, err := s.Store.TeamActivity(r.Context(), teamSlug, round.ID, limit)
	if err != nil {
		writeErrorDetails(w, http.StatusNotFound, "team_not_found", "team or round not found", map[string]interface{}{"team_slug": teamSlug, "round_id": round.ID})
		return
	}
	writeJSON(w, http.StatusOK, activity)
}
