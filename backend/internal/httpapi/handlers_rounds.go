package httpapi

import (
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

func (s *Server) createRound(w http.ResponseWriter, r *http.Request) {
	var input store.RoundInput
	if err := decodeJSON(r, &input); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
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
	_ = s.Events.Append(r.Context(), round.Slug, "admin", "admin_action", map[string]interface{}{"action": "create_round", "round_id": round.ID})
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

func (s *Server) setRoundStatus(w http.ResponseWriter, r *http.Request, status string) {
	id, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	round, err := s.Store.SetRoundStatus(r.Context(), id, status)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	s.invalidateLeaderboard(r.Context(), id)
	_ = s.Events.Append(r.Context(), round.Slug, "admin", "admin_action", map[string]interface{}{"action": "round_" + status, "round_id": id})
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
	_ = s.Events.Append(r.Context(), round.Slug, "admin", "admin_action", map[string]interface{}{"action": "reset_round", "round_id": id})
	reset, _ := s.Store.GetRound(r.Context(), id)
	writeJSON(w, http.StatusOK, reset)
}
