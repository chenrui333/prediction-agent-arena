package httpapi

import (
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
)

type createTeamRequest struct {
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type createTeamResponse struct {
	ID       int64  `json:"id"`
	Slug     string `json:"slug"`
	Name     string `json:"name"`
	APIToken string `json:"api_token"`
}

func (s *Server) createTeam(w http.ResponseWriter, r *http.Request) {
	var req createTeamRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	if req.Name == "" {
		req.Name = req.Slug
	}
	token, err := auth.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_failed", "failed to generate token")
		return
	}
	team, err := s.Store.CreateTeam(r.Context(), req.Slug, req.Name, auth.HashToken(token))
	if err != nil {
		writeError(w, http.StatusBadRequest, "team_failed", err.Error())
		return
	}
	teamID := team.ID
	s.recordAdminAction(r.Context(), "admin", "create_team", nil, &teamID, map[string]interface{}{"team_slug": team.Slug})
	writeJSON(w, http.StatusCreated, createTeamResponse{ID: team.ID, Slug: team.Slug, Name: team.Name, APIToken: token})
}

func (s *Server) listTeams(w http.ResponseWriter, r *http.Request) {
	teams, err := s.Store.ListTeams(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "teams_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, teams)
}

func (s *Server) pauseTeam(w http.ResponseWriter, r *http.Request) {
	s.setTeamActive(w, r, false)
}

func (s *Server) resumeTeam(w http.ResponseWriter, r *http.Request) {
	s.setTeamActive(w, r, true)
}

func (s *Server) setTeamActive(w http.ResponseWriter, r *http.Request, active bool) {
	id, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}
	team, err := s.Store.SetTeamActive(r.Context(), id, active)
	if err != nil {
		writeError(w, http.StatusNotFound, "team_not_found", "team not found")
		return
	}
	action := "resume_team"
	if !active {
		action = "pause_team"
	}
	roundSlug := "admin"
	if round, err := s.Store.GetLatestRound(r.Context()); err == nil {
		roundSlug = round.Slug
	}
	teamID := team.ID
	s.recordAdminAction(r.Context(), roundSlug, action, nil, &teamID, map[string]interface{}{"team_slug": team.Slug})
	writeJSON(w, http.StatusOK, team)
}

func (s *Server) rotateTeamToken(w http.ResponseWriter, r *http.Request) {
	id, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}
	token, err := auth.GenerateToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_failed", "failed to generate token")
		return
	}
	team, err := s.Store.UpdateTeamTokenHash(r.Context(), id, auth.HashToken(token))
	if err != nil {
		writeError(w, http.StatusNotFound, "team_not_found", "team not found")
		return
	}
	teamID := team.ID
	s.recordAdminAction(r.Context(), "admin", "rotate_team_token", nil, &teamID, map[string]interface{}{"team_slug": team.Slug})
	writeJSON(w, http.StatusOK, createTeamResponse{ID: team.ID, Slug: team.Slug, Name: team.Name, APIToken: token})
}

func (s *Server) resetTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}
	var input struct {
		Confirm string `json:"confirm"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(r, &input); err != nil {
			writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
			return
		}
	}
	if input.Confirm != "all_rounds" {
		writeErrorDetails(w, http.StatusBadRequest, "reset_requires_confirmation", "all-round team reset requires confirm=all_rounds; use the round-scoped reset endpoint by default", map[string]interface{}{"team_id": id})
		return
	}
	if err := s.Store.ResetTeam(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "reset_failed", err.Error())
		return
	}
	roundSlug := "admin"
	if round, err := s.Store.GetLatestRound(r.Context()); err == nil {
		roundSlug = round.Slug
	}
	s.recordAdminAction(r.Context(), roundSlug, "reset_team_all_rounds", nil, &id, nil)
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}

func (s *Server) resetTeamRound(w http.ResponseWriter, r *http.Request) {
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
	team, err := s.Store.GetTeam(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team_not_found", "team not found")
		return
	}
	if err := s.Store.ResetTeamRound(r.Context(), roundID, teamID); err != nil {
		writeError(w, http.StatusInternalServerError, "reset_failed", err.Error())
		return
	}
	s.invalidateLeaderboard(r.Context(), roundID)
	s.recordAdminAction(r.Context(), round.Slug, "reset_team_round", &roundID, &teamID, map[string]interface{}{"team_slug": team.Slug})
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset", "round_slug": round.Slug, "team_slug": team.Slug})
}
