package risk

type Policy struct {
	InitialBalanceCents         int64 `json:"initial_balance_cents"`
	MaxOrderValueCents          int64 `json:"max_order_value_cents"`
	MaxPositionPerMarketCents   int64 `json:"max_position_per_market_cents"`
	MaxTotalExposureCents       int64 `json:"max_total_exposure_cents"`
	MaxOrdersPerMinute          int   `json:"max_orders_per_minute"`
	MaxOpenOrders               int   `json:"max_open_orders"`
	RequireReason               bool  `json:"require_reason"`
	RequireEstimatedProbability bool  `json:"require_estimated_probability"`
	AllowMarketOrders           bool  `json:"allow_market_orders"`
	MinProbabilityBPS           int64 `json:"min_probability_bps"`
	MaxProbabilityBPS           int64 `json:"max_probability_bps"`
	MinLimitPriceBPS            int64 `json:"min_limit_price_bps"`
	MaxLimitPriceBPS            int64 `json:"max_limit_price_bps"`
}

func DefaultPolicy() Policy {
	return Policy{
		InitialBalanceCents:         1000000,
		MaxOrderValueCents:          50000,
		MaxPositionPerMarketCents:   100000,
		MaxTotalExposureCents:       400000,
		MaxOrdersPerMinute:          10,
		MaxOpenOrders:               20,
		RequireReason:               true,
		RequireEstimatedProbability: true,
		AllowMarketOrders:           false,
		MinProbabilityBPS:           1,
		MaxProbabilityBPS:           9999,
		MinLimitPriceBPS:            1,
		MaxLimitPriceBPS:            9999,
	}
}
