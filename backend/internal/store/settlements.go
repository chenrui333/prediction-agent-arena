package store

import (
	"context"
	"fmt"
)

func (s *Store) SettleRound(ctx context.Context, roundID int64, settledBy string) ([]Settlement, error) {
	if settledBy == "" {
		settledBy = "admin"
	}
	_ = settledBy
	settlements := []Settlement{}
	err := s.WithTx(ctx, func(tx *Tx) error {
		rows, err := tx.tx.QueryContext(ctx, `
			SELECT
				p.round_id,
				p.team_id,
				p.market_id,
				p.outcome,
				p.quantity_cents,
				p.avg_entry_price_bps,
				mo.outcome
			FROM positions p
			JOIN round_markets rm ON rm.round_id = p.round_id AND rm.market_id = p.market_id
			JOIN market_outcomes mo ON mo.market_id = p.market_id
			LEFT JOIN settlements existing
				ON existing.round_id = p.round_id
				AND existing.team_id = p.team_id
				AND existing.market_id = p.market_id
				AND existing.outcome = p.outcome
			WHERE p.round_id = ?
				AND p.quantity_cents > 0
				AND mo.outcome IN ('yes', 'no')
				AND mo.resolved_at IS NOT NULL
				AND existing.id IS NULL
			ORDER BY p.team_id, p.market_id, p.outcome
		`, roundID)
		if err != nil {
			return err
		}
		defer rows.Close()
		type candidate struct {
			roundID         int64
			teamID          int64
			marketID        int64
			outcome         string
			quantityCents   int64
			avgEntryBPS     int64
			resolvedOutcome string
		}
		candidates := []candidate{}
		for rows.Next() {
			var item candidate
			if err := rows.Scan(&item.roundID, &item.teamID, &item.marketID, &item.outcome, &item.quantityCents, &item.avgEntryBPS, &item.resolvedOutcome); err != nil {
				return err
			}
			candidates = append(candidates, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, item := range candidates {
			price := int64(0)
			if item.outcome == item.resolvedOutcome {
				price = 10000
			}
			payout := markValue(item.quantityCents, price)
			realized := payout - markValue(item.quantityCents, item.avgEntryBPS)
			settlement, err := tx.createSettlement(ctx, Settlement{
				RoundID:            item.roundID,
				TeamID:             item.teamID,
				MarketID:           item.marketID,
				Outcome:            item.outcome,
				ResolvedOutcome:    item.resolvedOutcome,
				QuantityCents:      item.quantityCents,
				SettlementPriceBPS: price,
				CashDeltaCents:     payout,
				RealizedPNLCents:   realized,
			})
			if err != nil {
				return err
			}
			settlements = append(settlements, settlement)
			if _, err := tx.tx.ExecContext(ctx, `
				UPDATE positions
				SET quantity_cents = 0,
					avg_entry_price_bps = 0,
					realized_pnl_cents = realized_pnl_cents + ?,
					updated_at = ?
				WHERE round_id = ? AND team_id = ? AND market_id = ? AND outcome = ?
			`, realized, Now(), item.roundID, item.teamID, item.marketID, item.outcome); err != nil {
				return fmt.Errorf("settle position for team %d market %d %s: %w", item.teamID, item.marketID, item.outcome, err)
			}
		}
		return nil
	})
	return settlements, err
}

func (tx *Tx) createSettlement(ctx context.Context, input Settlement) (Settlement, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO settlements(round_id, team_id, market_id, outcome, resolved_outcome, quantity_cents, settlement_price_bps, cash_delta_cents, realized_pnl_cents, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, input.MarketID, input.Outcome, input.ResolvedOutcome, input.QuantityCents, input.SettlementPriceBPS, input.CashDeltaCents, input.RealizedPNLCents, now)
	if err != nil {
		return Settlement{}, err
	}
	id, _ := res.LastInsertId()
	return tx.getSettlement(ctx, id)
}

func (tx *Tx) getSettlement(ctx context.Context, id int64) (Settlement, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, market_id, outcome, resolved_outcome, quantity_cents, settlement_price_bps, cash_delta_cents, realized_pnl_cents, created_at
		FROM settlements
		WHERE id = ?
	`, id)
	return scanSettlement(row)
}

type settlementScanner interface {
	Scan(dest ...interface{}) error
}

func scanSettlement(row settlementScanner) (Settlement, error) {
	var settlement Settlement
	err := row.Scan(&settlement.ID, &settlement.RoundID, &settlement.TeamID, &settlement.MarketID, &settlement.Outcome, &settlement.ResolvedOutcome, &settlement.QuantityCents, &settlement.SettlementPriceBPS, &settlement.CashDeltaCents, &settlement.RealizedPNLCents, &settlement.CreatedAt)
	return settlement, normalizeErr(err)
}

func (s *Store) ListSettlements(ctx context.Context, roundID int64) ([]Settlement, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, round_id, team_id, market_id, outcome, resolved_outcome, quantity_cents, settlement_price_bps, cash_delta_cents, realized_pnl_cents, created_at
		FROM settlements
		WHERE round_id = ?
		ORDER BY id
	`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	settlements := []Settlement{}
	for rows.Next() {
		settlement, err := scanSettlement(rows)
		if err != nil {
			return nil, err
		}
		settlements = append(settlements, settlement)
	}
	return settlements, rows.Err()
}
