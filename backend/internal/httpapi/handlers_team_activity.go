package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type teamActivityResponse struct {
	store.TeamActivity
	Visibility         string `json:"visibility"`
	DetailRedacted     bool   `json:"detail_redacted"`
	TradeCount         int64  `json:"trade_count"`
	RiskRejectionCount int64  `json:"risk_rejection_count"`
	LastHeartbeat      string `json:"last_heartbeat,omitempty"`
}

func (s *Server) getTeamActivity(w http.ResponseWriter, r *http.Request) {
	teamSlug := chi.URLParam(r, "team_slug")
	round, err := s.roundFromQuery(r)
	if err != nil {
		writeError(w, http.StatusNotFound, "round_not_found", "round not found")
		return
	}
	limit, ok := parseActivityLimit(w, r)
	if !ok {
		return
	}
	visibility, redacted, full := s.publicTeamActivityVisibility(round)
	if !full {
		cacheKey := fmt.Sprintf("team_activity_summary:round:%d:team:%s:visibility:%s", round.ID, teamSlug, visibility)
		var cached teamActivityResponse
		if ok, err := s.Cache.GetJSON(r.Context(), cacheKey, &cached); err == nil && ok {
			writeJSON(w, http.StatusOK, cached)
			return
		}
		summary, err := s.Store.TeamActivitySummary(r.Context(), teamSlug, round.ID)
		if err != nil {
			writeErrorDetails(w, http.StatusNotFound, "team_not_found", "team or round not found", map[string]interface{}{"team_slug": teamSlug, "round_id": round.ID})
			return
		}
		response := responseFromTeamActivitySummary(summary, visibility, redacted)
		_ = s.Cache.SetJSON(r.Context(), cacheKey, response, 5*time.Second)
		writeJSON(w, http.StatusOK, response)
		return
	}
	activity, err := s.Store.TeamActivity(r.Context(), teamSlug, round.ID, limit)
	if err != nil {
		writeErrorDetails(w, http.StatusNotFound, "team_not_found", "team or round not found", map[string]interface{}{"team_slug": teamSlug, "round_id": round.ID})
		return
	}
	response := teamActivityResponse{
		TeamActivity:       activity,
		Visibility:         visibility,
		DetailRedacted:     false,
		TradeCount:         int64(len(activity.Fills)),
		RiskRejectionCount: int64(len(activity.RiskEvents)),
	}
	writeJSON(w, http.StatusOK, response)
}

func (s *Server) getAdminTeamActivity(w http.ResponseWriter, r *http.Request) {
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
	team, err := s.Store.GetTeam(r.Context(), teamID)
	if err != nil {
		writeError(w, http.StatusNotFound, "team_not_found", "team not found")
		return
	}
	limit, ok := parseActivityLimit(w, r)
	if !ok {
		return
	}
	activity, err := s.Store.TeamActivity(r.Context(), team.Slug, roundID, limit)
	if err != nil {
		writeErrorDetails(w, http.StatusNotFound, "team_activity_not_found", "team or round activity not found", map[string]interface{}{"team_id": teamID, "round_id": roundID})
		return
	}
	writeJSON(w, http.StatusOK, teamActivityResponse{TeamActivity: activity, Visibility: "full"})
}

func parseActivityLimit(w http.ResponseWriter, r *http.Request) (int, bool) {
	limit := 25
	if value := r.URL.Query().Get("limit"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_limit", "limit must be a positive integer")
			return 0, false
		}
		limit = parsed
	}
	return limit, true
}

func (s *Server) publicTeamActivityVisibility(round store.Round) (visibility string, redacted bool, full bool) {
	policy := s.PublicTeamActivity
	switch policy {
	case "redacted", "full":
	default:
		policy = "summary"
	}
	if policy == "full" && round.Status == "completed" {
		return "full", false, true
	}
	if policy == "redacted" {
		return "redacted", true, false
	}
	return "summary", true, false
}

func responseFromTeamActivitySummary(summary store.TeamActivitySummary, visibility string, redacted bool) teamActivityResponse {
	return teamActivityResponse{
		TeamActivity: store.TeamActivity{
			Team:       summary.Team,
			Round:      summary.Round,
			Portfolio:  summary.Portfolio,
			Decisions:  []store.Decision{},
			Orders:     []store.Order{},
			Fills:      []store.Fill{},
			RiskEvents: []store.RiskEvent{},
		},
		Visibility:         visibility,
		DetailRedacted:     redacted,
		TradeCount:         summary.TradeCount,
		RiskRejectionCount: summary.RiskRejectionCount,
		LastHeartbeat:      summary.LastHeartbeat,
	}
}
