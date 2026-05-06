package store

import (
	"context"
	"database/sql"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/scoring"
)

func (s *Store) ScoreStats(ctx context.Context, roundID, teamID int64) (scoring.Stats, error) {
	round, err := s.GetRound(ctx, roundID)
	if err != nil {
		return scoring.Stats{}, err
	}
	portfolio, err := s.ComputePortfolio(ctx, roundID, teamID)
	if err != nil {
		return scoring.Stats{}, err
	}
	var tradeCount, orderCount, rejectedCount, avgSlippage sql.NullInt64
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM fills WHERE round_id = ? AND team_id = ?", roundID, teamID).Scan(&tradeCount); err != nil {
		return scoring.Stats{}, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE round_id = ? AND team_id = ?", roundID, teamID).Scan(&orderCount); err != nil {
		return scoring.Stats{}, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE round_id = ? AND team_id = ? AND status = 'rejected'", roundID, teamID).Scan(&rejectedCount); err != nil {
		return scoring.Stats{}, err
	}
	if err := s.db.QueryRowContext(ctx, "SELECT CAST(COALESCE(AVG(slippage_bps), 0) AS INTEGER) FROM fills WHERE round_id = ? AND team_id = ?", roundID, teamID).Scan(&avgSlippage); err != nil {
		return scoring.Stats{}, err
	}
	brierScore, err := s.BrierScoreBPS(ctx, roundID, teamID)
	if err != nil {
		return scoring.Stats{}, err
	}
	return scoring.Stats{
		InitialBalanceCents: round.InitialBalanceCents,
		EquityCents:         portfolio.EquityCents,
		MaxDrawdownBPS:      portfolio.MaxDrawdownBPS,
		GrossExposureCents:  portfolio.GrossExposureCents,
		BrierScoreBPS:       brierScore,
		TradeCount:          tradeCount.Int64,
		OrderCount:          orderCount.Int64,
		RejectedOrderCount:  rejectedCount.Int64,
		AverageSlippageBPS:  avgSlippage.Int64,
	}, nil
}

func (s *Store) BrierScoreBPS(ctx context.Context, roundID, teamID int64) (*int64, error) {
	var count, sum sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT
			COUNT(*),
			COALESCE(SUM(
				((d.estimated_probability_bps - CASE WHEN d.outcome = mo.outcome THEN 10000 ELSE 0 END)
				* (d.estimated_probability_bps - CASE WHEN d.outcome = mo.outcome THEN 10000 ELSE 0 END)) / 10000
			), 0)
		FROM decisions d
		JOIN market_outcomes mo ON mo.market_id = d.market_id
		WHERE d.round_id = ?
			AND d.team_id = ?
			AND d.estimated_probability_bps IS NOT NULL
			AND mo.outcome IN ('yes', 'no')
			AND mo.resolved_at IS NOT NULL
	`, roundID, teamID).Scan(&count, &sum)
	if err != nil {
		return nil, err
	}
	if !count.Valid || count.Int64 == 0 {
		return nil, nil
	}
	score := sum.Int64 / count.Int64
	return &score, nil
}

func (s *Store) CreateScoreSnapshot(ctx context.Context, roundID, teamID int64, score scoring.Snapshot) (ScoreSnapshot, error) {
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO score_snapshots(round_id, team_id, composite_score, return_score, risk_score, calibration_score, execution_score, cost_score, equity_cents, return_bps, max_drawdown_bps, brier_score_bps, trade_count, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, roundID, teamID, score.CompositeScore, score.ReturnScore, score.RiskScore, score.CalibrationScore, score.ExecutionScore, score.CostScore, score.EquityCents, score.ReturnBPS, score.MaxDrawdownBPS, score.BrierScoreBPS, score.TradeCount, Now())
	if err != nil {
		return ScoreSnapshot{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetScoreSnapshot(ctx, id)
}

func (s *Store) GetScoreSnapshot(ctx context.Context, id int64) (ScoreSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, composite_score, return_score, risk_score, calibration_score, execution_score, cost_score, equity_cents, return_bps, max_drawdown_bps, brier_score_bps, trade_count, created_at
		FROM score_snapshots
		WHERE id = ?
	`, id)
	score, err := scanScore(row)
	return score, normalizeErr(err)
}

func (s *Store) RefreshScore(ctx context.Context, roundID, teamID int64) (ScoreSnapshot, error) {
	stats, err := s.ScoreStats(ctx, roundID, teamID)
	if err != nil {
		return ScoreSnapshot{}, err
	}
	return s.CreateScoreSnapshot(ctx, roundID, teamID, scoring.Compute(stats))
}

func (s *Store) RefreshRoundScores(ctx context.Context, roundID int64) error {
	teams, err := s.ListTeams(ctx)
	if err != nil {
		return err
	}
	for _, team := range teams {
		if _, err := s.CreatePortfolioSnapshot(ctx, roundID, team.ID); err != nil {
			return err
		}
		if _, err := s.RefreshScore(ctx, roundID, team.ID); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) ListLeaderboard(ctx context.Context, roundID int64) ([]LeaderboardRow, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH latest_scores AS (
			SELECT ss.*
			FROM score_snapshots ss
			JOIN (
				SELECT round_id, team_id, MAX(id) AS id
				FROM score_snapshots
				WHERE round_id = ?
				GROUP BY round_id, team_id
			) latest ON latest.id = ss.id
		),
		latest_heartbeats AS (
			SELECT hb.*
			FROM agent_heartbeats hb
			JOIN (
				SELECT round_id, team_id, MAX(id) AS id
				FROM agent_heartbeats
				WHERE round_id = ?
				GROUP BY round_id, team_id
			) latest ON latest.id = hb.id
		),
		latest_portfolios AS (
			SELECT ps.*
			FROM portfolio_snapshots ps
			JOIN (
				SELECT round_id, team_id, MAX(id) AS id
				FROM portfolio_snapshots
				WHERE round_id = ?
				GROUP BY round_id, team_id
			) latest ON latest.id = ps.id
		)
		SELECT
			t.id,
			t.slug,
			t.name,
			COALESCE(ls.composite_score, 50),
			COALESCE(ls.equity_cents, r.initial_balance_cents),
			COALESCE(ls.return_bps, 0),
			COALESCE(ls.max_drawdown_bps, 0),
			COALESCE(ls.brier_score_bps, 5000),
			COALESCE(ls.trade_count, 0),
			COALESCE(lp.gross_exposure_cents, 0),
			COALESCE(lh.created_at, ''),
			CASE WHEN t.is_active = 0 THEN 'paused' ELSE COALESCE(lh.status, 'offline') END
		FROM teams t
		CROSS JOIN rounds r
		LEFT JOIN latest_scores ls ON ls.team_id = t.id
		LEFT JOIN latest_heartbeats lh ON lh.team_id = t.id
		LEFT JOIN latest_portfolios lp ON lp.team_id = t.id
		WHERE r.id = ?
		ORDER BY COALESCE(ls.composite_score, 50) DESC, COALESCE(ls.equity_cents, r.initial_balance_cents) DESC, t.id ASC
	`, roundID, roundID, roundID, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := []LeaderboardRow{}
	for rows.Next() {
		var row LeaderboardRow
		if err := rows.Scan(&row.TeamID, &row.TeamSlug, &row.TeamName, &row.CompositeScore, &row.EquityCents, &row.ReturnBPS, &row.MaxDrawdownBPS, &row.BrierScoreBPS, &row.TradeCount, &row.GrossExposureCents, &row.LastHeartbeat, &row.Status); err != nil {
			return nil, err
		}
		row.Rank = int64(len(result) + 1)
		result = append(result, row)
	}
	return result, rows.Err()
}

type scoreScanner interface {
	Scan(dest ...interface{}) error
}

func scanScore(row scoreScanner) (ScoreSnapshot, error) {
	var score ScoreSnapshot
	err := row.Scan(&score.ID, &score.RoundID, &score.TeamID, &score.CompositeScore, &score.ReturnScore, &score.RiskScore, &score.CalibrationScore, &score.ExecutionScore, &score.CostScore, &score.EquityCents, &score.ReturnBPS, &score.MaxDrawdownBPS, &score.BrierScoreBPS, &score.TradeCount, &score.CreatedAt)
	return score, err
}
