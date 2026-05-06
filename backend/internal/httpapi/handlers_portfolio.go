package httpapi

import "net/http"

func (s *Server) getPortfolio(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	round, err := s.Store.GetActiveRound(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "no active round")
		return
	}
	snapshot, err := s.Store.CreatePortfolioSnapshot(r.Context(), round.ID, team.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "portfolio_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"round": round, "team": team, "portfolio": snapshot})
}

func (s *Server) listFills(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	round, err := s.Store.GetActiveRound(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "no active round")
		return
	}
	fills, err := s.Store.ListFills(r.Context(), round.ID, team.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "fills_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"round": round, "fills": fills})
}
