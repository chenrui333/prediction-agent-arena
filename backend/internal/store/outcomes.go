package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) SetMarketOutcome(ctx context.Context, marketID int64, outcome, resolvedBy string) (MarketOutcome, error) {
	if marketID <= 0 {
		return MarketOutcome{}, fmt.Errorf("%w: market_id must be positive", ErrValidation)
	}
	if err := validateOutcome(outcome, false); err != nil {
		return MarketOutcome{}, err
	}
	if resolvedBy == "" {
		resolvedBy = "admin"
	}
	var result MarketOutcome
	err := s.WithTx(ctx, func(tx *Tx) error {
		if _, err := tx.getMarket(ctx, marketID); err != nil {
			return normalizeErr(err)
		}
		var err error
		result, err = tx.upsertMarketOutcome(ctx, marketID, outcome, resolvedBy)
		if err != nil {
			return err
		}
		yesPrice := terminalYesPrice(outcome)
		if _, err := tx.tx.ExecContext(ctx, `
			UPDATE markets
			SET status = 'resolved', yes_price_bps = ?, no_price_bps = ?, updated_at = ?
			WHERE id = ?
		`, yesPrice, 10000-yesPrice, Now(), marketID); err != nil {
			return fmt.Errorf("mark market %d resolved: %w", marketID, err)
		}
		return nil
	})
	return result, err
}

func (s *Store) GetMarketOutcome(ctx context.Context, marketID int64) (MarketOutcome, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT market_id, outcome, resolved_at, resolved_by, created_at, updated_at
		FROM market_outcomes
		WHERE market_id = ?
	`, marketID)
	outcome, err := scanMarketOutcome(row)
	return outcome, normalizeErr(err)
}

func (tx *Tx) upsertMarketOutcome(ctx context.Context, marketID int64, outcome, resolvedBy string) (MarketOutcome, error) {
	if err := validateOutcome(outcome, false); err != nil {
		return MarketOutcome{}, err
	}
	if resolvedBy == "" {
		resolvedBy = "system"
	}
	now := Now()
	_, err := tx.tx.ExecContext(ctx, `
		INSERT INTO market_outcomes(market_id, outcome, resolved_at, resolved_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(market_id) DO UPDATE SET
			outcome = excluded.outcome,
			resolved_at = excluded.resolved_at,
			resolved_by = excluded.resolved_by,
			updated_at = excluded.updated_at
	`, marketID, outcome, now, resolvedBy, now, now)
	if err != nil {
		return MarketOutcome{}, fmt.Errorf("upsert market %d outcome: %w", marketID, err)
	}
	return tx.getMarketOutcome(ctx, marketID)
}

func (tx *Tx) getMarketOutcome(ctx context.Context, marketID int64) (MarketOutcome, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT market_id, outcome, resolved_at, resolved_by, created_at, updated_at
		FROM market_outcomes
		WHERE market_id = ?
	`, marketID)
	return scanMarketOutcome(row)
}

type marketOutcomeScanner interface {
	Scan(dest ...interface{}) error
}

func scanMarketOutcome(row marketOutcomeScanner) (MarketOutcome, error) {
	var outcome MarketOutcome
	var resolvedAt, resolvedBy sql.NullString
	err := row.Scan(&outcome.MarketID, &outcome.Outcome, &resolvedAt, &resolvedBy, &outcome.CreatedAt, &outcome.UpdatedAt)
	outcome.ResolvedAt = nullString(resolvedAt)
	outcome.ResolvedBy = nullString(resolvedBy)
	return outcome, err
}

func validateOutcome(outcome string, allowUnknown bool) error {
	switch outcome {
	case "yes", "no":
		return nil
	case "unknown":
		if allowUnknown {
			return nil
		}
	}
	return fmt.Errorf("%w: outcome must be yes or no", ErrValidation)
}

func terminalYesPrice(outcome string) int64 {
	if outcome == "yes" {
		return 10000
	}
	return 0
}
