package httpapi

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type adminSummaryResponse struct {
	ActiveRound *store.Round           `json:"active_round"`
	LatestRound *store.Round           `json:"latest_round"`
	Teams       []store.AdminTeamStats `json:"teams"`
	Policy      interface{}            `json:"risk_policy"`
}

func (s *Server) adminSummary(w http.ResponseWriter, r *http.Request) {
	round, err := s.roundForAdminSummary(r)
	if err != nil {
		writeJSON(w, http.StatusOK, adminSummaryResponse{Teams: []store.AdminTeamStats{}, Policy: s.Policy})
		return
	}
	teams, err := s.Store.ListAdminTeamStats(r.Context(), round.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "admin_summary_failed", err.Error())
		return
	}
	activeRound, activeErr := s.Store.GetActiveRound(r.Context())
	var active *store.Round
	if activeErr == nil {
		active = &activeRound
	}
	writeJSON(w, http.StatusOK, adminSummaryResponse{ActiveRound: active, LatestRound: &round, Teams: teams, Policy: s.Policy})
}

func (s *Server) compactSnapshots(w http.ResponseWriter, r *http.Request) {
	var input struct {
		RoundID   int64  `json:"round_id"`
		KeepEvery string `json:"keep_every"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	if input.RoundID <= 0 {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	keepEvery := 5 * time.Minute
	if input.KeepEvery != "" {
		parsed, err := time.ParseDuration(input.KeepEvery)
		if err != nil || parsed <= 0 {
			writeError(w, http.StatusBadRequest, "invalid_keep_every", "keep_every must be a positive Go duration such as 5m")
			return
		}
		keepEvery = parsed
	}
	result, err := s.Store.CompactSnapshots(r.Context(), input.RoundID, keepEvery)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compact_snapshots_failed", err.Error())
		return
	}
	round, _ := s.Store.GetRound(r.Context(), input.RoundID)
	s.recordAdminAction(r.Context(), round.Slug, "compact_snapshots", &input.RoundID, nil, map[string]interface{}{"keep_every": keepEvery.String(), "portfolio_deleted": result.PortfolioSnapshotsDeleted, "score_deleted": result.ScoreSnapshotsDeleted})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) compactAudit(w http.ResponseWriter, r *http.Request) {
	var input struct {
		OlderThan string `json:"older_than"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	if input.OlderThan == "" {
		input.OlderThan = "14d"
	}
	retention, err := parseRetentionDuration(input.OlderThan)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_older_than", "older_than must be a positive duration such as 14d or 336h")
		return
	}
	cutoff := time.Now().UTC().Add(-retention).Format(time.RFC3339Nano)
	deleted, err := s.Store.DeleteAPIRequestsBefore(r.Context(), cutoff)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "compact_audit_failed", err.Error())
		return
	}
	s.recordAdminAction(r.Context(), "admin", "compact_audit", nil, nil, map[string]interface{}{"older_than": input.OlderThan, "cutoff": cutoff, "deleted": deleted})
	writeJSON(w, http.StatusOK, map[string]interface{}{"deleted": deleted, "cutoff": cutoff, "older_than": input.OlderThan})
}

func parseRetentionDuration(value string) (time.Duration, error) {
	value = strings.TrimSpace(value)
	if strings.HasSuffix(value, "d") {
		days, err := strconv.Atoi(strings.TrimSuffix(value, "d"))
		if err != nil {
			return 0, err
		}
		if days <= 0 {
			return 0, fmt.Errorf("duration must be positive")
		}
		return time.Duration(days) * 24 * time.Hour, nil
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("duration must be positive")
	}
	return parsed, nil
}

func (s *Server) roundForAdminSummary(r *http.Request) (store.Round, error) {
	if idValue := r.URL.Query().Get("round_id"); idValue != "" {
		id, err := strconv.ParseInt(idValue, 10, 64)
		if err != nil {
			return store.Round{}, err
		}
		return s.Store.GetRound(r.Context(), id)
	}
	if active, err := s.Store.GetActiveRound(r.Context()); err == nil {
		return active, nil
	}
	return s.Store.GetLatestRound(r.Context())
}

func (s *Server) exportRound(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	if err := s.Store.RefreshRoundScores(r.Context(), roundID); err != nil {
		writeError(w, http.StatusInternalServerError, "score_refresh_failed", err.Error())
		return
	}
	result, err := s.Store.ExportRound(r.Context(), roundID, s.ExportDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "export_failed", err.Error())
		return
	}
	round, _ := s.Store.GetRound(r.Context(), roundID)
	s.recordAdminAction(r.Context(), round.Slug, "export_round", &roundID, nil, map[string]interface{}{"artifacts": result.Artifacts})
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) freezeLeaderboard(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	if err := s.Store.RefreshRoundScores(r.Context(), roundID); err != nil {
		writeError(w, http.StatusInternalServerError, "score_refresh_failed", err.Error())
		return
	}
	result, err := s.Store.ExportRound(r.Context(), roundID, s.ExportDir)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "freeze_failed", err.Error())
		return
	}
	round, _ := s.Store.GetRound(r.Context(), roundID)
	s.recordAdminAction(r.Context(), round.Slug, "freeze_leaderboard", &roundID, nil, map[string]interface{}{"artifacts": result.Artifacts})
	writeJSON(w, http.StatusOK, result)
}
