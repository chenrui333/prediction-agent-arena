package httpapi

import (
	"errors"
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

func (s *Server) listRoundTeams(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	items, err := s.Store.ListRoundTeams(r.Context(), roundID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "round_teams_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, items)
}

func (s *Server) enrollRoundTeam(w http.ResponseWriter, r *http.Request) {
	s.setRoundTeamStatus(w, r, "active", "enroll_round_team")
}

func (s *Server) pauseRoundTeam(w http.ResponseWriter, r *http.Request) {
	s.setRoundTeamStatus(w, r, "paused", "pause_round_team")
}

func (s *Server) resumeRoundTeam(w http.ResponseWriter, r *http.Request) {
	s.setRoundTeamStatus(w, r, "active", "resume_round_team")
}

func (s *Server) withdrawRoundTeam(w http.ResponseWriter, r *http.Request) {
	s.setRoundTeamStatus(w, r, "withdrawn", "withdraw_round_team")
}

func (s *Server) setRoundTeamStatus(w http.ResponseWriter, r *http.Request, status, action string) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	teamID, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}
	round, err := s.Store.GetRound(r.Context(), roundID)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	if _, err := s.Store.GetTeam(r.Context(), teamID); err != nil {
		writeError(w, http.StatusNotFound, "team_not_found", "team not found")
		return
	}
	var item store.RoundTeam
	if action == "enroll_round_team" {
		item, err = s.Store.EnrollRoundTeam(r.Context(), store.RoundTeamInput{RoundID: roundID, TeamID: teamID, Status: status})
	} else {
		item, err = s.Store.SetRoundTeamStatus(r.Context(), roundID, teamID, status)
	}
	if err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "round_team_not_found", "team is not enrolled in this round")
			return
		}
		writeError(w, http.StatusBadRequest, "round_team_failed", err.Error())
		return
	}
	s.recordAdminAction(r.Context(), round.Slug, action, &roundID, &teamID, map[string]interface{}{"round_team_status": item.Status})
	writeJSON(w, http.StatusOK, item)
}
