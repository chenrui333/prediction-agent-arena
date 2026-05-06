package risk

import (
	"errors"
	"strings"
)

var ErrRejected = errors.New("risk rejected")

type Input struct {
	RoundStatus              string
	TeamActive               bool
	Action                   string
	Outcome                  string
	AmountCents              int64
	LimitPriceBPS            *int64
	EstimatedProbabilityBPS  *int64
	Reason                   string
	OpenOrders               int
	OrdersLastMinute         int
	RateLimitAllowed         bool
	CashCents                int64
	OpenBuyNotionalCents     int64
	CurrentMarketExposure    int64
	CurrentTotalExposure     int64
	SellableOutcomeQuantity  int64
	RequestedOutcomeQuantity int64
}

type Violation struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func Check(policy Policy, input Input) *Violation {
	if input.RoundStatus != "active" {
		return reject("round_not_active", "round is not active")
	}
	if !input.TeamActive {
		return reject("team_paused", "team is paused")
	}
	if input.Action != "buy" && input.Action != "sell" {
		return reject("invalid_action", "action must be buy or sell")
	}
	if input.Outcome != "yes" && input.Outcome != "no" {
		return reject("invalid_outcome", "outcome must be yes or no")
	}
	if input.AmountCents <= 0 {
		return reject("invalid_amount", "amount_cents must be positive")
	}
	if input.AmountCents > policy.MaxOrderValueCents {
		return reject("order_value_limit", "order amount exceeds max_order_value_cents")
	}
	if !policy.AllowMarketOrders && input.LimitPriceBPS == nil {
		return reject("limit_price_required", "limit_price_bps is required")
	}
	if input.LimitPriceBPS != nil && (*input.LimitPriceBPS < policy.MinLimitPriceBPS || *input.LimitPriceBPS > policy.MaxLimitPriceBPS) {
		return reject("limit_price_range", "limit_price_bps must be between configured min and max")
	}
	if policy.RequireEstimatedProbability && input.EstimatedProbabilityBPS == nil {
		return reject("estimated_probability_required", "estimated_probability_bps is required")
	}
	if input.EstimatedProbabilityBPS != nil && (*input.EstimatedProbabilityBPS < policy.MinProbabilityBPS || *input.EstimatedProbabilityBPS > policy.MaxProbabilityBPS) {
		return reject("estimated_probability_range", "estimated_probability_bps must be between configured min and max")
	}
	if policy.RequireReason && strings.TrimSpace(input.Reason) == "" {
		return reject("reason_required", "reason is required")
	}
	if input.OpenOrders >= policy.MaxOpenOrders {
		return reject("too_many_open_orders", "team has too many open orders")
	}
	if input.OrdersLastMinute >= policy.MaxOrdersPerMinute || !input.RateLimitAllowed {
		return reject("rate_limit", "team exceeded max_orders_per_minute")
	}
	if input.Action == "buy" {
		if input.AmountCents+input.OpenBuyNotionalCents > input.CashCents {
			return reject("insufficient_cash", "buy order exceeds available simulated cash")
		}
		if input.CurrentMarketExposure+input.AmountCents > policy.MaxPositionPerMarketCents {
			return reject("market_exposure_limit", "order would exceed max_position_per_market_cents")
		}
		if input.CurrentTotalExposure+input.AmountCents > policy.MaxTotalExposureCents {
			return reject("total_exposure_limit", "order would exceed max_total_exposure_cents")
		}
	}
	if input.Action == "sell" && input.RequestedOutcomeQuantity > input.SellableOutcomeQuantity {
		return reject("insufficient_position", "sell order exceeds available simulated position")
	}
	return nil
}

func reject(kind, message string) *Violation {
	return &Violation{Type: kind, Message: message}
}
