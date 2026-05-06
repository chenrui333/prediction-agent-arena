package httpapi

import (
	"errors"
	"net/http"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

func (s *Server) listMarkets(w http.ResponseWriter, r *http.Request) {
	round, err := s.Store.GetActiveRound(r.Context())
	if err != nil {
		markets, listErr := s.Store.ListMarkets(r.Context())
		if listErr != nil {
			writeError(w, http.StatusInternalServerError, "markets_failed", listErr.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"round": nil, "markets": markets})
		return
	}
	markets, err := s.Store.ListRoundMarkets(r.Context(), round.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "markets_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"round": round, "markets": markets})
}

func (s *Server) getMarket(w http.ResponseWriter, r *http.Request) {
	id, err := parseParamID(r, "market_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_market_id", "market_id must be a positive integer")
		return
	}
	market, err := s.Store.GetMarket(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusNotFound, "market_not_found", "market not found")
		return
	}
	writeJSON(w, http.StatusOK, market)
}

func (s *Server) adminListMarkets(w http.ResponseWriter, r *http.Request) {
	markets, err := s.Store.ListMarkets(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "markets_failed", err.Error())
		return
	}
	writeJSON(w, http.StatusOK, markets)
}

func (s *Server) upsertMarket(w http.ResponseWriter, r *http.Request) {
	var input store.MarketInput
	if err := decodeJSON(r, &input); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	market, err := s.Store.UpsertMarket(r.Context(), input)
	if err != nil {
		writeError(w, http.StatusBadRequest, "market_failed", err.Error())
		return
	}
	_ = s.Events.Append(r.Context(), "admin", "admin", "admin_action", map[string]interface{}{"action": "upsert_market", "market_id": market.ID, "market_slug": market.Slug})
	writeJSON(w, http.StatusCreated, market)
}

func (s *Server) allowMarket(w http.ResponseWriter, r *http.Request) {
	roundID, err := parseParamID(r, "round_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_round_id", "round_id must be a positive integer")
		return
	}
	marketID, err := parseParamID(r, "market_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_market_id", "market_id must be a positive integer")
		return
	}
	if err := s.Store.AddRoundMarket(r.Context(), roundID, marketID); err != nil {
		writeError(w, http.StatusBadRequest, "allow_market_failed", err.Error())
		return
	}
	roundSlug := "admin"
	if round, err := s.Store.GetRound(r.Context(), roundID); err == nil {
		roundSlug = round.Slug
	}
	_ = s.Events.Append(r.Context(), roundSlug, "admin", "admin_action", map[string]interface{}{"action": "allow_market", "round_id": roundID, "market_id": marketID})
	writeJSON(w, http.StatusOK, map[string]string{"status": "allowed"})
}

func (s *Server) resolveMarket(w http.ResponseWriter, r *http.Request) {
	marketID, err := parseParamID(r, "market_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_market_id", "market_id must be a positive integer")
		return
	}
	var input struct {
		Outcome    string `json:"outcome"`
		ResolvedBy string `json:"resolved_by"`
	}
	if err := decodeJSON(r, &input); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	result, err := s.resolveMarketOutcome(r, marketID, input.Outcome, input.ResolvedBy)
	if err != nil {
		writeError(w, http.StatusBadRequest, "resolve_market_failed", err.Error())
		return
	}
	if round, roundErr := s.Store.GetActiveRound(r.Context()); roundErr == nil {
		s.invalidateLeaderboard(r.Context(), round.ID)
		_ = s.Events.Append(r.Context(), round.Slug, "admin", "admin_action", map[string]interface{}{"action": "resolve_market", "market_id": marketID, "outcome": input.Outcome})
	} else {
		_ = s.Events.Append(r.Context(), "admin", "admin", "admin_action", map[string]interface{}{"action": "resolve_market", "market_id": marketID, "outcome": input.Outcome})
	}
	writeJSON(w, http.StatusOK, result)
}

func (s *Server) resolveMarketOutcome(r *http.Request, marketID int64, outcome, resolvedBy string) (interface{}, error) {
	state, err := s.Store.ResolveSimulatedMarket(r.Context(), marketID, outcome, resolvedBy)
	if err == nil {
		return state, nil
	}
	if errors.Is(err, store.ErrNotFound) {
		return s.Store.SetMarketOutcome(r.Context(), marketID, outcome, resolvedBy)
	}
	return nil, err
}
