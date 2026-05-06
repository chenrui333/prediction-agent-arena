package httpapi

import (
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type meResponse struct {
	Team           store.Team   `json:"team"`
	Agent          *store.Agent `json:"agent"`
	ActiveRound    *store.Round `json:"active_round"`
	LegacyTeamAuth bool         `json:"legacy_team_auth"`
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	var agent *store.Agent
	if currentAgent, ok := agentFromContext(r.Context()); ok {
		agent = &currentAgent
	}
	var active *store.Round
	if round, err := s.Store.GetActiveRound(r.Context()); err == nil {
		active = &round
	}
	writeJSON(w, http.StatusOK, meResponse{
		Team:           team,
		Agent:          agent,
		ActiveRound:    active,
		LegacyTeamAuth: legacyTeamAuthFromContext(r.Context()),
	})
}
