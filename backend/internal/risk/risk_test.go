package risk

import "testing"

func TestRiskRejections(t *testing.T) {
	policy := DefaultPolicy()
	price := int64(5700)
	prob := int64(6400)
	tooLowProb := int64(0)
	tests := []struct {
		name string
		in   Input
		want string
	}{
		{
			name: "order too large",
			in: Input{
				RoundStatus:             "active",
				TeamActive:              true,
				Action:                  "buy",
				Outcome:                 "yes",
				AmountCents:             policy.MaxOrderValueCents + 1,
				LimitPriceBPS:           &price,
				EstimatedProbabilityBPS: &prob,
				Reason:                  "edge",
				RateLimitAllowed:        true,
			},
			want: "order_value_limit",
		},
		{
			name: "missing probability",
			in: Input{
				RoundStatus:      "active",
				TeamActive:       true,
				Action:           "buy",
				Outcome:          "yes",
				AmountCents:      1000,
				LimitPriceBPS:    &price,
				Reason:           "edge",
				RateLimitAllowed: true,
			},
			want: "estimated_probability_required",
		},
		{
			name: "malformed probability",
			in: Input{
				RoundStatus:             "active",
				TeamActive:              true,
				Action:                  "buy",
				Outcome:                 "yes",
				AmountCents:             1000,
				LimitPriceBPS:           &price,
				EstimatedProbabilityBPS: &tooLowProb,
				Reason:                  "edge",
				RateLimitAllowed:        true,
			},
			want: "estimated_probability_range",
		},
		{
			name: "max open orders",
			in: Input{
				RoundStatus:             "active",
				TeamActive:              true,
				Action:                  "buy",
				Outcome:                 "yes",
				AmountCents:             1000,
				LimitPriceBPS:           &price,
				EstimatedProbabilityBPS: &prob,
				Reason:                  "edge",
				OpenOrders:              policy.MaxOpenOrders,
				RateLimitAllowed:        true,
			},
			want: "too_many_open_orders",
		},
		{
			name: "rate limit",
			in: Input{
				RoundStatus:             "active",
				TeamActive:              true,
				Action:                  "buy",
				Outcome:                 "yes",
				AmountCents:             1000,
				LimitPriceBPS:           &price,
				EstimatedProbabilityBPS: &prob,
				Reason:                  "edge",
				OrdersLastMinute:        policy.MaxOrdersPerMinute,
				RateLimitAllowed:        true,
			},
			want: "rate_limit",
		},
		{
			name: "market exposure",
			in: Input{
				RoundStatus:              "active",
				TeamActive:               true,
				Action:                   "buy",
				Outcome:                  "yes",
				AmountCents:              1000,
				LimitPriceBPS:            &price,
				EstimatedProbabilityBPS:  &prob,
				Reason:                   "edge",
				RateLimitAllowed:         true,
				CurrentMarketExposure:    policy.MaxPositionPerMarketCents,
				RequestedOutcomeQuantity: 1000,
			},
			want: "market_exposure_limit",
		},
		{
			name: "total exposure",
			in: Input{
				RoundStatus:              "active",
				TeamActive:               true,
				Action:                   "buy",
				Outcome:                  "yes",
				AmountCents:              1000,
				LimitPriceBPS:            &price,
				EstimatedProbabilityBPS:  &prob,
				Reason:                   "edge",
				RateLimitAllowed:         true,
				CurrentTotalExposure:     policy.MaxTotalExposureCents,
				RequestedOutcomeQuantity: 1000,
			},
			want: "total_exposure_limit",
		},
		{
			name: "insufficient sell position",
			in: Input{
				RoundStatus:              "active",
				TeamActive:               true,
				Action:                   "sell",
				Outcome:                  "yes",
				AmountCents:              1000,
				LimitPriceBPS:            &price,
				EstimatedProbabilityBPS:  &prob,
				Reason:                   "edge",
				RateLimitAllowed:         true,
				SellableOutcomeQuantity:  100,
				RequestedOutcomeQuantity: 200,
			},
			want: "insufficient_position",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Check(policy, tt.in)
			if got == nil || got.Type != tt.want {
				t.Fatalf("got %#v, want %s", got, tt.want)
			}
		})
	}
}
