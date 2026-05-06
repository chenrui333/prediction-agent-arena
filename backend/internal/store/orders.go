package store

import (
	"context"
	"database/sql"
)

func (tx *Tx) CreateDecision(ctx context.Context, input DecisionInput) (Decision, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO decisions(round_id, team_id, market_id, observed_price_bps, estimated_probability_bps, edge_bps, action, outcome, amount_cents, confidence, reason, raw_payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, input.MarketID, input.ObservedPriceBPS, ptrToNullInt64(input.EstimatedProbabilityBPS), input.EdgeBPS, input.Action, input.Outcome, input.AmountCents, input.Confidence, input.Reason, input.RawPayloadJSON, now)
	if err != nil {
		return Decision{}, err
	}
	id, _ := res.LastInsertId()
	return tx.getDecision(ctx, id)
}

func (tx *Tx) CreateOrder(ctx context.Context, input OrderInput) (Order, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO orders(round_id, team_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, input.MarketID, input.VenueOrderID, input.Action, input.Outcome, input.AmountCents, input.LimitPriceBPS, input.Status, input.RejectionReason, now, now)
	if err != nil {
		return Order{}, err
	}
	id, _ := res.LastInsertId()
	return tx.getOrder(ctx, id)
}

func (tx *Tx) CreateFill(ctx context.Context, input FillInput) (Fill, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO fills(round_id, team_id, order_id, market_id, action, outcome, amount_cents, fill_price_bps, fee_cents, slippage_bps, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, input.OrderID, input.MarketID, input.Action, input.Outcome, input.AmountCents, input.FillPriceBPS, input.FeeCents, input.SlippageBPS, now)
	if err != nil {
		return Fill{}, err
	}
	if err := tx.applyFillToPosition(ctx, input); err != nil {
		return Fill{}, err
	}
	id, _ := res.LastInsertId()
	return tx.getFill(ctx, id)
}

func (tx *Tx) CreateRiskEvent(ctx context.Context, input RiskEvent) (RiskEvent, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO risk_events(round_id, team_id, order_id, type, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, ptrToNullInt64(input.OrderID), input.Type, input.Message, now)
	if err != nil {
		return RiskEvent{}, err
	}
	id, _ := res.LastInsertId()
	return tx.getRiskEvent(ctx, id)
}

func (tx *Tx) UpdateOrderStatus(ctx context.Context, orderID int64, status, rejectionReason string) (Order, error) {
	_, err := tx.tx.ExecContext(ctx, "UPDATE orders SET status = ?, rejection_reason = ?, updated_at = ? WHERE id = ?", status, rejectionReason, Now(), orderID)
	if err != nil {
		return Order{}, err
	}
	return tx.getOrder(ctx, orderID)
}

func (tx *Tx) applyFillToPosition(ctx context.Context, input FillInput) error {
	quantity := quantityForAmount(input.AmountCents, input.FillPriceBPS)
	if input.Action == "sell" {
		quantity = -quantity
	}
	now := Now()
	_, err := tx.tx.ExecContext(ctx, `
		INSERT INTO positions(round_id, team_id, market_id, outcome, quantity_cents, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(round_id, team_id, market_id, outcome) DO UPDATE SET
			quantity_cents = quantity_cents + excluded.quantity_cents,
			updated_at = excluded.updated_at
	`, input.RoundID, input.TeamID, input.MarketID, input.Outcome, quantity, now)
	return err
}

func (s *Store) GetOrder(ctx context.Context, id int64) (Order, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at
		FROM orders
		WHERE id = ?
	`, id)
	order, err := scanOrder(row)
	return order, normalizeErr(err)
}

func (tx *Tx) getOrder(ctx context.Context, id int64) (Order, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at
		FROM orders
		WHERE id = ?
	`, id)
	return scanOrder(row)
}

func (tx *Tx) getDecision(ctx context.Context, id int64) (Decision, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, market_id, observed_price_bps, estimated_probability_bps, edge_bps, action, outcome, amount_cents, confidence, reason, raw_payload_json, created_at
		FROM decisions
		WHERE id = ?
	`, id)
	return scanDecision(row)
}

func (tx *Tx) getFill(ctx context.Context, id int64) (Fill, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, order_id, market_id, action, outcome, amount_cents, fill_price_bps, fee_cents, slippage_bps, created_at
		FROM fills
		WHERE id = ?
	`, id)
	return scanFill(row)
}

func (tx *Tx) getRiskEvent(ctx context.Context, id int64) (RiskEvent, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, order_id, type, message, created_at
		FROM risk_events
		WHERE id = ?
	`, id)
	return scanRiskEvent(row)
}

func (s *Store) ListFills(ctx context.Context, roundID, teamID int64) ([]Fill, error) {
	return s.ListRecentFills(ctx, roundID, teamID, 100)
}

func (s *Store) ListRecentFills(ctx context.Context, roundID, teamID int64, limit int) ([]Fill, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, round_id, team_id, order_id, market_id, action, outcome, amount_cents, fill_price_bps, fee_cents, slippage_bps, created_at
		FROM fills
		WHERE round_id = ? AND team_id = ?
		ORDER BY id DESC
		LIMIT ?
	`, roundID, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	fills := []Fill{}
	for rows.Next() {
		fill, err := scanFill(rows)
		if err != nil {
			return nil, err
		}
		fills = append(fills, fill)
	}
	return fills, rows.Err()
}

func (s *Store) ListRecentDecisions(ctx context.Context, roundID, teamID int64, limit int) ([]Decision, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, round_id, team_id, market_id, observed_price_bps, estimated_probability_bps, edge_bps, action, outcome, amount_cents, confidence, reason, raw_payload_json, created_at
		FROM decisions
		WHERE round_id = ? AND team_id = ?
		ORDER BY id DESC
		LIMIT ?
	`, roundID, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	decisions := []Decision{}
	for rows.Next() {
		decision, err := scanDecision(rows)
		if err != nil {
			return nil, err
		}
		decisions = append(decisions, decision)
	}
	return decisions, rows.Err()
}

func (s *Store) ListRecentOrders(ctx context.Context, roundID, teamID int64, limit int) ([]Order, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, round_id, team_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at
		FROM orders
		WHERE round_id = ? AND team_id = ?
		ORDER BY id DESC
		LIMIT ?
	`, roundID, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := []Order{}
	for rows.Next() {
		order, err := scanOrder(rows)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *Store) ListRecentRiskEvents(ctx context.Context, roundID, teamID int64, limit int) ([]RiskEvent, error) {
	if limit <= 0 || limit > 100 {
		limit = 25
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, round_id, team_id, order_id, type, message, created_at
		FROM risk_events
		WHERE round_id = ? AND team_id = ?
		ORDER BY id DESC
		LIMIT ?
	`, roundID, teamID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	events := []RiskEvent{}
	for rows.Next() {
		event, err := scanRiskEvent(rows)
		if err != nil {
			return nil, err
		}
		events = append(events, event)
	}
	return events, rows.Err()
}

func (s *Store) CountOpenOrders(ctx context.Context, roundID, teamID int64) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM orders
		WHERE round_id = ? AND team_id = ? AND status IN ('submitted', 'open')
	`, roundID, teamID).Scan(&count)
	return count, err
}

func (s *Store) CountOrdersSince(ctx context.Context, roundID, teamID int64, since string) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM orders
		WHERE round_id = ? AND team_id = ? AND created_at >= ?
	`, roundID, teamID, since).Scan(&count)
	return count, err
}

func (s *Store) QuantityForOutcome(ctx context.Context, roundID, teamID, marketID int64, outcome string) (int64, error) {
	var quantity sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT quantity_cents FROM positions
		WHERE round_id = ? AND team_id = ? AND market_id = ? AND outcome = ?
	`, roundID, teamID, marketID, outcome).Scan(&quantity)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !quantity.Valid {
		return 0, nil
	}
	return quantity.Int64, nil
}

func (s *Store) MarketExposure(ctx context.Context, roundID, teamID, marketID int64) (int64, error) {
	var exposure sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT SUM(CASE p.outcome WHEN 'yes' THEN ABS(p.quantity_cents * m.yes_price_bps / 10000) ELSE ABS(p.quantity_cents * m.no_price_bps / 10000) END)
		FROM positions p
		JOIN markets m ON m.id = p.market_id
		WHERE p.round_id = ? AND p.team_id = ? AND p.market_id = ?
	`, roundID, teamID, marketID).Scan(&exposure)
	if err != nil {
		return 0, err
	}
	if !exposure.Valid {
		return 0, nil
	}
	return exposure.Int64, nil
}

func (s *Store) TotalExposure(ctx context.Context, roundID, teamID int64) (int64, error) {
	var exposure sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT SUM(CASE p.outcome WHEN 'yes' THEN ABS(p.quantity_cents * m.yes_price_bps / 10000) ELSE ABS(p.quantity_cents * m.no_price_bps / 10000) END)
		FROM positions p
		JOIN markets m ON m.id = p.market_id
		WHERE p.round_id = ? AND p.team_id = ?
	`, roundID, teamID).Scan(&exposure)
	if err != nil {
		return 0, err
	}
	if !exposure.Valid {
		return 0, nil
	}
	return exposure.Int64, nil
}

type orderScanner interface {
	Scan(dest ...interface{}) error
}

func scanOrder(row orderScanner) (Order, error) {
	var order Order
	err := row.Scan(&order.ID, &order.RoundID, &order.TeamID, &order.MarketID, &order.VenueOrderID, &order.Action, &order.Outcome, &order.AmountCents, &order.LimitPriceBPS, &order.Status, &order.RejectionReason, &order.CreatedAt, &order.UpdatedAt)
	return order, err
}

func scanDecision(row orderScanner) (Decision, error) {
	var decision Decision
	var estimated sql.NullInt64
	err := row.Scan(&decision.ID, &decision.RoundID, &decision.TeamID, &decision.MarketID, &decision.ObservedPriceBPS, &estimated, &decision.EdgeBPS, &decision.Action, &decision.Outcome, &decision.AmountCents, &decision.Confidence, &decision.Reason, &decision.RawPayloadJSON, &decision.CreatedAt)
	decision.EstimatedProbabilityBPS = nullInt64Ptr(estimated)
	return decision, err
}

func scanFill(row orderScanner) (Fill, error) {
	var fill Fill
	err := row.Scan(&fill.ID, &fill.RoundID, &fill.TeamID, &fill.OrderID, &fill.MarketID, &fill.Action, &fill.Outcome, &fill.AmountCents, &fill.FillPriceBPS, &fill.FeeCents, &fill.SlippageBPS, &fill.CreatedAt)
	return fill, err
}

func scanRiskEvent(row orderScanner) (RiskEvent, error) {
	var event RiskEvent
	var orderID sql.NullInt64
	err := row.Scan(&event.ID, &event.RoundID, &event.TeamID, &orderID, &event.Type, &event.Message, &event.CreatedAt)
	event.OrderID = nullInt64Ptr(orderID)
	return event, err
}
