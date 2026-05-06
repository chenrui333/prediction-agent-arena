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
	_ = s.Events.Append(r.Context(), "admin", "admin", "admin_action", map[string]interface{}{"action": "create_team", "team_id": team.ID, "team_slug": team.Slug})
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
	_ = s.Events.Append(r.Context(), roundSlug, "admin", "admin_action", map[string]interface{}{"action": action, "team_id": team.ID, "team_slug": team.Slug})
	writeJSON(w, http.StatusOK, team)
}

func (s *Server) resetTeam(w http.ResponseWriter, r *http.Request) {
	id, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
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
	_ = s.Events.Append(r.Context(), roundSlug, "admin", "admin_action", map[string]interface{}{"action": "reset_team", "team_id": id})
	writeJSON(w, http.StatusOK, map[string]string{"status": "reset"})
}
