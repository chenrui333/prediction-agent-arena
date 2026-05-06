package httpapi

import (
	"net/http"
	"strconv"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type adminSummaryResponse struct {
	ActiveRound *store.Round           `json:"active_round"`
	LatestRound *store.Round           `json:"latest_round"`
	Teams       []store.AdminTeamStats `json:"teams"`
	Policy      interface{}            `json:"risk_policy"`
}

func (s *Server) adminSummary(w http.ResponseWriter, r *http.Request) {
	round, err := s.roundForAdminSummary(r)
	if err != nil {
		writeJSON(w, http.StatusOK, adminSummaryResponse{Teams: []store.AdminTeamStats{}, Policy: s.Policy})
		return
	}
	teams, err := s.Store.ListAdminTeamStats(r.Context(), round.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_summary_failed", err.Error())
		return
	}
	activeRound, activeErr := s.Store.GetActiveRound(r.Context())
	var active *store.Round
	if activeErr == nil {
		active = &activeRound
	}
	writeJSON(w, http.StatusOK, adminSummaryResponse{ActiveRound: active, LatestRound: &round, Teams: teams, Policy: s.Policy})
}

func (s *Server) roundForAdminSummary(r *http.Request) (store.Round, error) {
	if idValue := r.URL.Query().Get("round_id"); idValue != "" {
		id, err := strconv.ParseInt(idValue, 10, 64)
		if err != nil {
			return store.Round{}, err
		}
		return s.Store.GetRound(r.Context(), id)
	}
	if active, err := s.Store.GetActiveRound(r.Context()); err == nil {
		return active, nil
	}
	return s.Store.GetLatestRound(r.Context())
}

func (s *Server) exportRound(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	if err := s.Store.RefreshRoundScores(r.Context(), roundID); err != nil {
		writeError(w, http.StatusInternalServerError, "score_refresh_failed", err.Error())
		return
	}
	result, err := s.Store.ExportRound(r.Context(), roundID, s.ExportDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) freezeLeaderboard(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	if err := s.Store.RefreshRoundScores(r.Context(), roundID); err != nil {
		writeError(w, http.StatusInternalServerError, "score_refresh_failed", err.Error())
		return
	}
	result, err := s.Store.ExportRound(r.Context(), roundID, s.ExportDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "freeze_failed", err.Error())
		return
	}
	round, _ := s.Store.GetRound(r.Context(), roundID)
	_ = s.Events.Append(r.Context(), round.Slug, "admin", "admin_action", map[string]interface{}{"action": "freeze_leaderboard", "round_id": roundID, "artifacts": result.Artifacts})
	writeJSON(w, http.StatusOK, result)
}
