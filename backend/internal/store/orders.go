package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (tx *Tx) CreateDecision(ctx context.Context, input DecisionInput) (Decision, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO decisions(round_id, team_id, agent_id, market_id, observed_price_bps, estimated_probability_bps, edge_bps, action, outcome, amount_cents, confidence, reason, raw_payload_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, ptrToNullInt64(input.AgentID), input.MarketID, input.ObservedPriceBPS, ptrToNullInt64(input.EstimatedProbabilityBPS), input.EdgeBPS, input.Action, input.Outcome, input.AmountCents, input.Confidence, input.Reason, input.RawPayloadJSON, now)
	if err != nil {
		return Decision{}, err
	}
	id, _ := res.LastInsertId()
	return tx.getDecision(ctx, id)
}

func (tx *Tx) CreateOrder(ctx context.Context, input OrderInput) (Order, error) {
	now := Now()
	res, err := tx.tx.ExecContext(ctx, `
		INSERT INTO orders(round_id, team_id, agent_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, ptrToNullInt64(input.AgentID), input.MarketID, input.VenueOrderID, input.Action, input.Outcome, input.AmountCents, input.LimitPriceBPS, input.Status, input.RejectionReason, now, now)
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
		INSERT INTO risk_events(round_id, team_id, agent_id, order_id, type, message, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, input.RoundID, input.TeamID, ptrToNullInt64(input.AgentID), ptrToNullInt64(input.OrderID), input.Type, input.Message, now)
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
	if quantity <= 0 {
		return fmt.Errorf("%w: fill quantity must be positive", ErrValidation)
	}
	now := Now()
	var existingQuantity, existingAvgEntry, existingRealized sql.NullInt64
	err := tx.tx.QueryRowContext(ctx, `
		SELECT quantity_cents, avg_entry_price_bps, realized_pnl_cents
		FROM positions
		WHERE round_id = ? AND team_id = ? AND market_id = ? AND outcome = ?
	`, input.RoundID, input.TeamID, input.MarketID, input.Outcome).Scan(&existingQuantity, &existingAvgEntry, &existingRealized)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	if err == sql.ErrNoRows {
		if input.Action == "sell" {
			return fmt.Errorf("%w: sell order exceeds available simulated position", ErrValidation)
		}
		_, err := tx.tx.ExecContext(ctx, `
			INSERT INTO positions(round_id, team_id, market_id, outcome, quantity_cents, avg_entry_price_bps, realized_pnl_cents, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, 0, ?)
		`, input.RoundID, input.TeamID, input.MarketID, input.Outcome, quantity, input.FillPriceBPS, now)
		return err
	}

	currentQuantity := existingQuantity.Int64
	currentAvgEntry := existingAvgEntry.Int64
	currentRealized := existingRealized.Int64
	switch input.Action {
	case "buy":
		newQuantity := currentQuantity + quantity
		newAvgEntry := input.FillPriceBPS
		if newQuantity > 0 {
			newAvgEntry = ((currentQuantity * currentAvgEntry) + (quantity * input.FillPriceBPS)) / newQuantity
		}
		_, err := tx.tx.ExecContext(ctx, `
			UPDATE positions
			SET quantity_cents = ?, avg_entry_price_bps = ?, updated_at = ?
			WHERE round_id = ? AND team_id = ? AND market_id = ? AND outcome = ?
		`, newQuantity, newAvgEntry, now, input.RoundID, input.TeamID, input.MarketID, input.Outcome)
		return err
	case "sell":
		if quantity > currentQuantity {
			return fmt.Errorf("%w: sell order exceeds available simulated position", ErrValidation)
		}
		newQuantity := currentQuantity - quantity
		newAvgEntry := currentAvgEntry
		if newQuantity == 0 {
			newAvgEntry = 0
		}
		proceeds := input.AmountCents - input.FeeCents
		realizedDelta := proceeds - markValue(quantity, currentAvgEntry)
		_, err := tx.tx.ExecContext(ctx, `
			UPDATE positions
			SET quantity_cents = ?, avg_entry_price_bps = ?, realized_pnl_cents = ?, updated_at = ?
			WHERE round_id = ? AND team_id = ? AND market_id = ? AND outcome = ?
		`, newQuantity, newAvgEntry, currentRealized+realizedDelta, now, input.RoundID, input.TeamID, input.MarketID, input.Outcome)
		return err
	default:
		return fmt.Errorf("%w: action must be buy or sell", ErrValidation)
	}
}

func (s *Store) GetOrder(ctx context.Context, id int64) (Order, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, agent_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at
		FROM orders
		WHERE id = ?
	`, id)
	order, err := scanOrder(row)
	return order, normalizeErr(err)
}

func (tx *Tx) getOrder(ctx context.Context, id int64) (Order, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, agent_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at
		FROM orders
		WHERE id = ?
	`, id)
	return scanOrder(row)
}

func (tx *Tx) getDecision(ctx context.Context, id int64) (Decision, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, agent_id, market_id, observed_price_bps, estimated_probability_bps, edge_bps, action, outcome, amount_cents, confidence, reason, raw_payload_json, created_at
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
		SELECT id, round_id, team_id, agent_id, order_id, type, message, created_at
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
		SELECT id, round_id, team_id, agent_id, market_id, observed_price_bps, estimated_probability_bps, edge_bps, action, outcome, amount_cents, confidence, reason, raw_payload_json, created_at
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
		SELECT id, round_id, team_id, agent_id, market_id, venue_order_id, action, outcome, amount_cents, limit_price_bps, status, rejection_reason, created_at, updated_at
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
		SELECT id, round_id, team_id, agent_id, order_id, type, message, created_at
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

func (s *Store) OpenBuyNotional(ctx context.Context, roundID, teamID int64) (int64, error) {
	var notional sql.NullInt64
	err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(amount_cents), 0)
		FROM orders
		WHERE round_id = ? AND team_id = ? AND action = 'buy' AND status IN ('submitted', 'open')
	`, roundID, teamID).Scan(&notional)
	if err != nil {
		return 0, err
	}
	return notional.Int64, nil
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

func (s *Store) FillOpenOrders(ctx context.Context, roundID int64) ([]OrderFillResult, error) {
	results := []OrderFillResult{}
	err := s.WithTx(ctx, func(tx *Tx) error {
		rows, err := tx.tx.QueryContext(ctx, `
			SELECT
				o.id, o.round_id, o.team_id, o.agent_id, o.market_id, o.venue_order_id, o.action, o.outcome,
				o.amount_cents, o.limit_price_bps, o.status, o.rejection_reason, o.created_at, o.updated_at,
				r.slug, t.slug, m.yes_price_bps, m.no_price_bps, m.status
			FROM orders o
			JOIN rounds r ON r.id = o.round_id
			JOIN teams t ON t.id = o.team_id
			JOIN markets m ON m.id = o.market_id
			WHERE o.round_id = ? AND o.status IN ('submitted', 'open')
			ORDER BY o.id
		`, roundID)
		if err != nil {
			return err
		}
		defer rows.Close()
		type candidate struct {
			order       Order
			roundSlug   string
			teamSlug    string
			yesPriceBPS int64
			noPriceBPS  int64
			marketState string
		}
		candidates := []candidate{}
		for rows.Next() {
			var item candidate
			var agentID sql.NullInt64
			if err := rows.Scan(&item.order.ID, &item.order.RoundID, &item.order.TeamID, &agentID, &item.order.MarketID, &item.order.VenueOrderID, &item.order.Action, &item.order.Outcome, &item.order.AmountCents, &item.order.LimitPriceBPS, &item.order.Status, &item.order.RejectionReason, &item.order.CreatedAt, &item.order.UpdatedAt, &item.roundSlug, &item.teamSlug, &item.yesPriceBPS, &item.noPriceBPS, &item.marketState); err != nil {
				return err
			}
			item.order.AgentID = nullInt64Ptr(agentID)
			candidates = append(candidates, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, item := range candidates {
			if item.marketState != "open" {
				continue
			}
			fillPrice := item.yesPriceBPS
			if item.order.Outcome == "no" {
				fillPrice = item.noPriceBPS
			}
			fillable := false
			switch item.order.Action {
			case "buy":
				fillable = item.order.LimitPriceBPS >= fillPrice
			case "sell":
				bid := fillPrice - 50
				if bid < 1 {
					bid = 1
				}
				fillable = item.order.LimitPriceBPS <= bid
				fillPrice = bid
			}
			if !fillable {
				continue
			}
			if item.order.Action == "sell" {
				quantity, err := tx.quantityForOutcome(ctx, item.order.RoundID, item.order.TeamID, item.order.MarketID, item.order.Outcome)
				if err != nil {
					return err
				}
				if quantityForAmount(item.order.AmountCents, fillPrice) > quantity {
					if _, err := tx.UpdateOrderStatus(ctx, item.order.ID, "failed", "insufficient_position"); err != nil {
						return err
					}
					continue
				}
			}
			updated, err := tx.UpdateOrderStatus(ctx, item.order.ID, "filled", "")
			if err != nil {
				return err
			}
			fill, err := tx.CreateFill(ctx, FillInput{
				RoundID:      item.order.RoundID,
				TeamID:       item.order.TeamID,
				OrderID:      item.order.ID,
				MarketID:     item.order.MarketID,
				Action:       item.order.Action,
				Outcome:      item.order.Outcome,
				AmountCents:  item.order.AmountCents,
				FillPriceBPS: fillPrice,
				FeeCents:     0,
				SlippageBPS:  absInt64(item.order.LimitPriceBPS - fillPrice),
			})
			if err != nil {
				return err
			}
			results = append(results, OrderFillResult{RoundSlug: item.roundSlug, TeamSlug: item.teamSlug, Order: updated, Fill: fill})
		}
		return nil
	})
	return results, err
}

func (tx *Tx) quantityForOutcome(ctx context.Context, roundID, teamID, marketID int64, outcome string) (int64, error) {
	var quantity sql.NullInt64
	err := tx.tx.QueryRowContext(ctx, `
		SELECT quantity_cents FROM positions
		WHERE round_id = ? AND team_id = ? AND market_id = ? AND outcome = ?
	`, roundID, teamID, marketID, outcome).Scan(&quantity)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	return quantity.Int64, nil
}

func absInt64(value int64) int64 {
	if value < 0 {
		return -value
	}
	return value
}

type orderScanner interface {
	Scan(dest ...interface{}) error
}

func scanOrder(row orderScanner) (Order, error) {
	var order Order
	var agentID sql.NullInt64
	err := row.Scan(&order.ID, &order.RoundID, &order.TeamID, &agentID, &order.MarketID, &order.VenueOrderID, &order.Action, &order.Outcome, &order.AmountCents, &order.LimitPriceBPS, &order.Status, &order.RejectionReason, &order.CreatedAt, &order.UpdatedAt)
	order.AgentID = nullInt64Ptr(agentID)
	return order, err
}

func scanDecision(row orderScanner) (Decision, error) {
	var decision Decision
	var agentID sql.NullInt64
	var estimated sql.NullInt64
	err := row.Scan(&decision.ID, &decision.RoundID, &decision.TeamID, &agentID, &decision.MarketID, &decision.ObservedPriceBPS, &estimated, &decision.EdgeBPS, &decision.Action, &decision.Outcome, &decision.AmountCents, &decision.Confidence, &decision.Reason, &decision.RawPayloadJSON, &decision.CreatedAt)
	decision.AgentID = nullInt64Ptr(agentID)
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
	var agentID sql.NullInt64
	var orderID sql.NullInt64
	err := row.Scan(&event.ID, &event.RoundID, &event.TeamID, &agentID, &orderID, &event.Type, &event.Message, &event.CreatedAt)
	event.AgentID = nullInt64Ptr(agentID)
	event.OrderID = nullInt64Ptr(orderID)
	return event, err
}
