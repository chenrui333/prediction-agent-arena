package store

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type ExportResult struct {
	RoundSlug      string   `json:"round_slug"`
	ExportDir      string   `json:"export_dir"`
	Artifacts      []string `json:"artifacts"`
	LeaderboardCSV string   `json:"leaderboard_csv"`
	ScoresJSONL    string   `json:"scores_jsonl"`
	PerMarketPnL   string   `json:"per_market_pnl_csv"`
	DecisionCSV    string   `json:"decision_quality_csv"`
	TradeCSV       string   `json:"trade_report_csv"`
	CalibrationCSV string   `json:"calibration_csv"`
	TeamBundleDir  string   `json:"team_bundle_dir"`
}

func (s *Store) ExportRound(ctx context.Context, roundID int64, exportRoot string) (ExportResult, error) {
	round, err := s.GetRound(ctx, roundID)
	if err != nil {
		return ExportResult{}, err
	}
	rows, err := s.ListLeaderboard(ctx, roundID)
	if err != nil {
		return ExportResult{}, err
	}
	dir := filepath.Join(exportRoot, round.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ExportResult{}, err
	}
	leaderboardPath := filepath.Join(dir, "leaderboard.csv")
	if err := writeLeaderboardCSV(leaderboardPath, rows); err != nil {
		return ExportResult{}, err
	}
	scoresPath := filepath.Join(dir, "scores.jsonl")
	if err := writeScoresJSONL(scoresPath, rows); err != nil {
		return ExportResult{}, err
	}
	teams, err := s.ListTeams(ctx)
	if err != nil {
		return ExportResult{}, err
	}
	pnlPath := filepath.Join(dir, "per_market_pnl.csv")
	if err := s.writePerMarketPnLCSV(ctx, pnlPath, roundID, teams); err != nil {
		return ExportResult{}, err
	}
	decisionPath := filepath.Join(dir, "decision_quality.csv")
	if err := s.writeDecisionQualityCSV(ctx, decisionPath, roundID); err != nil {
		return ExportResult{}, err
	}
	tradePath := filepath.Join(dir, "trade_report.csv")
	if err := s.writeTradeReportCSV(ctx, tradePath, roundID); err != nil {
		return ExportResult{}, err
	}
	calibrationPath := filepath.Join(dir, "calibration_bins.csv")
	if err := s.writeCalibrationCSV(ctx, calibrationPath, roundID); err != nil {
		return ExportResult{}, err
	}
	bundleDir := filepath.Join(dir, "teams")
	if err := os.MkdirAll(bundleDir, 0o755); err != nil {
		return ExportResult{}, err
	}
	if err := s.writeTeamBundles(ctx, bundleDir, round, rows, teams); err != nil {
		return ExportResult{}, err
	}
	artifacts := []string{leaderboardPath, scoresPath, pnlPath, decisionPath, tradePath, calibrationPath, bundleDir}
	return ExportResult{
		RoundSlug:      round.Slug,
		ExportDir:      dir,
		Artifacts:      artifacts,
		LeaderboardCSV: leaderboardPath,
		ScoresJSONL:    scoresPath,
		PerMarketPnL:   pnlPath,
		DecisionCSV:    decisionPath,
		TradeCSV:       tradePath,
		CalibrationCSV: calibrationPath,
		TeamBundleDir:  bundleDir,
	}, nil
}

func writeLeaderboardCSV(path string, rows []LeaderboardRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"rank", "team_slug", "team_name", "composite_score", "equity_cents", "return_bps", "max_drawdown_bps", "brier_score_bps", "trade_count", "gross_exposure_cents", "last_heartbeat", "status"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			strconv.FormatInt(row.Rank, 10),
			row.TeamSlug,
			row.TeamName,
			strconv.FormatInt(row.CompositeScore, 10),
			strconv.FormatInt(row.EquityCents, 10),
			strconv.FormatInt(row.ReturnBPS, 10),
			strconv.FormatInt(row.MaxDrawdownBPS, 10),
			strconv.FormatInt(row.BrierScoreBPS, 10),
			strconv.FormatInt(row.TradeCount, 10),
			strconv.FormatInt(row.GrossExposureCents, 10),
			row.LastHeartbeat,
			row.Status,
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeScoresJSONL(path string, rows []LeaderboardRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) writePerMarketPnLCSV(ctx context.Context, path string, roundID int64, teams []Team) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"team_slug", "market_slug", "outcome", "quantity_cents", "avg_entry_price_bps", "realized_pnl_cents", "unrealized_pnl_cents", "mark_value_cents"}); err != nil {
		return err
	}
	for _, team := range teams {
		rows, err := s.ListPerMarketPnL(ctx, roundID, team.ID)
		if err != nil {
			return err
		}
		for _, row := range rows {
			if err := writer.Write([]string{
				team.Slug,
				row.MarketSlug,
				row.Outcome,
				strconv.FormatInt(row.QuantityCents, 10),
				strconv.FormatInt(row.AvgEntryPriceBPS, 10),
				strconv.FormatInt(row.RealizedPNLCents, 10),
				strconv.FormatInt(row.UnrealizedPNLCents, 10),
				strconv.FormatInt(row.MarkValueCents, 10),
			}); err != nil {
				return err
			}
		}
	}
	return writer.Error()
}

func (s *Store) writeDecisionQualityCSV(ctx context.Context, path string, roundID int64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"team_slug", "market_slug", "decision_id", "outcome", "resolved_outcome", "estimated_probability_bps", "brier_error_bps", "edge_bps", "action", "amount_cents", "created_at"}); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.slug, m.slug, d.id, d.outcome, mo.outcome, d.estimated_probability_bps,
			((d.estimated_probability_bps - CASE WHEN d.outcome = mo.outcome THEN 10000 ELSE 0 END)
			* (d.estimated_probability_bps - CASE WHEN d.outcome = mo.outcome THEN 10000 ELSE 0 END)) / 10000,
			d.edge_bps, d.action, d.amount_cents, d.created_at
		FROM decisions d
		JOIN teams t ON t.id = d.team_id
		JOIN markets m ON m.id = d.market_id
		JOIN market_outcomes mo ON mo.market_id = d.market_id
		WHERE d.round_id = ?
			AND d.estimated_probability_bps IS NOT NULL
			AND mo.outcome IN ('yes', 'no')
			AND mo.resolved_at IS NOT NULL
		ORDER BY t.id, d.id
	`, roundID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		values := make([]string, 11)
		if err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &values[8], &values[9], &values[10]); err != nil {
			return err
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) writeTradeReportCSV(ctx context.Context, path string, roundID int64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"team_slug", "market_slug", "fill_id", "order_id", "action", "outcome", "amount_cents", "fill_price_bps", "current_mark_bps", "estimated_trade_pnl_cents", "created_at"}); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.slug, m.slug, f.id, f.order_id, f.action, f.outcome, f.amount_cents, f.fill_price_bps,
			CASE f.outcome WHEN 'yes' THEN m.yes_price_bps ELSE m.no_price_bps END,
			CASE f.action
				WHEN 'buy' THEN ((f.amount_cents * (CASE f.outcome WHEN 'yes' THEN m.yes_price_bps ELSE m.no_price_bps END)) / f.fill_price_bps) - f.amount_cents
				ELSE f.amount_cents - ((f.amount_cents * (CASE f.outcome WHEN 'yes' THEN m.yes_price_bps ELSE m.no_price_bps END)) / f.fill_price_bps)
			END,
			f.created_at
		FROM fills f
		JOIN teams t ON t.id = f.team_id
		JOIN markets m ON m.id = f.market_id
		WHERE f.round_id = ?
		ORDER BY estimated_trade_pnl_cents DESC, f.id
	`, roundID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		values := make([]string, 11)
		if err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5], &values[6], &values[7], &values[8], &values[9], &values[10]); err != nil {
			return err
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) writeCalibrationCSV(ctx context.Context, path string, roundID int64) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"team_slug", "probability_bucket_bps", "decision_count", "observed_success_count", "observed_success_rate_bps", "avg_brier_error_bps"}); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT t.slug,
			(d.estimated_probability_bps / 1000) * 1000 AS bucket,
			COUNT(*) AS decision_count,
			SUM(CASE WHEN d.outcome = mo.outcome THEN 1 ELSE 0 END) AS success_count,
			(SUM(CASE WHEN d.outcome = mo.outcome THEN 1 ELSE 0 END) * 10000) / COUNT(*) AS success_rate_bps,
			AVG(((d.estimated_probability_bps - CASE WHEN d.outcome = mo.outcome THEN 10000 ELSE 0 END)
			* (d.estimated_probability_bps - CASE WHEN d.outcome = mo.outcome THEN 10000 ELSE 0 END)) / 10000) AS avg_brier
		FROM decisions d
		JOIN teams t ON t.id = d.team_id
		JOIN market_outcomes mo ON mo.market_id = d.market_id
		WHERE d.round_id = ?
			AND d.estimated_probability_bps IS NOT NULL
			AND mo.outcome IN ('yes', 'no')
			AND mo.resolved_at IS NOT NULL
		GROUP BY t.slug, bucket
		ORDER BY t.slug, bucket
	`, roundID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		values := make([]string, 6)
		if err := rows.Scan(&values[0], &values[1], &values[2], &values[3], &values[4], &values[5]); err != nil {
			return err
		}
		if err := writer.Write(values); err != nil {
			return err
		}
	}
	return rows.Err()
}

func (s *Store) writeTeamBundles(ctx context.Context, dir string, round Round, leaderboard []LeaderboardRow, teams []Team) error {
	settlements, err := s.ListSettlements(ctx, round.ID)
	if err != nil {
		return err
	}
	leaderboardByTeam := map[int64]LeaderboardRow{}
	for _, row := range leaderboard {
		leaderboardByTeam[row.TeamID] = row
	}
	settlementsByTeam := map[int64][]Settlement{}
	for _, settlement := range settlements {
		settlementsByTeam[settlement.TeamID] = append(settlementsByTeam[settlement.TeamID], settlement)
	}
	for _, team := range teams {
		pnl, err := s.ListPerMarketPnL(ctx, round.ID, team.ID)
		if err != nil {
			return err
		}
		activity, err := s.TeamActivity(ctx, team.Slug, round.ID, 100)
		if err != nil {
			return err
		}
		bundle := map[string]interface{}{
			"team":           team,
			"round":          round,
			"leaderboard":    leaderboardByTeam[team.ID],
			"portfolio":      activity.Portfolio,
			"decisions":      activity.Decisions,
			"orders":         activity.Orders,
			"fills":          activity.Fills,
			"risk_events":    activity.RiskEvents,
			"per_market_pnl": pnl,
			"settlements":    settlementsByTeam[team.ID],
		}
		raw, err := json.MarshalIndent(bundle, "", "  ")
		if err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, team.Slug+".json"), raw, 0o644); err != nil {
			return err
		}
	}
	return nil
}
