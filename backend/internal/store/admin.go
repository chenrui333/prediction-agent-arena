package store

import (
	"context"
)

type TeamActivity struct {
	Team       Team              `json:"team"`
	Round      Round             `json:"round"`
	Portfolio  PortfolioSnapshot `json:"portfolio"`
	Decisions  []Decision        `json:"decisions"`
	Orders     []Order           `json:"orders"`
	Fills      []Fill            `json:"fills"`
	RiskEvents []RiskEvent       `json:"risk_events"`
}

func (s *Store) ListAdminTeamStats(ctx context.Context, roundID int64) ([]AdminTeamStats, error) {
	rows, err := s.db.QueryContext(ctx, `
		WITH latest_heartbeats AS (
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
		),
		trade_counts AS (
			SELECT team_id, COUNT(*) AS trade_count
			FROM fills
			WHERE round_id = ?
			GROUP BY team_id
		),
		risk_counts AS (
			SELECT team_id, COUNT(*) AS risk_count
			FROM risk_events
			WHERE round_id = ?
			GROUP BY team_id
		)
		SELECT
			t.id,
			t.slug,
			t.name,
			t.is_active,
			CASE WHEN t.is_active = 0 THEN 'paused' ELSE COALESCE(lh.status, 'offline') END,
			COALESCE(lh.created_at, ''),
			COALESCE(lp.equity_cents, r.initial_balance_cents),
			COALESCE(tc.trade_count, 0),
			COALESCE(rc.risk_count, 0),
			COALESCE(lp.gross_exposure_cents, 0)
		FROM teams t
		CROSS JOIN rounds r
		LEFT JOIN latest_heartbeats lh ON lh.team_id = t.id
		LEFT JOIN latest_portfolios lp ON lp.team_id = t.id
		LEFT JOIN trade_counts tc ON tc.team_id = t.id
		LEFT JOIN risk_counts rc ON rc.team_id = t.id
		WHERE r.id = ?
		ORDER BY t.id
	`, roundID, roundID, roundID, roundID, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	stats := []AdminTeamStats{}
	for rows.Next() {
		var stat AdminTeamStats
		var active int64
		if err := rows.Scan(&stat.TeamID, &stat.TeamSlug, &stat.TeamName, &active, &stat.Status, &stat.LastHeartbeat, &stat.EquityCents, &stat.TradeCount, &stat.RiskRejectionCount, &stat.GrossExposureCents); err != nil {
			return nil, err
		}
		stat.IsActive = scanBool(active)
		stats = append(stats, stat)
	}
	return stats, rows.Err()
}

func (s *Store) ResetRound(ctx context.Context, roundID int64) error {
	return s.WithTx(ctx, func(tx *Tx) error {
		tables := []string{"agent_heartbeats", "score_snapshots", "risk_events", "positions", "fills", "orders", "decisions", "portfolio_snapshots"}
		for _, table := range tables {
			if _, err := tx.tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE round_id = ?", roundID); err != nil {
				return err
			}
		}
		_, err := tx.tx.ExecContext(ctx, "UPDATE rounds SET status = 'draft', updated_at = ? WHERE id = ?", Now(), roundID)
		return err
	})
}

func (s *Store) TeamActivity(ctx context.Context, teamSlug string, roundID int64, limit int) (TeamActivity, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	team, err := s.GetTeamBySlug(ctx, teamSlug)
	if err != nil {
		return TeamActivity{}, err
	}
	round, err := s.GetRound(ctx, roundID)
	if err != nil {
		return TeamActivity{}, err
	}
	portfolio, err := s.ComputePortfolio(ctx, round.ID, team.ID)
	if err != nil {
		return TeamActivity{}, err
	}
	decisions, err := s.ListRecentDecisions(ctx, round.ID, team.ID, limit)
	if err != nil {
		return TeamActivity{}, err
	}
	orders, err := s.ListRecentOrders(ctx, round.ID, team.ID, limit)
	if err != nil {
		return TeamActivity{}, err
	}
	fills, err := s.ListRecentFills(ctx, round.ID, team.ID, limit)
	if err != nil {
		return TeamActivity{}, err
	}
	riskEvents, err := s.ListRecentRiskEvents(ctx, round.ID, team.ID, limit)
	if err != nil {
		return TeamActivity{}, err
	}
	return TeamActivity{Team: team, Round: round, Portfolio: portfolio, Decisions: decisions, Orders: orders, Fills: fills, RiskEvents: riskEvents}, nil
}
