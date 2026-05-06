package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type leaderboardResponse struct {
	Round store.Round            `json:"round"`
	Rows  []store.LeaderboardRow `json:"rows"`
}

func (s *Server) leaderboard(w http.ResponseWriter, r *http.Request) {
	round, err := s.roundFromQuery(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	key := leaderboardKey(round.ID)
	var cached leaderboardResponse
	if ok, err := s.Cache.GetJSON(r.Context(), key, &cached); err == nil && ok {
		writeJSON(w, http.StatusOK, cached)
		return
	}
	if err := s.Store.RefreshRoundScores(r.Context(), round.ID); err != nil {
		writeError(w, http.StatusInternalServerError, "score_refresh_failed", err.Error())
		return
	}
	rows, err := s.Store.ListLeaderboard(r.Context(), round.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "leaderboard_failed", err.Error())
		return
	}
	response := leaderboardResponse{Round: round, Rows: rows}
	_ = s.Cache.SetJSON(r.Context(), key, response, s.LeaderboardTTL)
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) roundFromQuery(r *http.Request) (store.Round, error) {
	if idValue := r.URL.Query().Get("round_id"); idValue != "" {
		id, err := strconv.ParseInt(idValue, 10, 64)
		if err != nil {
			return store.Round{}, err
		}
		return s.Store.GetRound(r.Context(), id)
	}
	if slug := r.URL.Query().Get("round_slug"); slug != "" {
		return s.Store.GetRoundBySlug(r.Context(), slug)
	}
	return s.Store.GetActiveRound(r.Context())
}

func (s *Server) invalidateLeaderboard(ctx context.Context, roundID int64) {
	s.Cache.Delete(ctx, leaderboardKey(roundID))
}

func leaderboardKey(roundID int64) string {
	return fmt.Sprintf("leaderboard:round:%d", roundID)
}
