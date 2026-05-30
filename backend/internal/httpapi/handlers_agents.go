package httpapi

import (
	"encoding/json"
	"errors"
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
	if err := decodeJSON(w, r, &req); err != nil {
		writeDecodeError(w, err)
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

func (s *Server) lockRoundAgent(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	agentID, err := parseParamID(r, "agent_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_agent_id", "agent_id must be a positive integer")
		return
	}
	var req struct {
		CommitSHA    string          `json:"commit_sha"`
		DockerImage  string          `json:"docker_image"`
		Metadata     json.RawMessage `json:"metadata"`
		MetadataJSON string          `json:"metadata_json"`
		LockedBy     string          `json:"locked_by"`
		Confirm      string          `json:"confirm"`
	}
	if r.ContentLength != 0 {
		if err := decodeJSON(w, r, &req); err != nil {
			writeDecodeError(w, err)
			return
		}
	}
	round, err := s.Store.GetRound(r.Context(), roundID)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	agent, err := s.Store.GetAgent(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found", "agent not found")
		return
	}
	if round.Status == "completed" {
		writeErrorDetails(w, http.StatusConflict, "round_agent_lock_immutable", "round agent locks cannot be changed after round completion", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug})
		return
	}
	existing, existingErr := s.Store.GetRoundAgentForTeam(r.Context(), round.ID, agent.TeamID)
	if existingErr != nil && !errors.Is(existingErr, store.ErrNotFound) {
		writeError(w, http.StatusInternalServerError, "round_agent_lock_check_failed", existingErr.Error())
		return
	}
	if round.Status == "active" && req.Confirm != "replace_active_round_lock" {
		details := map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug, "new_agent_id": agent.ID}
		if existingErr == nil {
			details["old_agent_id"] = existing.AgentID
		}
		writeErrorDetails(w, http.StatusConflict, "active_round_lock_confirm_required", "changing round agent locks during an active round requires confirm=replace_active_round_lock", details)
		return
	}
	metadata := req.MetadataJSON
	if len(req.Metadata) > 0 {
		metadata = string(req.Metadata)
	}
	locked, err := s.Store.LockRoundAgent(r.Context(), store.RoundAgentInput{
		RoundID:      roundID,
		AgentID:      agentID,
		CommitSHA:    req.CommitSHA,
		DockerImage:  req.DockerImage,
		MetadataJSON: metadata,
		LockedBy:     req.LockedBy,
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "round_agent_lock_failed", err.Error())
		return
	}
	roundIDPtr := locked.RoundID
	teamID := locked.TeamID
	actionMetadata := map[string]interface{}{"new_agent_id": locked.AgentID, "agent_id": locked.AgentID, "agent_slug": locked.AgentSlug, "commit_sha": locked.CommitSHA, "docker_image": locked.DockerImage}
	if existingErr == nil {
		actionMetadata["old_agent_id"] = existing.AgentID
	}
	s.recordAdminAction(r.Context(), locked.RoundSlug, "lock_round_agent", &roundIDPtr, &teamID, actionMetadata)
	writeJSON(w, http.StatusOK, locked)
}

func (s *Server) unlockRoundAgent(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	agentID, err := parseParamID(r, "agent_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_agent_id", "agent_id must be a positive integer")
		return
	}
	round, err := s.Store.GetRound(r.Context(), roundID)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	agent, err := s.Store.GetAgent(r.Context(), agentID)
	if err != nil {
		writeError(w, http.StatusNotFound, "agent_not_found", "agent not found")
		return
	}
	if round.Status == "completed" {
		writeErrorDetails(w, http.StatusConflict, "round_agent_lock_immutable", "round agent locks cannot be changed after round completion", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug})
		return
	}
	if round.Status == "active" && r.URL.Query().Get("confirm") != "replace_active_round_lock" {
		writeErrorDetails(w, http.StatusConflict, "active_round_lock_confirm_required", "changing round agent locks during an active round requires confirm=replace_active_round_lock", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug, "agent_id": agent.ID})
		return
	}
	deleted, err := s.Store.DeleteRoundAgentLock(r.Context(), round.ID, agent.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "round_agent_unlock_failed", err.Error())
		return
	}
	roundIDPtr := round.ID
	teamID := agent.TeamID
	s.recordAdminAction(r.Context(), round.Slug, "unlock_round_agent", &roundIDPtr, &teamID, map[string]interface{}{"agent_id": agent.ID, "agent_slug": agent.Slug, "deleted": deleted})
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted})
}

func (s *Server) listRoundAgents(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	agents, err := s.Store.ListRoundAgents(r.Context(), roundID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "round_agents_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, agents)
}
