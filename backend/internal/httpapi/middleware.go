package httpapi

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type contextKey string

const teamContextKey contextKey = "team"

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
		origin := s.CORSOrigin
		if origin == "" {
			origin = "*"
		}
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (s *Server) studentAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing_token", "Authorization: Bearer token is required")
			return
		}
		team, err := s.Store.FindTeamByTokenHash(r.Context(), auth.HashToken(token))
		if err != nil {
			writeError(w, http.StatusUnauthorized, "invalid_token", "team token is invalid")
			return
		}
		if !team.IsActive {
			writeErrorDetails(w, http.StatusForbidden, "inactive_team", "team is paused by the instructor", map[string]interface{}{"team_id": team.ID, "team_slug": team.Slug})
			return
		}
		ctx := context.WithValue(r.Context(), teamContextKey, team)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) adminAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r)
		if token == "" || subtle.ConstantTimeCompare([]byte(token), []byte(s.AdminToken)) != 1 {
			writeError(w, http.StatusUnauthorized, "admin_auth_required", "valid admin bearer token is required")
			return
		}
		next.ServeHTTP(w, r)
	})
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
