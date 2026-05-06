package scoring

type Stats struct {
	InitialBalanceCents int64
	EquityCents         int64
	MaxDrawdownBPS      int64
	GrossExposureCents  int64
	BrierScoreBPS       *int64
	TradeCount          int64
	OrderCount          int64
	RejectedOrderCount  int64
	AverageSlippageBPS  int64
}

type Snapshot struct {
	CompositeScore   int64 `json:"composite_score"`
	ReturnScore      int64 `json:"return_score"`
	RiskScore        int64 `json:"risk_score"`
	CalibrationScore int64 `json:"calibration_score"`
	ExecutionScore   int64 `json:"execution_score"`
	CostScore        int64 `json:"cost_score"`
	EquityCents      int64 `json:"equity_cents"`
	ReturnBPS        int64 `json:"return_bps"`
	MaxDrawdownBPS   int64 `json:"max_drawdown_bps"`
	BrierScoreBPS    int64 `json:"brier_score_bps"`
	TradeCount       int64 `json:"trade_count"`
}

func Compute(stats Stats) Snapshot {
	initial := stats.InitialBalanceCents
	if initial <= 0 {
		initial = 1
	}
	returnBPS := ((stats.EquityCents - initial) * 10000) / initial
	returnScore := clamp(50+(returnBPS/100), 0, 100)

	riskPenalty := stats.MaxDrawdownBPS/100 + stats.GrossExposureCents*20/initial
	riskScore := clamp(100-riskPenalty, 0, 100)

	brier := int64(5000)
	calibrationScore := int64(50)
	if stats.BrierScoreBPS != nil {
		brier = *stats.BrierScoreBPS
		calibrationScore = clamp(100-(brier/100), 0, 100)
	}

	executionScore := int64(50)
	if stats.OrderCount > 0 {
		rejectionPenalty := stats.RejectedOrderCount * 100 / stats.OrderCount
		slippagePenalty := stats.AverageSlippageBPS / 50
		executionScore = clamp(100-rejectionPenalty-slippagePenalty, 0, 100)
	}

	costScore := int64(100)
	composite := (40*returnScore + 20*riskScore + 20*calibrationScore + 10*executionScore + 10*costScore) / 100
	return Snapshot{
		CompositeScore:   composite,
		ReturnScore:      returnScore,
		RiskScore:        riskScore,
		CalibrationScore: calibrationScore,
		ExecutionScore:   executionScore,
		CostScore:        costScore,
		EquityCents:      stats.EquityCents,
		ReturnBPS:        returnBPS,
		MaxDrawdownBPS:   stats.MaxDrawdownBPS,
		BrierScoreBPS:    brier,
		TradeCount:       stats.TradeCount,
	}
}

func clamp(v, min, max int64) int64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}
