package httpapi

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type contextKey string

const teamContextKey contextKey = "team"
const agentContextKey contextKey = "agent"
const legacyTeamAuthContextKey contextKey = "legacy_team_auth"

type apiError struct {
	Error apiErrorBody `json:"error"`
}

type apiErrorBody struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}

func (s *Server) recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				if s.Logger != nil {
					s.Logger.Error("panic in http handler", "panic", rec)
				}
				writeError(w, http.StatusInternalServerError, "internal_error", "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func (s *Server) cors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && s.originAllowed(origin) {
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Vary", "Origin")
			w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		}
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) originAllowed(origin string) bool {
	for _, allowed := range s.CORSOrigins {
		if allowed == "*" || allowed == origin {
			return true
		}
	}
	return false
}

func (s *Server) agentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			if !s.limitAuthFailure(w, r) {
				return
			}
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization: Bearer token is required")
			return
		}
		tokenHash := auth.HashToken(token)
		agent, team, err := s.Store.FindAgentByTokenHash(r.Context(), tokenHash)
		if err == nil {
			if !team.IsActive {
				writeErrorDetails(w, http.StatusForbidden, "inactive_team", "team is paused by the operator", map[string]interface{}{"team_id": team.ID, "team_slug": team.Slug})
				return
			}
			if agent.Status == "revoked" {
				writeErrorDetails(w, http.StatusForbidden, "revoked_agent", "agent token has been revoked", map[string]interface{}{"agent_id": agent.ID, "agent_slug": agent.Slug})
				return
			}
			ctx := context.WithValue(r.Context(), teamContextKey, team)
			ctx = context.WithValue(ctx, agentContextKey, agent)
			next.ServeHTTP(w, r.WithContext(ctx))
			return
		}
		if s.LegacyTeamAuth {
			team, err := s.Store.FindTeamByTokenHash(r.Context(), tokenHash)
			if err == nil {
				if !team.IsActive {
					writeErrorDetails(w, http.StatusForbidden, "inactive_team", "team is paused by the operator", map[string]interface{}{"team_id": team.ID, "team_slug": team.Slug})
					return
				}
				ctx := context.WithValue(r.Context(), teamContextKey, team)
				ctx = context.WithValue(ctx, legacyTeamAuthContextKey, true)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}
		}
		if !s.limitAuthFailure(w, r) {
			return
		}
		writeError(w, http.StatusUnauthorized, "invalid_token", "agent token is invalid")
	})
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" || s.AdminToken == "" || !constantTimeStringEqual(token, s.AdminToken) {
			if !s.limitAuthFailure(w, r) {
				return
			}
			writeError(w, http.StatusUnauthorized, "admin_auth_required", "valid admin bearer token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func constantTimeStringEqual(a, b string) bool {
	aSum := sha256.Sum256([]byte(a))
	bSum := sha256.Sum256([]byte(b))
	return subtle.ConstantTimeCompare(aSum[:], bSum[:]) == 1
}

func bearerToken(r *http.Request) string {
	header := strings.TrimSpace(r.Header.Get("Authorization"))
	if header == "" {
		return ""
	}
	parts := strings.SplitN(header, " ", 2)
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return ""
	}
	return strings.TrimSpace(parts[1])
}

func teamFromContext(ctx context.Context) (store.Team, bool) {
	team, ok := ctx.Value(teamContextKey).(store.Team)
	return team, ok
}

func agentFromContext(ctx context.Context) (store.Agent, bool) {
	agent, ok := ctx.Value(agentContextKey).(store.Agent)
	return agent, ok
}

func agentIDFromContext(ctx context.Context) *int64 {
	agent, ok := agentFromContext(ctx)
	if !ok {
		return nil
	}
	return &agent.ID
}

func legacyTeamAuthFromContext(ctx context.Context) bool {
	return ctx.Value(legacyTeamAuthContextKey) == true
}

func (s *Server) requireActiveAgentMutation(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		agent, ok := agentFromContext(r.Context())
		if !ok {
			if r.Context().Value(legacyTeamAuthContextKey) == true {
				next.ServeHTTP(w, r)
				return
			}
			writeError(w, http.StatusUnauthorized, "missing_agent", "agent context missing")
			return
		}
		if agent.Status == "paused" {
			writeErrorDetails(w, http.StatusForbidden, "paused_agent", "agent is paused by the operator", map[string]interface{}{"agent_id": agent.ID, "agent_slug": agent.Slug})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireActiveRoundEnrollment(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		team, ok := teamFromContext(r.Context())
		if !ok {
			writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
			return
		}
		round, err := s.Store.GetActiveRound(r.Context())
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		enrollment, err := s.Store.GetRoundTeam(r.Context(), round.ID, team.ID)
		if err != nil {
			if !errors.Is(err, store.ErrNotFound) {
				writeError(w, http.StatusInternalServerError, "round_enrollment_check_failed", err.Error())
				return
			}
			writeErrorDetails(w, http.StatusForbidden, "team_not_enrolled", "team is not enrolled in the active round", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug, "team_id": team.ID, "team_slug": team.Slug})
			return
		}
		if enrollment.Status != "active" {
			writeErrorDetails(w, http.StatusForbidden, "round_team_not_active", "team enrollment is not active for this round", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug, "team_id": team.ID, "team_slug": team.Slug, "enrollment_status": enrollment.Status})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) requireRoundAgentIfLocked(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		round, err := s.Store.GetActiveRound(r.Context())
		if err != nil {
			next.ServeHTTP(w, r)
			return
		}
		if !roundRequiresLockedAgent(round) {
			next.ServeHTTP(w, r)
			return
		}
		agent, ok := agentFromContext(r.Context())
		if !ok {
			writeErrorDetails(w, http.StatusForbidden, "round_agent_lock_required", "this round requires a registered locked agent", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug})
			return
		}
		locked, err := s.Store.RoundAgentLocked(r.Context(), round.ID, agent.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "round_agent_lock_check_failed", err.Error())
			return
		}
		if !locked {
			writeErrorDetails(w, http.StatusForbidden, "agent_not_locked_for_round", "agent is not locked for this round", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug, "agent_id": agent.ID, "agent_slug": agent.Slug})
			return
		}
		next.ServeHTTP(w, r)
	})
}

func roundRequiresLockedAgent(round store.Round) bool {
	return round.Mode == "replay" || round.RequireLockedAgents
}

func writeJSON(w http.ResponseWriter, status int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	writeErrorDetails(w, status, code, message, nil)
}

func writeErrorDetails(w http.ResponseWriter, status int, code, message string, details map[string]interface{}) {
	writeJSON(w, status, apiError{Error: apiErrorBody{Code: code, Message: message, Details: details}})
}

func decodeJSON(r *http.Request, dest interface{}) error {
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(dest); err != nil {
		return fmt.Errorf("request body must be valid JSON: %w", err)
	}
	return nil
}

func parseParamID(r *http.Request, name string) (int64, error) {
	value := chi.URLParam(r, name)
	id, err := strconv.ParseInt(value, 10, 64)
	if err != nil || id <= 0 {
		return 0, errors.New("invalid id")
	}
	return id, nil
}

func (s *Server) logWarn(message string, args ...interface{}) {
	if s.Logger != nil {
		s.Logger.Warn(message, args...)
		return
	}
	slog.Warn(message, args...)
}

func (s *Server) rateLimitPublicRead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRate(w, r, "public_read:"+s.remoteHash(r), s.RateLimits.PublicReadPerMinute) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitAgentRead(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRate(w, r, "agent_read:"+s.agentRateKey(r), s.RateLimits.AgentReadPerMinute) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitHeartbeat(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRate(w, r, "heartbeat:"+s.agentRateKey(r), s.RateLimits.AgentHeartbeatPerMinute) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitDecision(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRate(w, r, "decision:"+s.agentRateKey(r), s.RateLimits.AgentDecisionPerMinute) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitOrder(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRate(w, r, "order:"+s.agentRateKey(r), s.RateLimits.AgentOrderPerMinute) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) rateLimitAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.allowRate(w, r, "admin:"+s.remoteHash(r), s.RateLimits.AdminPerMinute) {
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) limitAuthFailure(w http.ResponseWriter, r *http.Request) bool {
	return s.allowRate(w, r, "auth_failure:"+s.remoteHash(r), s.RateLimits.AuthFailurePerMinute)
}

func (s *Server) allowRate(w http.ResponseWriter, r *http.Request, key string, limit int) bool {
	if !s.RateLimits.Enabled || limit <= 0 {
		return true
	}
	allowed, err := s.Cache.Allow(r.Context(), "rate:"+key, limit, time.Minute)
	if err != nil {
		s.logWarn("redis rate limiter unavailable", "key", key, "error", err)
		if s.RateLimits.FailClosed {
			writeError(w, http.StatusTooManyRequests, "rate_limiter_unavailable", "rate limiter is unavailable")
			return false
		}
		return true
	}
	if !allowed {
		writeErrorDetails(w, http.StatusTooManyRequests, "rate_limit_exceeded", "rate limit exceeded", map[string]interface{}{"limit_per_minute": limit})
		return false
	}
	return true
}

func (s *Server) agentRateKey(r *http.Request) string {
	if agent, ok := agentFromContext(r.Context()); ok {
		return "agent:" + strconv.FormatInt(agent.ID, 10)
	}
	if team, ok := teamFromContext(r.Context()); ok {
		return "legacy_team:" + strconv.FormatInt(team.ID, 10)
	}
	return "unknown:" + s.remoteHash(r)
}

func (s *Server) remoteHash(r *http.Request) string {
	return hashAuditValue(s.AuditSalt, s.clientIP(r))
}

func (s *Server) clientIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	if host == "" {
		return "unknown"
	}
	if !s.TrustProxyHeaders || !s.remoteAddrTrusted(host) {
		return host
	}
	forwardedFor := r.Header.Get("X-Forwarded-For")
	if forwardedFor == "" {
		return host
	}
	parts := strings.Split(forwardedFor, ",")
	client := strings.TrimSpace(parts[0])
	if client == "" {
		return host
	}
	parsed := net.ParseIP(client)
	if parsed == nil {
		return host
	}
	return parsed.String()
}

func (s *Server) remoteAddrTrusted(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}
	for _, value := range s.TrustedProxyCIDRs {
		_, network, err := net.ParseCIDR(strings.TrimSpace(value))
		if err != nil {
			continue
		}
		if network.Contains(ip) {
			return true
		}
	}
	return false
}

func hashAuditValue(salt, value string) string {
	sum := sha256.Sum256([]byte(salt + ":" + value))
	return fmt.Sprintf("%x", sum[:])
}

type captureResponseWriter struct {
	http.ResponseWriter
	status int
}

func (w *captureResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *captureResponseWriter) Write(data []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(data)
}

func (s *Server) auditRequests(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capture := &captureResponseWriter{ResponseWriter: w}
		next.ServeHTTP(capture, r)
		if !shouldAuditRequest(r) {
			return
		}
		status := capture.status
		if status == 0 {
			status = http.StatusOK
		}
		var teamID, agentID *int64
		if team, ok := teamFromContext(r.Context()); ok {
			teamID = &team.ID
		}
		if agent, ok := agentFromContext(r.Context()); ok {
			agentID = &agent.ID
		}
		if err := s.Store.CreateAPIRequest(r.Context(), store.APIRequestInput{
			TeamID:        teamID,
			AgentID:       agentID,
			Method:        r.Method,
			Path:          r.URL.Path,
			Status:        status,
			RateLimited:   status == http.StatusTooManyRequests,
			IPHash:        hashAuditValue(s.AuditSalt, s.clientIP(r)),
			UserAgentHash: hashAuditValue(s.AuditSalt, r.UserAgent()),
		}); err != nil {
			s.logWarn("api request audit write failed", "error", err)
		}
	})
}

func shouldAuditRequest(r *http.Request) bool {
	if r.Method == http.MethodGet || r.Method == http.MethodHead || r.Method == http.MethodOptions {
		return false
	}
	path := r.URL.Path
	if strings.HasPrefix(path, "/api/v1/admin/") {
		return true
	}
	switch path {
	case "/api/v1/heartbeat", "/api/v1/decisions", "/api/v1/orders":
		return true
	default:
		return strings.HasPrefix(path, "/api/v1/orders/") && strings.HasSuffix(path, "/cancel")
	}
}
