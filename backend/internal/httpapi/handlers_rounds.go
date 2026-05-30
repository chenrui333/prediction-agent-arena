package httpapi

import (
	"fmt"
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

func (s *Server) createRound(w http.ResponseWriter, r *http.Request) {
	var input store.RoundInput
	if err := decodeJSON(w, r, &input); err != nil {
		writeDecodeError(w, err)
		return
	}
	if input.InitialBalanceCents == 0 {
		input.InitialBalanceCents = s.Policy.InitialBalanceCents
	}
	round, err := s.Store.CreateRound(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "round_failed", err.Error())
		return
	}
	roundID := round.ID
	s.recordAdminAction(r.Context(), round.Slug, "create_round", &roundID, nil, nil)
	writeJSON(w, http.StatusCreated, round)
}

func (s *Server) listRounds(w http.ResponseWriter, r *http.Request) {
	rounds, err := s.Store.ListRounds(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "rounds_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, rounds)
}

func (s *Server) activateRound(w http.ResponseWriter, r *http.Request) {
	s.setRoundStatus(w, r, "active")
}

func (s *Server) pauseRound(w http.ResponseWriter, r *http.Request) {
	s.setRoundStatus(w, r, "paused")
}

func (s *Server) completeRound(w http.ResponseWriter, r *http.Request) {
	s.setRoundStatus(w, r, "completed")
}

func (s *Server) requireLockedAgentsRound(w http.ResponseWriter, r *http.Request) {
	s.setRoundRequireLockedAgents(w, r, true, "require_locked_agents")
}

func (s *Server) allowUnlockedAgentsRound(w http.ResponseWriter, r *http.Request) {
	s.setRoundRequireLockedAgents(w, r, false, "allow_unlocked_agents")
}

func (s *Server) setRoundStatus(w http.ResponseWriter, r *http.Request, status string) {
	id, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	if status == "active" {
		round, err := s.Store.GetRound(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusNotFound, "round_not_found", "round not found")
			return
		}
		totalEnrolled, err := s.Store.CountRoundTeams(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "round_enrollment_check_failed", err.Error())
			return
		}
		activeEnrolled, err := s.Store.CountActiveRoundTeams(r.Context(), id)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "round_enrollment_check_failed", err.Error())
			return
		}
		if totalEnrolled == 0 || activeEnrolled == 0 {
			writeErrorDetails(w, http.StatusConflict, "round_enrollment_empty", "round must have at least one active enrolled team before activation", map[string]interface{}{
				"round_id":            round.ID,
				"round_slug":          round.Slug,
				"enrolled_team_count": totalEnrolled,
				"active_team_count":   activeEnrolled,
			})
			return
		}
		if roundRequiresLockedAgent(round) {
			preflight, err := s.Store.CheckRoundAgentLocks(r.Context(), id)
			if err != nil {
				writeError(w, http.StatusInternalServerError, "round_agent_lock_check_failed", err.Error())
				return
			}
			if !preflight.OK() {
				issueCount := len(preflight.MissingTeams) + len(preflight.InvalidTeams)
				message := fmt.Sprintf("%d active teams do not have valid locked agents", issueCount)
				if issueCount == 1 {
					message = "1 active team does not have a valid locked agent"
				}
				writeErrorDetails(w, http.StatusConflict, "round_agent_locks_incomplete", message, map[string]interface{}{
					"round_id":      round.ID,
					"round_slug":    round.Slug,
					"missing_teams": preflight.MissingTeams,
					"invalid_teams": preflight.InvalidTeams,
				})
				return
			}
		}
	}
	round, err := s.Store.SetRoundStatus(r.Context(), id, status)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	s.invalidateLeaderboard(r.Context(), id)
	s.recordAdminAction(r.Context(), round.Slug, "round_"+status, &id, nil, nil)
	writeJSON(w, http.StatusOK, round)
}

func (s *Server) setRoundRequireLockedAgents(w http.ResponseWriter, r *http.Request, required bool, action string) {
	id, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	round, err := s.Store.SetRoundRequireLockedAgents(r.Context(), id, required)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	s.recordAdminAction(r.Context(), round.Slug, action, &id, nil, map[string]interface{}{"require_locked_agents": required})
	writeJSON(w, http.StatusOK, round)
}

func (s *Server) resetRound(w http.ResponseWriter, r *http.Request) {
	id, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	round, err := s.Store.GetRound(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	if err := s.Store.ResetRound(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "reset_round_failed", err.Error())
		return
	}
	s.invalidateLeaderboard(r.Context(), id)
	s.recordAdminAction(r.Context(), round.Slug, "reset_round", &id, nil, nil)
	reset, _ := s.Store.GetRound(r.Context(), id)
	writeJSON(w, http.StatusOK, reset)
}

func (s *Server) settleRound(w http.ResponseWriter, r *http.Request) {
	id, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	round, err := s.Store.GetRound(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	var input struct {
		SettledBy               string `json:"settled_by"`
		Confirm                 string `json:"confirm"`
		CompleteAfterSettlement bool   `json:"complete_after_settlement"`
	}
	if r.Body != nil && r.ContentLength != 0 {
		if err := decodeJSON(w, r, &input); err != nil {
			writeDecodeError(w, err)
			return
		}
	}
	if round.Status == "active" && input.Confirm != "settle_active_round" {
		writeErrorDetails(w, http.StatusConflict, "settle_active_round_confirm_required", "settling an active round requires confirm=settle_active_round", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug})
		return
	}
	unresolved, err := s.Store.ListUnresolvedRoundMarkets(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settlement_preflight_failed", err.Error())
		return
	}
	if len(unresolved) > 0 {
		writeErrorDetails(w, http.StatusConflict, "round_markets_unresolved", "all round markets must be resolved before settlement", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug, "unresolved_markets": unresolved})
		return
	}
	settlements, err := s.Store.SettleRound(r.Context(), id, input.SettledBy)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "settlement_failed", err.Error())
		return
	}
	if err := s.Store.RefreshRoundScores(r.Context(), id); err != nil {
		writeError(w, http.StatusInternalServerError, "score_refresh_failed", err.Error())
		return
	}
	s.invalidateLeaderboard(r.Context(), id)
	for _, settlement := range settlements {
		team, err := s.Store.GetTeam(r.Context(), settlement.TeamID)
		if err != nil {
			continue
		}
		_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "settlement", settlement)
	}
	if input.CompleteAfterSettlement {
		completed, err := s.Store.SetRoundStatus(r.Context(), id, "completed")
		if err != nil {
			writeError(w, http.StatusInternalServerError, "complete_round_failed", err.Error())
			return
		}
		round = completed
	}
	s.recordAdminAction(r.Context(), round.Slug, "settle_round", &id, nil, map[string]interface{}{"settlement_count": len(settlements), "complete_after_settlement": input.CompleteAfterSettlement})
	writeJSON(w, http.StatusOK, map[string]interface{}{"round": round, "settlements": settlements})
}
