package store

import (
	"context"
)

type MarketInput struct {
	Venue        string `json:"venue"`
	ExternalID   string `json:"external_id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	YesPriceBPS  int64  `json:"yes_price_bps"`
	NoPriceBPS   int64  `json:"no_price_bps"`
	MetadataJSON string `json:"metadata_json"`
}

func (s *Store) UpsertMarket(ctx context.Context, input MarketInput) (Market, error) {
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
	return s.GetMarketByExternalID(ctx, input.Venue, input.ExternalID)
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
