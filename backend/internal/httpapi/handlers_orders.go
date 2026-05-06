package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/risk"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

type tradeRequest struct {
	MarketID                int64  `json:"market_id"`
	Outcome                 string `json:"outcome"`
	Action                  string `json:"action"`
	AmountCents             int64  `json:"amount_cents"`
	LimitPriceBPS           *int64 `json:"limit_price_bps,omitempty"`
	EstimatedProbabilityBPS *int64 `json:"estimated_probability_bps,omitempty"`
	Confidence              string `json:"confidence"`
	Reason                  string `json:"reason"`
	PriorDecisionID         *int64 `json:"prior_decision_id,omitempty"`
}

type orderResponse struct {
	Decision  *store.Decision          `json:"decision,omitempty"`
	Order     store.Order              `json:"order"`
	Fill      *store.Fill              `json:"fill,omitempty"`
	Portfolio *store.PortfolioSnapshot `json:"portfolio,omitempty"`
	Score     *store.ScoreSnapshot     `json:"score,omitempty"`
	RiskEvent *store.RiskEvent         `json:"risk_event,omitempty"`
	Violation *risk.Violation          `json:"violation,omitempty"`
}

func (s *Server) postHeartbeat(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	agentID := agentIDFromContext(r.Context())
	round, err := s.Store.GetActiveRound(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "no_active_round", "no active round")
		return
	}
	var req struct {
		Status   string          `json:"status"`
		Metadata json.RawMessage `json:"metadata"`
	}
	if err := decodeJSON(r, &req); err != nil {
		req.Status = "online"
		req.Metadata = json.RawMessage(`{}`)
	}
	metadata := string(req.Metadata)
	if metadata == "" {
		metadata = "{}"
	}
	hb, err := s.Store.CreateHeartbeat(r.Context(), round.ID, team.ID, agentID, req.Status, metadata)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "heartbeat_failed", err.Error())
		return
	}
	_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "heartbeat", hb)
	s.invalidateLeaderboard(r.Context(), round.ID)
	writeJSON(w, http.StatusCreated, hb)
}

func (s *Server) postDecision(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	agentID := agentIDFromContext(r.Context())
	var req tradeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	if shapeErr := s.validateTradeRequestShape(req); shapeErr != nil {
		writeErrorDetails(w, http.StatusBadRequest, shapeErr.Code, shapeErr.Message, shapeErr.Details)
		return
	}
	round, market, raw, err := s.prepareTrade(r, req)
	if err != nil {
		s.writeTradePrepError(w, err, req.MarketID)
		return
	}
	observed := observedPrice(req.Outcome, market)
	edge := int64(0)
	if req.EstimatedProbabilityBPS != nil {
		edge = *req.EstimatedProbabilityBPS - observed
	}
	var decision store.Decision
	err = s.Store.WithTx(r.Context(), func(tx *store.Tx) error {
		var err error
		decision, err = tx.CreateDecision(r.Context(), store.DecisionInput{
			RoundID:                 round.ID,
			TeamID:                  team.ID,
			AgentID:                 agentID,
			MarketID:                market.ID,
			ObservedPriceBPS:        observed,
			EstimatedProbabilityBPS: req.EstimatedProbabilityBPS,
			EdgeBPS:                 edge,
			Action:                  req.Action,
			Outcome:                 req.Outcome,
			AmountCents:             req.AmountCents,
			Confidence:              req.Confidence,
			Reason:                  req.Reason,
			RawPayloadJSON:          string(raw),
		})
		return err
	})
	if err != nil {
		writeError(w, http.StatusBadRequest, "decision_failed", err.Error())
		return
	}
	_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "decision", decision)
	writeJSON(w, http.StatusCreated, decision)
}

func (s *Server) postOrder(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	agentID := agentIDFromContext(r.Context())
	var req tradeRequest
	if err := decodeJSON(r, &req); err != nil {
		writeErrorDetails(w, http.StatusBadRequest, "invalid_json", "request body must be valid JSON", map[string]interface{}{"decode_error": err.Error()})
		return
	}
	if shapeErr := s.validateTradeRequestShape(req); shapeErr != nil {
		writeErrorDetails(w, http.StatusBadRequest, shapeErr.Code, shapeErr.Message, shapeErr.Details)
		return
	}
	round, market, raw, err := s.prepareTrade(r, req)
	if err != nil {
		s.writeTradePrepError(w, err, req.MarketID)
		return
	}
	violation, err := s.checkRisk(r, team, round, market, req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "risk_failed", err.Error())
		return
	}
	if violation != nil {
		s.rejectOrder(w, r, team, round, market, req, raw, violation)
		return
	}

	limit := int64(0)
	if req.LimitPriceBPS != nil {
		limit = *req.LimitPriceBPS
	}
	venueResult, err := s.Venue.PlaceOrder(r.Context(), venue.PlaceOrderRequest{
		TeamSlug:      team.Slug,
		RoundSlug:     round.Slug,
		ExternalID:    market.ExternalID,
		Action:        req.Action,
		Outcome:       req.Outcome,
		AmountCents:   req.AmountCents,
		LimitPriceBPS: limit,
	})
	if err != nil {
		writeErrorDetails(w, http.StatusBadGateway, "venue_unavailable", "venue unavailable", map[string]interface{}{"venue": market.Venue, "error": err.Error()})
		return
	}
	observed := observedPrice(req.Outcome, market)
	edge := int64(0)
	if req.EstimatedProbabilityBPS != nil {
		edge = *req.EstimatedProbabilityBPS - observed
	}
	status := venueResult.Status
	if status == "" {
		status = "open"
	}
	var decision store.Decision
	var order store.Order
	var fill *store.Fill
	err = s.Store.WithTx(r.Context(), func(tx *store.Tx) error {
		var err error
		decision, err = tx.CreateDecision(r.Context(), store.DecisionInput{
			RoundID:                 round.ID,
			TeamID:                  team.ID,
			AgentID:                 agentID,
			MarketID:                market.ID,
			ObservedPriceBPS:        observed,
			EstimatedProbabilityBPS: req.EstimatedProbabilityBPS,
			EdgeBPS:                 edge,
			Action:                  req.Action,
			Outcome:                 req.Outcome,
			AmountCents:             req.AmountCents,
			Confidence:              req.Confidence,
			Reason:                  req.Reason,
			RawPayloadJSON:          string(raw),
		})
		if err != nil {
			return err
		}
		order, err = tx.CreateOrder(r.Context(), store.OrderInput{
			RoundID:       round.ID,
			TeamID:        team.ID,
			AgentID:       agentID,
			MarketID:      market.ID,
			VenueOrderID:  venueResult.VenueOrderID,
			Action:        req.Action,
			Outcome:       req.Outcome,
			AmountCents:   req.AmountCents,
			LimitPriceBPS: limit,
			Status:        status,
		})
		if err != nil {
			return err
		}
		if venueResult.Filled {
			createdFill, err := tx.CreateFill(r.Context(), store.FillInput{
				RoundID:      round.ID,
				TeamID:       team.ID,
				OrderID:      order.ID,
				MarketID:     market.ID,
				Action:       req.Action,
				Outcome:      req.Outcome,
				AmountCents:  req.AmountCents,
				FillPriceBPS: venueResult.FillPriceBPS,
				FeeCents:     venueResult.FeeCents,
				SlippageBPS:  venueResult.SlippageBPS,
			})
			if err != nil {
				return err
			}
			fill = &createdFill
		}
		return nil
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "order_failed", err.Error())
		return
	}

	portfolio, score := s.afterTrade(r, round, team)
	_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "decision", decision)
	_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "order_submitted", order)
	if fill != nil {
		_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "fill", fill)
	}
	if portfolio != nil {
		_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "portfolio_snapshot", portfolio)
	}
	if score != nil {
		_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "score_snapshot", score)
	}
	writeJSON(w, http.StatusCreated, orderResponse{Decision: &decision, Order: order, Fill: fill, Portfolio: portfolio, Score: score})
}

func (s *Server) cancelOrder(w http.ResponseWriter, r *http.Request) {
	team, ok := teamFromContext(r.Context())
	if !ok {
		writeError(w, http.StatusUnauthorized, "missing_team", "team context missing")
		return
	}
	orderID, err := parseParamID(r, "order_id")
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_order_id", "order_id must be a positive integer")
		return
	}
	order, err := s.Store.GetOrder(r.Context(), orderID)
	if err != nil || order.TeamID != team.ID {
		writeError(w, http.StatusNotFound, "order_not_found", "order not found")
		return
	}
	round, err := s.Store.GetActiveRound(r.Context())
	if err != nil {
		writeError(w, http.StatusNotFound, "no_active_round", "no active round")
		return
	}
	if order.RoundID != round.ID {
		writeErrorDetails(w, http.StatusConflict, "order_not_in_active_round", "order does not belong to the active round", map[string]interface{}{"order_id": order.ID, "order_round_id": order.RoundID, "active_round_id": round.ID})
		return
	}
	if roundRequiresLockedAgent(round) {
		agent, ok := agentFromContext(r.Context())
		if !ok {
			writeErrorDetails(w, http.StatusForbidden, "round_agent_lock_required", "this round requires a registered locked agent", map[string]interface{}{"round_id": round.ID, "round_slug": round.Slug})
			return
		}
		if order.AgentID == nil || *order.AgentID != agent.ID {
			writeErrorDetails(w, http.StatusForbidden, "order_agent_mismatch", "locked-round orders can only be canceled by the agent that created them", map[string]interface{}{"order_id": order.ID, "agent_id": agent.ID})
			return
		}
	}
	if order.Status != "submitted" && order.Status != "open" {
		writeError(w, http.StatusBadRequest, "order_not_cancelable", "only submitted or open orders can be canceled")
		return
	}
	if err := s.Venue.CancelOrder(r.Context(), order.VenueOrderID); err != nil {
		writeError(w, http.StatusBadGateway, "venue_cancel_failed", err.Error())
		return
	}
	var updated store.Order
	err = s.Store.WithTx(r.Context(), func(tx *store.Tx) error {
		var err error
		updated, err = tx.UpdateOrderStatus(r.Context(), order.ID, "canceled", "")
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "cancel_failed", err.Error())
		return
	}
	s.invalidateLeaderboard(r.Context(), order.RoundID)
	writeJSON(w, http.StatusOK, updated)
}

func (s *Server) prepareTrade(r *http.Request, req tradeRequest) (store.Round, store.Market, []byte, error) {
	round, err := s.Store.GetActiveRound(r.Context())
	if err != nil {
		if latest, latestErr := s.Store.GetLatestRound(r.Context()); latestErr == nil && latest.Status == "paused" {
			return store.Round{}, store.Market{}, nil, errPausedRound
		}
		return store.Round{}, store.Market{}, nil, errNoActiveRound
	}
	market, err := s.Store.GetRoundMarket(r.Context(), round.ID, req.MarketID)
	if err != nil {
		return store.Round{}, store.Market{}, nil, errInvalidMarket
	}
	if market.Status != "open" {
		return store.Round{}, store.Market{}, nil, errMarketNotOpen
	}
	raw, _ := json.Marshal(req)
	return round, market, raw, nil
}

func (s *Server) validateTradeRequestShape(req tradeRequest) *apiErrorBody {
	if req.MarketID <= 0 {
		return &apiErrorBody{
			Code:    "invalid_market",
			Message: "market_id must be a positive integer",
			Details: map[string]interface{}{
				"field": "market_id",
			},
		}
	}
	if req.Action != "buy" && req.Action != "sell" {
		return &apiErrorBody{
			Code:    "invalid_action",
			Message: "action must be buy or sell",
			Details: map[string]interface{}{
				"field":   "action",
				"allowed": []string{"buy", "sell"},
			},
		}
	}
	if req.Outcome != "yes" && req.Outcome != "no" {
		return &apiErrorBody{
			Code:    "invalid_outcome",
			Message: "outcome must be yes or no",
			Details: map[string]interface{}{
				"field":   "outcome",
				"allowed": []string{"yes", "no"},
			},
		}
	}
	if req.AmountCents <= 0 {
		return &apiErrorBody{
			Code:    "invalid_amount",
			Message: "amount_cents must be positive",
			Details: map[string]interface{}{
				"field": "amount_cents",
			},
		}
	}
	if req.LimitPriceBPS != nil && (*req.LimitPriceBPS < s.Policy.MinLimitPriceBPS || *req.LimitPriceBPS > s.Policy.MaxLimitPriceBPS) {
		return &apiErrorBody{
			Code:    "malformed_limit_price",
			Message: "limit_price_bps must be between configured min and max",
			Details: map[string]interface{}{
				"field": "limit_price_bps",
				"min":   s.Policy.MinLimitPriceBPS,
				"max":   s.Policy.MaxLimitPriceBPS,
			},
		}
	}
	if req.EstimatedProbabilityBPS != nil && (*req.EstimatedProbabilityBPS < s.Policy.MinProbabilityBPS || *req.EstimatedProbabilityBPS > s.Policy.MaxProbabilityBPS) {
		return &apiErrorBody{
			Code:    "malformed_probability",
			Message: "estimated_probability_bps must be between configured min and max",
			Details: map[string]interface{}{
				"field": "estimated_probability_bps",
				"min":   s.Policy.MinProbabilityBPS,
				"max":   s.Policy.MaxProbabilityBPS,
			},
		}
	}
	return nil
}

func (s *Server) writeTradePrepError(w http.ResponseWriter, err error, marketID int64) {
	switch {
	case errors.Is(err, errPausedRound):
		writeError(w, http.StatusConflict, "paused_round", "round is paused")
	case errors.Is(err, errNoActiveRound):
		writeError(w, http.StatusNotFound, "no_active_round", "no active round")
	case errors.Is(err, errInvalidMarket):
		writeErrorDetails(w, http.StatusBadRequest, "invalid_market", "market is not available in the active round", map[string]interface{}{"market_id": marketID})
	case errors.Is(err, errMarketNotOpen):
		writeErrorDetails(w, http.StatusConflict, "market_not_open", "market is not open for new simulated trades", map[string]interface{}{"market_id": marketID})
	default:
		writeError(w, http.StatusBadRequest, "invalid_request", err.Error())
	}
}

func (s *Server) checkRisk(r *http.Request, team store.Team, round store.Round, market store.Market, req tradeRequest) (*risk.Violation, error) {
	openOrders, err := s.Store.CountOpenOrders(r.Context(), round.ID, team.ID)
	if err != nil {
		return nil, err
	}
	since := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339Nano)
	ordersLastMinute, err := s.Store.CountOrdersSince(r.Context(), round.ID, team.ID, since)
	if err != nil {
		return nil, err
	}
	marketExposure, err := s.Store.MarketExposure(r.Context(), round.ID, team.ID, market.ID)
	if err != nil {
		return nil, err
	}
	totalExposure, err := s.Store.TotalExposure(r.Context(), round.ID, team.ID)
	if err != nil {
		return nil, err
	}
	sellable, err := s.Store.QuantityForOutcome(r.Context(), round.ID, team.ID, market.ID, req.Outcome)
	if err != nil {
		return nil, err
	}
	portfolio, err := s.Store.ComputePortfolio(r.Context(), round.ID, team.ID)
	if err != nil {
		return nil, err
	}
	openBuyNotional, err := s.Store.OpenBuyNotional(r.Context(), round.ID, team.ID)
	if err != nil {
		return nil, err
	}
	limit := int64(0)
	if req.LimitPriceBPS != nil {
		limit = *req.LimitPriceBPS
	}
	return risk.Check(s.Policy, risk.Input{
		RoundStatus:              round.Status,
		TeamActive:               team.IsActive,
		Action:                   req.Action,
		Outcome:                  req.Outcome,
		AmountCents:              req.AmountCents,
		LimitPriceBPS:            req.LimitPriceBPS,
		EstimatedProbabilityBPS:  req.EstimatedProbabilityBPS,
		Reason:                   req.Reason,
		OpenOrders:               openOrders,
		OrdersLastMinute:         ordersLastMinute,
		RateLimitAllowed:         true,
		CashCents:                portfolio.CashCents,
		OpenBuyNotionalCents:     openBuyNotional,
		CurrentMarketExposure:    marketExposure,
		CurrentTotalExposure:     totalExposure,
		SellableOutcomeQuantity:  sellable,
		RequestedOutcomeQuantity: quantityForAmount(req.AmountCents, limit),
	}), nil
}

func (s *Server) rejectOrder(w http.ResponseWriter, r *http.Request, team store.Team, round store.Round, market store.Market, req tradeRequest, raw []byte, violation *risk.Violation) {
	limit := int64(0)
	if req.LimitPriceBPS != nil {
		limit = *req.LimitPriceBPS
	}
	observed := observedPrice(req.Outcome, market)
	edge := int64(0)
	if req.EstimatedProbabilityBPS != nil {
		edge = *req.EstimatedProbabilityBPS - observed
	}
	agentID := agentIDFromContext(r.Context())
	var decision *store.Decision
	var order store.Order
	var event store.RiskEvent
	err := s.Store.WithTx(r.Context(), func(tx *store.Tx) error {
		if req.EstimatedProbabilityBPS != nil {
			created, err := tx.CreateDecision(r.Context(), store.DecisionInput{
				RoundID:                 round.ID,
				TeamID:                  team.ID,
				AgentID:                 agentID,
				MarketID:                market.ID,
				ObservedPriceBPS:        observed,
				EstimatedProbabilityBPS: req.EstimatedProbabilityBPS,
				EdgeBPS:                 edge,
				Action:                  req.Action,
				Outcome:                 req.Outcome,
				AmountCents:             req.AmountCents,
				Confidence:              req.Confidence,
				Reason:                  req.Reason,
				RawPayloadJSON:          string(raw),
			})
			if err != nil {
				return err
			}
			decision = &created
		}
		var err error
		order, err = tx.CreateOrder(r.Context(), store.OrderInput{
			RoundID:         round.ID,
			TeamID:          team.ID,
			AgentID:         agentID,
			MarketID:        market.ID,
			Action:          req.Action,
			Outcome:         req.Outcome,
			AmountCents:     req.AmountCents,
			LimitPriceBPS:   limit,
			Status:          "rejected",
			RejectionReason: violation.Message,
		})
		if err != nil {
			return err
		}
		event, err = tx.CreateRiskEvent(r.Context(), store.RiskEvent{
			RoundID: round.ID,
			TeamID:  team.ID,
			AgentID: agentID,
			OrderID: &order.ID,
			Type:    violation.Type,
			Message: violation.Message,
		})
		return err
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "reject_failed", err.Error())
		return
	}
	s.invalidateLeaderboard(r.Context(), round.ID)
	_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "order_rejected", order)
	_ = s.Events.Append(r.Context(), round.Slug, team.Slug, "risk_event", event)
	writeJSON(w, http.StatusBadRequest, map[string]interface{}{
		"error": apiErrorBody{
			Code:    riskAPIErrorCode(violation.Type),
			Message: violation.Message,
			Details: map[string]interface{}{
				"risk_type": violation.Type,
				"order_id":  order.ID,
			},
		},
		"decision":   decision,
		"order":      order,
		"risk_event": event,
		"violation":  violation,
	})
}

func riskAPIErrorCode(violationType string) string {
	switch violationType {
	case "order_value_limit":
		return "amount_too_large"
	case "estimated_probability_required":
		return "missing_estimated_probability"
	case "estimated_probability_range":
		return "malformed_probability"
	case "too_many_open_orders":
		return "max_open_orders_exceeded"
	case "rate_limit":
		return "rate_limit_exceeded"
	case "limit_price_range":
		return "malformed_limit_price"
	case "limit_price_required":
		return "limit_price_required"
	case "reason_required":
		return "missing_reason"
	case "market_exposure_limit":
		return "market_exposure_exceeded"
	case "total_exposure_limit":
		return "total_exposure_exceeded"
	case "insufficient_cash":
		return "insufficient_cash"
	case "insufficient_position":
		return "insufficient_position"
	default:
		return "risk_limit_exceeded"
	}
}

func (s *Server) afterTrade(r *http.Request, round store.Round, team store.Team) (*store.PortfolioSnapshot, *store.ScoreSnapshot) {
	portfolio, err := s.Store.CreatePortfolioSnapshot(r.Context(), round.ID, team.ID)
	if err != nil {
		s.logWarn("portfolio snapshot failed", "error", err)
		return nil, nil
	}
	score, err := s.Store.RefreshScore(r.Context(), round.ID, team.ID)
	if err != nil {
		s.logWarn("score snapshot failed", "error", err)
		return &portfolio, nil
	}
	s.invalidateLeaderboard(r.Context(), round.ID)
	return &portfolio, &score
}

func observedPrice(outcome string, market store.Market) int64 {
	if outcome == "no" {
		return market.NoPriceBPS
	}
	return market.YesPriceBPS
}

func quantityForAmount(amountCents, priceBPS int64) int64 {
	if priceBPS <= 0 {
		return 0
	}
	return (amountCents * 10000) / priceBPS
}
