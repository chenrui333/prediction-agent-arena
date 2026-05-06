package store

import (
	"context"
	"database/sql"
)

func (s *Store) CreatePortfolioSnapshot(ctx context.Context, roundID, teamID int64) (PortfolioSnapshot, error) {
	snapshot, err := s.ComputePortfolio(ctx, roundID, teamID)
	if err != nil {
		return PortfolioSnapshot{}, err
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO portfolio_snapshots(round_id, team_id, cash_cents, equity_cents, realized_pnl_cents, unrealized_pnl_cents, gross_exposure_cents, max_drawdown_bps, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, roundID, teamID, snapshot.CashCents, snapshot.EquityCents, snapshot.RealizedPNLCents, snapshot.UnrealizedPNLCents, snapshot.GrossExposureCents, snapshot.MaxDrawdownBPS, Now())
	if err != nil {
		return PortfolioSnapshot{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetPortfolioSnapshot(ctx, id)
}

func (s *Store) LatestPortfolioSnapshot(ctx context.Context, roundID, teamID int64) (PortfolioSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, cash_cents, equity_cents, realized_pnl_cents, unrealized_pnl_cents, gross_exposure_cents, max_drawdown_bps, created_at
		FROM portfolio_snapshots
		WHERE round_id = ? AND team_id = ?
		ORDER BY id DESC
		LIMIT 1
	`, roundID, teamID)
	snapshot, err := scanPortfolio(row)
	return snapshot, normalizeErr(err)
}

func (s *Store) GetPortfolioSnapshot(ctx context.Context, id int64) (PortfolioSnapshot, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, cash_cents, equity_cents, realized_pnl_cents, unrealized_pnl_cents, gross_exposure_cents, max_drawdown_bps, created_at
		FROM portfolio_snapshots
		WHERE id = ?
	`, id)
	snapshot, err := scanPortfolio(row)
	return snapshot, normalizeErr(err)
}

func (s *Store) ComputePortfolio(ctx context.Context, roundID, teamID int64) (PortfolioSnapshot, error) {
	round, err := s.GetRound(ctx, roundID)
	if err != nil {
		return PortfolioSnapshot{}, err
	}
	var buyCost, sellProceeds, fees sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `
		SELECT
			COALESCE(SUM(CASE WHEN action = 'buy' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(CASE WHEN action = 'sell' THEN amount_cents ELSE 0 END), 0),
			COALESCE(SUM(fee_cents), 0)
		FROM fills
		WHERE round_id = ? AND team_id = ?
	`, roundID, teamID).Scan(&buyCost, &sellProceeds, &fees); err != nil {
		return PortfolioSnapshot{}, err
	}
	cash := round.InitialBalanceCents - buyCost.Int64 + sellProceeds.Int64 - fees.Int64

	grossExposure, err := s.TotalExposure(ctx, roundID, teamID)
	if err != nil {
		return PortfolioSnapshot{}, err
	}
	equity := cash + grossExposure
	unrealized := equity - round.InitialBalanceCents
	maxDrawdown, err := s.maxDrawdownWithCurrent(ctx, roundID, teamID, equity)
	if err != nil {
		return PortfolioSnapshot{}, err
	}
	return PortfolioSnapshot{
		RoundID:            roundID,
		TeamID:             teamID,
		CashCents:          cash,
		EquityCents:        equity,
		RealizedPNLCents:   0,
		UnrealizedPNLCents: unrealized,
		GrossExposureCents: grossExposure,
		MaxDrawdownBPS:     maxDrawdown,
		CreatedAt:          Now(),
	}, nil
}

func (s *Store) maxDrawdownWithCurrent(ctx context.Context, roundID, teamID, currentEquity int64) (int64, error) {
	var peak, previousMax sql.NullInt64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(MAX(equity_cents), 0), COALESCE(MAX(max_drawdown_bps), 0)
		FROM portfolio_snapshots
		WHERE round_id = ? AND team_id = ?
	`, roundID, teamID).Scan(&peak, &previousMax); err != nil {
		return 0, err
	}
	if !peak.Valid || peak.Int64 < currentEquity {
		peak.Int64 = currentEquity
	}
	drawdown := int64(0)
	if peak.Int64 > 0 && currentEquity < peak.Int64 {
		drawdown = ((peak.Int64 - currentEquity) * 10000) / peak.Int64
	}
	if previousMax.Valid && previousMax.Int64 > drawdown {
		return previousMax.Int64, nil
	}
	return drawdown, nil
}

type portfolioScanner interface {
	Scan(dest ...interface{}) error
}

func scanPortfolio(row portfolioScanner) (PortfolioSnapshot, error) {
	var snapshot PortfolioSnapshot
	err := row.Scan(&snapshot.ID, &snapshot.RoundID, &snapshot.TeamID, &snapshot.CashCents, &snapshot.EquityCents, &snapshot.RealizedPNLCents, &snapshot.UnrealizedPNLCents, &snapshot.GrossExposureCents, &snapshot.MaxDrawdownBPS, &snapshot.CreatedAt)
	return snapshot, err
}
