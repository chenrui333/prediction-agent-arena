package httpapi

import (
	"encoding/json"
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type createAgentRequest struct {
	Slug         string          `json:"slug"`
	Name         string          `json:"name"`
	Kind         string          `json:"kind"`
	RepoURL      string          `json:"repo_url"`
	CommitSHA    string          `json:"commit_sha"`
	DockerImage  string          `json:"docker_image"`
	Metadata     json.RawMessage `json:"metadata"`
	MetadataJSON string          `json:"metadata_json"`
}

type agentTokenResponse struct {
	Agent    store.Agent `json:"agent"`
	APIToken string      `json:"api_token,omitempty"`
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}
	if _, err := s.Store.GetTeam(r.Context(), teamID); err != nil {
		writeError(w, http.StatusNotFound, "team_not_found", "team not found")
		return
	}
	var req createAgentRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	metadata := req.MetadataJSON
	if len(req.Metadata) > 0 {
		metadata = string(req.Metadata)
	}
	if metadata == "" {
		metadata = "{}"
	}
	token, err := auth.GenerateAgentToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_failed", "failed to generate token")
		return
	}
	agent, err := s.Store.CreateAgent(r.Context(), store.AgentInput{
		TeamID:       teamID,
		Slug:         req.Slug,
		Name:         req.Name,
		Status:       "active",
		Kind:         req.Kind,
		RepoURL:      req.RepoURL,
		CommitSHA:    req.CommitSHA,
		DockerImage:  req.DockerImage,
		MetadataJSON: metadata,
	}, auth.HashToken(token))
	if err != nil {
		writeError(w, http.StatusBadRequest, "agent_failed", err.Error())
		return
	}
	s.recordAdminAction(r.Context(), "admin", "create_agent", nil, &teamID, map[string]interface{}{"agent_id": agent.ID, "agent_slug": agent.Slug})
	writeJSON(w, http.StatusCreated, agentTokenResponse{Agent: agent, APIToken: token})
}

func (s *Server) listTeamAgents(w http.ResponseWriter, r *http.Request) {
	teamID, err := parseParamID(r, "team_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_team_id", "team_id must be a positive integer")
		return
	}
	agents, err := s.Store.ListTeamAgents(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "agents_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}

func (s *Server) pauseAgent(w http.ResponseWriter, r *http.Request) {
	s.setAgentStatus(w, r, "paused", "pause_agent")
}

func (s *Server) resumeAgent(w http.ResponseWriter, r *http.Request) {
	s.setAgentStatus(w, r, "active", "resume_agent")
}

func (s *Server) revokeAgent(w http.ResponseWriter, r *http.Request) {
	s.setAgentStatus(w, r, "revoked", "revoke_agent")
}

func (s *Server) setAgentStatus(w http.ResponseWriter, r *http.Request, status, action string) {
	agentID, err := parseParamID(r, "agent_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_agent_id", "agent_id must be a positive integer")
		return
	}
	agent, err := s.Store.SetAgentStatus(r.Context(), agentID, status)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found", "agent not found")
		return
	}
	teamID := agent.TeamID
	s.recordAdminAction(r.Context(), "admin", action, nil, &teamID, map[string]interface{}{"agent_id": agent.ID, "agent_slug": agent.Slug})
	writeJSON(w, http.StatusOK, agent)
}

func (s *Server) rotateAgentToken(w http.ResponseWriter, r *http.Request) {
	agentID, err := parseParamID(r, "agent_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_agent_id", "agent_id must be a positive integer")
		return
	}
	token, err := auth.GenerateAgentToken()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "token_failed", "failed to generate token")
		return
	}
	agent, err := s.Store.UpdateAgentTokenHash(r.Context(), agentID, auth.HashToken(token))
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found", "agent not found")
		return
	}
	teamID := agent.TeamID
	s.recordAdminAction(r.Context(), "admin", "rotate_agent_token", nil, &teamID, map[string]interface{}{"agent_id": agent.ID, "agent_slug": agent.Slug})
	writeJSON(w, http.StatusOK, agentTokenResponse{Agent: agent, APIToken: token})
}
