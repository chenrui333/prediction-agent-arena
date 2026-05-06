package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
)

type MarketInput struct {
	Venue              string  `json:"venue"`
	ExternalID         string  `json:"external_id"`
	Slug               string  `json:"slug"`
	Title              string  `json:"title"`
	Category           string  `json:"category"`
	Status             string  `json:"status"`
	YesPriceBPS        int64   `json:"yes_price_bps"`
	NoPriceBPS         int64   `json:"no_price_bps"`
	MetadataJSON       string  `json:"metadata_json"`
	TrueProbabilityBPS *int64  `json:"true_probability_bps,omitempty"`
	PricePathBPS       []int64 `json:"price_path_bps,omitempty"`
	FinalOutcome       string  `json:"final_outcome,omitempty"`
}

type SimulatedMarketStateInput struct {
	MarketID           int64
	TrueProbabilityBPS *int64
	CurrentTick        int64
	PricePathBPS       []int64
	FinalOutcome       string
}

func (s *Store) UpsertMarket(ctx context.Context, input MarketInput) (Market, error) {
	if hasSimulatedMarketConfig(input) {
		if err := validateSimulatedMarketConfig(input.PricePathBPS, input.FinalOutcome, input.TrueProbabilityBPS); err != nil {
			return Market{}, err
		}
	}
	if input.Venue == "" {
		input.Venue = "fake"
	}
	if input.Status == "" {
		input.Status = "open"
	}
	if input.MetadataJSON == "" {
		input.MetadataJSON = "{}"
	}
	if input.NoPriceBPS == 0 && input.YesPriceBPS > 0 {
		input.NoPriceBPS = 10000 - input.YesPriceBPS
	}
	now := Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO markets(venue, external_id, slug, title, category, status, yes_price_bps, no_price_bps, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(venue, external_id) DO UPDATE SET
			slug = excluded.slug,
			title = excluded.title,
			category = excluded.category,
			status = excluded.status,
			yes_price_bps = excluded.yes_price_bps,
			no_price_bps = excluded.no_price_bps,
			metadata_json = excluded.metadata_json,
			updated_at = excluded.updated_at
	`, input.Venue, input.ExternalID, input.Slug, input.Title, input.Category, input.Status, input.YesPriceBPS, input.NoPriceBPS, input.MetadataJSON, now, now)
	if err != nil {
		return Market{}, err
	}
	market, err := s.GetMarketByExternalID(ctx, input.Venue, input.ExternalID)
	if err != nil {
		return Market{}, err
	}
	if hasSimulatedMarketConfig(input) {
		if _, err := s.UpsertSimulatedMarketState(ctx, SimulatedMarketStateInput{
			MarketID:           market.ID,
			TrueProbabilityBPS: input.TrueProbabilityBPS,
			PricePathBPS:       input.PricePathBPS,
			FinalOutcome:       input.FinalOutcome,
		}); err != nil {
			return Market{}, err
		}
		return s.GetMarket(ctx, market.ID)
	}
	return market, nil
}

func (s *Store) ListMarkets(ctx context.Context) ([]Market, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, venue, external_id, slug, title, category, status, yes_price_bps, no_price_bps, metadata_json, created_at, updated_at
		FROM markets
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMarkets(rows)
}

func (s *Store) ListRoundMarkets(ctx context.Context, roundID int64) ([]Market, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT m.id, m.venue, m.external_id, m.slug, m.title, m.category, m.status, m.yes_price_bps, m.no_price_bps, m.metadata_json, m.created_at, m.updated_at
		FROM markets m
		JOIN round_markets rm ON rm.market_id = m.id
		WHERE rm.round_id = ?
		ORDER BY m.id
	`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMarkets(rows)
}

func (s *Store) GetMarket(ctx context.Context, id int64) (Market, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, venue, external_id, slug, title, category, status, yes_price_bps, no_price_bps, metadata_json, created_at, updated_at
		FROM markets
		WHERE id = ?
	`, id)
	market, err := scanMarket(row)
	return market, normalizeErr(err)
}

func (tx *Tx) getMarket(ctx context.Context, id int64) (Market, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, venue, external_id, slug, title, category, status, yes_price_bps, no_price_bps, metadata_json, created_at, updated_at
		FROM markets
		WHERE id = ?
	`, id)
	return scanMarket(row)
}

func (s *Store) GetRoundMarket(ctx context.Context, roundID, marketID int64) (Market, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT m.id, m.venue, m.external_id, m.slug, m.title, m.category, m.status, m.yes_price_bps, m.no_price_bps, m.metadata_json, m.created_at, m.updated_at
		FROM markets m
		JOIN round_markets rm ON rm.market_id = m.id
		WHERE rm.round_id = ? AND m.id = ?
	`, roundID, marketID)
	market, err := scanMarket(row)
	return market, normalizeErr(err)
}

func (s *Store) GetMarketByExternalID(ctx context.Context, venue, externalID string) (Market, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, venue, external_id, slug, title, category, status, yes_price_bps, no_price_bps, metadata_json, created_at, updated_at
		FROM markets
		WHERE venue = ? AND external_id = ?
	`, venue, externalID)
	market, err := scanMarket(row)
	return market, normalizeErr(err)
}

func (s *Store) AddRoundMarket(ctx context.Context, roundID, marketID int64) error {
	_, err := s.db.ExecContext(ctx, "INSERT OR IGNORE INTO round_markets(round_id, market_id) VALUES (?, ?)", roundID, marketID)
	return err
}

func (s *Store) UpsertSimulatedMarketState(ctx context.Context, input SimulatedMarketStateInput) (SimulatedMarketState, error) {
	if input.MarketID <= 0 {
		return SimulatedMarketState{}, fmt.Errorf("%w: market_id must be positive", ErrValidation)
	}
	if err := validateSimulatedMarketConfig(input.PricePathBPS, input.FinalOutcome, input.TrueProbabilityBPS); err != nil {
		return SimulatedMarketState{}, err
	}
	finalOutcome := input.FinalOutcome
	if finalOutcome == "" {
		finalOutcome = "unknown"
	}
	pricePathJSON, err := json.Marshal(input.PricePathBPS)
	if err != nil {
		return SimulatedMarketState{}, fmt.Errorf("encode simulated price path: %w", err)
	}
	currentTick := input.CurrentTick
	if currentTick < 0 {
		currentTick = 0
	}
	if currentTick >= int64(len(input.PricePathBPS)) {
		currentTick = int64(len(input.PricePathBPS) - 1)
	}
	now := Now()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO simulated_market_states(market_id, true_probability_bps, current_tick, price_path_json, final_outcome, resolved_at, resolved_by, updated_at)
		VALUES (?, ?, ?, ?, ?, NULL, '', ?)
		ON CONFLICT(market_id) DO UPDATE SET
			true_probability_bps = excluded.true_probability_bps,
			current_tick = excluded.current_tick,
			price_path_json = excluded.price_path_json,
			final_outcome = excluded.final_outcome,
			resolved_at = NULL,
			resolved_by = '',
			updated_at = excluded.updated_at
	`, input.MarketID, ptrToNullInt64(input.TrueProbabilityBPS), currentTick, string(pricePathJSON), finalOutcome, now)
	if err != nil {
		return SimulatedMarketState{}, fmt.Errorf("upsert simulated market state: %w", err)
	}
	if err := s.setMarketPrice(ctx, input.MarketID, input.PricePathBPS[currentTick], "open"); err != nil {
		return SimulatedMarketState{}, err
	}
	if err := s.recordMarketPriceTick(ctx, input.MarketID, currentTick, input.PricePathBPS[currentTick], "seed"); err != nil {
		return SimulatedMarketState{}, err
	}
	return s.GetSimulatedMarketState(ctx, input.MarketID)
}

func (s *Store) GetSimulatedMarketState(ctx context.Context, marketID int64) (SimulatedMarketState, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT market_id, true_probability_bps, current_tick, price_path_json, final_outcome, resolved_at, resolved_by, updated_at
		FROM simulated_market_states
		WHERE market_id = ?
	`, marketID)
	state, err := scanSimulatedMarketState(row)
	return state, normalizeErr(err)
}

func (s *Store) AdvanceRoundSimulatedMarkets(ctx context.Context, roundID int64) ([]MarketPriceTick, error) {
	ticks := []MarketPriceTick{}
	err := s.WithTx(ctx, func(tx *Tx) error {
		rows, err := tx.tx.QueryContext(ctx, `
			SELECT sms.market_id, sms.current_tick, sms.price_path_json
			FROM simulated_market_states sms
			JOIN round_markets rm ON rm.market_id = sms.market_id
			JOIN markets m ON m.id = sms.market_id
			WHERE rm.round_id = ? AND sms.resolved_at IS NULL AND m.status = 'open'
			ORDER BY sms.market_id
		`, roundID)
		if err != nil {
			return fmt.Errorf("list simulated markets for round %d: %w", roundID, err)
		}
		defer rows.Close()
		type candidate struct {
			marketID      int64
			currentTick   int64
			pricePathJSON string
		}
		candidates := []candidate{}
		for rows.Next() {
			var item candidate
			if err := rows.Scan(&item.marketID, &item.currentTick, &item.pricePathJSON); err != nil {
				return err
			}
			candidates = append(candidates, item)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		for _, item := range candidates {
			var path []int64
			if err := json.Unmarshal([]byte(item.pricePathJSON), &path); err != nil {
				return fmt.Errorf("decode price path for market %d: %w", item.marketID, err)
			}
			if len(path) == 0 || item.currentTick >= int64(len(path)-1) {
				continue
			}
			nextTick := item.currentTick + 1
			yesPrice := path[nextTick]
			if err := validateBPS("price_path_bps", yesPrice); err != nil {
				return err
			}
			tick, err := tx.updateSimulatedMarketPrice(ctx, item.marketID, nextTick, yesPrice, "simulated")
			if err != nil {
				return err
			}
			ticks = append(ticks, tick)
		}
		return nil
	})
	return ticks, err
}

func (s *Store) ResolveSimulatedMarket(ctx context.Context, marketID int64, outcome, resolvedBy string) (SimulatedMarketState, error) {
	if err := validateOutcome(outcome, false); err != nil {
		return SimulatedMarketState{}, err
	}
	if resolvedBy == "" {
		resolvedBy = "admin"
	}
	var state SimulatedMarketState
	err := s.WithTx(ctx, func(tx *Tx) error {
		var err error
		state, err = tx.getSimulatedMarketState(ctx, marketID)
		if err != nil {
			return normalizeErr(err)
		}
		if state.ResolvedAt != "" {
			return fmt.Errorf("%w: market is already resolved", ErrValidation)
		}
		now := Now()
		if _, err := tx.tx.ExecContext(ctx, `
			UPDATE simulated_market_states
			SET final_outcome = ?, resolved_at = ?, resolved_by = ?, updated_at = ?
			WHERE market_id = ?
		`, outcome, now, resolvedBy, now, marketID); err != nil {
			return fmt.Errorf("resolve simulated market %d: %w", marketID, err)
		}
		yesPrice := terminalYesPrice(outcome)
		if _, err := tx.updateSimulatedMarketPrice(ctx, marketID, state.CurrentTick+1, yesPrice, "resolution"); err != nil {
			return err
		}
		if _, err := tx.tx.ExecContext(ctx, "UPDATE markets SET status = 'resolved', updated_at = ? WHERE id = ?", now, marketID); err != nil {
			return fmt.Errorf("mark market %d resolved: %w", marketID, err)
		}
		if _, err := tx.upsertMarketOutcome(ctx, marketID, outcome, resolvedBy); err != nil {
			return err
		}
		state, err = tx.getSimulatedMarketState(ctx, marketID)
		return err
	})
	return state, err
}

func (tx *Tx) updateSimulatedMarketPrice(ctx context.Context, marketID, tick, yesPriceBPS int64, source string) (MarketPriceTick, error) {
	if err := validateBPS("yes_price_bps", yesPriceBPS); err != nil && yesPriceBPS != 0 && yesPriceBPS != 10000 {
		return MarketPriceTick{}, err
	}
	noPriceBPS := 10000 - yesPriceBPS
	now := Now()
	if _, err := tx.tx.ExecContext(ctx, `
		UPDATE simulated_market_states
		SET current_tick = ?, updated_at = ?
		WHERE market_id = ?
	`, tick, now, marketID); err != nil {
		return MarketPriceTick{}, fmt.Errorf("advance simulated market %d: %w", marketID, err)
	}
	if _, err := tx.tx.ExecContext(ctx, `
		UPDATE markets
		SET yes_price_bps = ?, no_price_bps = ?, updated_at = ?
		WHERE id = ?
	`, yesPriceBPS, noPriceBPS, now, marketID); err != nil {
		return MarketPriceTick{}, fmt.Errorf("update market %d price: %w", marketID, err)
	}
	_, err := tx.tx.ExecContext(ctx, `
		INSERT INTO market_price_ticks(market_id, tick, yes_price_bps, no_price_bps, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(market_id, tick) DO UPDATE SET
			yes_price_bps = excluded.yes_price_bps,
			no_price_bps = excluded.no_price_bps,
			source = excluded.source,
			created_at = excluded.created_at
	`, marketID, tick, yesPriceBPS, noPriceBPS, source, now)
	if err != nil {
		return MarketPriceTick{}, fmt.Errorf("record market %d price tick: %w", marketID, err)
	}
	return tx.getMarketPriceTick(ctx, marketID, tick)
}

func (s *Store) setMarketPrice(ctx context.Context, marketID, yesPriceBPS int64, status string) error {
	if err := validateBPS("yes_price_bps", yesPriceBPS); err != nil {
		return err
	}
	now := Now()
	_, err := s.db.ExecContext(ctx, `
		UPDATE markets
		SET yes_price_bps = ?, no_price_bps = ?, status = ?, updated_at = ?
		WHERE id = ?
	`, yesPriceBPS, 10000-yesPriceBPS, status, now, marketID)
	if err != nil {
		return fmt.Errorf("set market %d simulated price: %w", marketID, err)
	}
	return nil
}

func (s *Store) recordMarketPriceTick(ctx context.Context, marketID, tick, yesPriceBPS int64, source string) error {
	if err := validateBPS("yes_price_bps", yesPriceBPS); err != nil {
		return err
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO market_price_ticks(market_id, tick, yes_price_bps, no_price_bps, source, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(market_id, tick) DO UPDATE SET
			yes_price_bps = excluded.yes_price_bps,
			no_price_bps = excluded.no_price_bps,
			source = excluded.source,
			created_at = excluded.created_at
	`, marketID, tick, yesPriceBPS, 10000-yesPriceBPS, source, Now())
	if err != nil {
		return fmt.Errorf("record market %d price tick: %w", marketID, err)
	}
	return nil
}

type marketRows interface {
	Next() bool
	Scan(dest ...interface{}) error
	Err() error
}

type marketScanner interface {
	Scan(dest ...interface{}) error
}

func scanMarkets(rows marketRows) ([]Market, error) {
	markets := []Market{}
	for rows.Next() {
		market, err := scanMarket(rows)
		if err != nil {
			return nil, err
		}
		markets = append(markets, market)
	}
	return markets, rows.Err()
}

func scanMarket(row marketScanner) (Market, error) {
	var market Market
	err := row.Scan(&market.ID, &market.Venue, &market.ExternalID, &market.Slug, &market.Title, &market.Category, &market.Status, &market.YesPriceBPS, &market.NoPriceBPS, &market.MetadataJSON, &market.CreatedAt, &market.UpdatedAt)
	return market, err
}

type simulatedMarketStateScanner interface {
	Scan(dest ...interface{}) error
}

func scanSimulatedMarketState(row simulatedMarketStateScanner) (SimulatedMarketState, error) {
	var state SimulatedMarketState
	var trueProbability sql.NullInt64
	var resolvedAt, resolvedBy sql.NullString
	err := row.Scan(&state.MarketID, &trueProbability, &state.CurrentTick, &state.PricePathJSON, &state.FinalOutcome, &resolvedAt, &resolvedBy, &state.UpdatedAt)
	state.TrueProbabilityBPS = nullInt64Ptr(trueProbability)
	state.ResolvedAt = nullString(resolvedAt)
	state.ResolvedBy = nullString(resolvedBy)
	return state, err
}

func (tx *Tx) getSimulatedMarketState(ctx context.Context, marketID int64) (SimulatedMarketState, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT market_id, true_probability_bps, current_tick, price_path_json, final_outcome, resolved_at, resolved_by, updated_at
		FROM simulated_market_states
		WHERE market_id = ?
	`, marketID)
	return scanSimulatedMarketState(row)
}

func (tx *Tx) getMarketPriceTick(ctx context.Context, marketID, tick int64) (MarketPriceTick, error) {
	row := tx.tx.QueryRowContext(ctx, `
		SELECT id, market_id, tick, yes_price_bps, no_price_bps, source, created_at
		FROM market_price_ticks
		WHERE market_id = ? AND tick = ?
	`, marketID, tick)
	return scanMarketPriceTick(row)
}

func scanMarketPriceTick(row marketScanner) (MarketPriceTick, error) {
	var tick MarketPriceTick
	err := row.Scan(&tick.ID, &tick.MarketID, &tick.Tick, &tick.YesPriceBPS, &tick.NoPriceBPS, &tick.Source, &tick.CreatedAt)
	return tick, err
}

func hasSimulatedMarketConfig(input MarketInput) bool {
	return len(input.PricePathBPS) > 0 || input.FinalOutcome != "" || input.TrueProbabilityBPS != nil
}

func validateSimulatedMarketConfig(pricePath []int64, finalOutcome string, trueProbability *int64) error {
	if len(pricePath) == 0 {
		return fmt.Errorf("%w: price_path_bps is required for simulated markets", ErrValidation)
	}
	for _, price := range pricePath {
		if err := validateBPS("price_path_bps", price); err != nil {
			return err
		}
	}
	if finalOutcome != "" && finalOutcome != "unknown" && finalOutcome != "yes" && finalOutcome != "no" {
		return fmt.Errorf("%w: final_outcome must be yes, no, or unknown", ErrValidation)
	}
	if trueProbability != nil {
		if err := validateBPS("true_probability_bps", *trueProbability); err != nil {
			return err
		}
	}
	return nil
}

func validateBPS(field string, value int64) error {
	if value < 1 || value > 9999 {
		return fmt.Errorf("%w: %s must be between 1 and 9999", ErrValidation, field)
	}
	return nil
}
