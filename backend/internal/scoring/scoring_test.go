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
