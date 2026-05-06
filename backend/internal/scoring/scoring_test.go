package scoring

import "testing"

func TestComputeDeterministic(t *testing.T) {
	stats := Stats{
		InitialBalanceCents: 1000000,
		EquityCents:         1010000,
		MaxDrawdownBPS:      100,
		GrossExposureCents:  50000,
		TradeCount:          3,
		OrderCount:          4,
		RejectedOrderCount:  1,
		AverageSlippageBPS:  25,
	}
	a := Compute(stats)
	b := Compute(stats)
	if a != b {
		t.Fatalf("scores differ: %#v != %#v", a, b)
	}
	if a.CompositeScore <= 0 || a.CompositeScore > 100 {
		t.Fatalf("composite out of range: %d", a.CompositeScore)
	}
}

func TestComputeHandlesEdgeCases(t *testing.T) {
	brier := int64(2500)
	tests := []struct {
		name string
		in   Stats
	}{
		{
			name: "no trades and zero initial balance",
			in:   Stats{},
		},
		{
			name: "missing brier stays neutral",
			in: Stats{
				InitialBalanceCents: 1000000,
				EquityCents:         1000000,
			},
		},
		{
			name: "large losses clamp scores",
			in: Stats{
				InitialBalanceCents: 1000000,
				EquityCents:         -500000,
				MaxDrawdownBPS:      50000,
				GrossExposureCents:  9000000,
				OrderCount:          1,
				RejectedOrderCount:  10,
				AverageSlippageBPS:  100000,
				BrierScoreBPS:       &brier,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Compute(tt.in)
			assertScoreRange(t, "composite", got.CompositeScore)
			assertScoreRange(t, "return", got.ReturnScore)
			assertScoreRange(t, "risk", got.RiskScore)
			assertScoreRange(t, "calibration", got.CalibrationScore)
			assertScoreRange(t, "execution", got.ExecutionScore)
			assertScoreRange(t, "cost", got.CostScore)
			if tt.in.BrierScoreBPS == nil && got.BrierScoreBPS != 5000 {
				t.Fatalf("brier = %d, want neutral 5000", got.BrierScoreBPS)
			}
		})
	}
}

func assertScoreRange(t *testing.T, name string, got int64) {
	t.Helper()
	if got < 0 || got > 100 {
		t.Fatalf("%s score = %d, want 0..100", name, got)
	}
}
