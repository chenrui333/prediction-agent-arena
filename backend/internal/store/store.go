package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

var (
	ErrNotFound   = errors.New("not found")
	ErrValidation = errors.New("validation error")
)

type Store struct {
	db *sql.DB
}

type Tx struct {
	tx *sql.Tx
}

func New(db *sql.DB) *Store {
	return &Store{db: db}
}

func (s *Store) DB() *sql.DB {
	return s.db
}

func (s *Store) WithTx(ctx context.Context, fn func(*Tx) error) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	if err := fn(&Tx{tx: tx}); err != nil {
		if rollbackErr := tx.Rollback(); rollbackErr != nil {
			return fmt.Errorf("rollback after transaction failure: %v: %w", rollbackErr, err)
		}
		return fmt.Errorf("transaction failed: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}
	return nil
}

func Now() string {
	return time.Now().UTC().Format(time.RFC3339Nano)
}

type Team struct {
	ID           int64  `json:"id"`
	Slug         string `json:"slug"`
	Name         string `json:"name"`
	APITokenHash string `json:"-"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type Round struct {
	ID                  int64  `json:"id"`
	Slug                string `json:"slug"`
	Name                string `json:"name"`
	Mode                string `json:"mode"`
	Status              string `json:"status"`
	InitialBalanceCents int64  `json:"initial_balance_cents"`
	StartsAt            string `json:"starts_at,omitempty"`
	EndsAt              string `json:"ends_at,omitempty"`
	CreatedAt           string `json:"created_at"`
	UpdatedAt           string `json:"updated_at"`
}

type Market struct {
	ID           int64  `json:"id"`
	Venue        string `json:"venue"`
	ExternalID   string `json:"external_id"`
	Slug         string `json:"slug"`
	Title        string `json:"title"`
	Category     string `json:"category"`
	Status       string `json:"status"`
	YesPriceBPS  int64  `json:"yes_price_bps"`
	NoPriceBPS   int64  `json:"no_price_bps"`
	MetadataJSON string `json:"metadata_json"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

type PortfolioSnapshot struct {
	ID                 int64  `json:"id"`
	RoundID            int64  `json:"round_id"`
	TeamID             int64  `json:"team_id"`
	CashCents          int64  `json:"cash_cents"`
	EquityCents        int64  `json:"equity_cents"`
	RealizedPNLCents   int64  `json:"realized_pnl_cents"`
	UnrealizedPNLCents int64  `json:"unrealized_pnl_cents"`
	GrossExposureCents int64  `json:"gross_exposure_cents"`
	MaxDrawdownBPS     int64  `json:"max_drawdown_bps"`
	CreatedAt          string `json:"created_at"`
}

type Decision struct {
	ID                      int64  `json:"id"`
	RoundID                 int64  `json:"round_id"`
	TeamID                  int64  `json:"team_id"`
	MarketID                int64  `json:"market_id"`
	ObservedPriceBPS        int64  `json:"observed_price_bps"`
	EstimatedProbabilityBPS *int64 `json:"estimated_probability_bps,omitempty"`
	EdgeBPS                 int64  `json:"edge_bps"`
	Action                  string `json:"action"`
	Outcome                 string `json:"outcome"`
	AmountCents             int64  `json:"amount_cents"`
	Confidence              string `json:"confidence"`
	Reason                  string `json:"reason"`
	RawPayloadJSON          string `json:"raw_payload_json"`
	CreatedAt               string `json:"created_at"`
}

type Order struct {
	ID              int64  `json:"id"`
	RoundID         int64  `json:"round_id"`
	TeamID          int64  `json:"team_id"`
	MarketID        int64  `json:"market_id"`
	VenueOrderID    string `json:"venue_order_id"`
	Action          string `json:"action"`
	Outcome         string `json:"outcome"`
	AmountCents     int64  `json:"amount_cents"`
	LimitPriceBPS   int64  `json:"limit_price_bps"`
	Status          string `json:"status"`
	RejectionReason string `json:"rejection_reason,omitempty"`
	CreatedAt       string `json:"created_at"`
	UpdatedAt       string `json:"updated_at"`
}

type Fill struct {
	ID           int64  `json:"id"`
	RoundID      int64  `json:"round_id"`
	TeamID       int64  `json:"team_id"`
	OrderID      int64  `json:"order_id"`
	MarketID     int64  `json:"market_id"`
	Action       string `json:"action"`
	Outcome      string `json:"outcome"`
	AmountCents  int64  `json:"amount_cents"`
	FillPriceBPS int64  `json:"fill_price_bps"`
	FeeCents     int64  `json:"fee_cents"`
	SlippageBPS  int64  `json:"slippage_bps"`
	CreatedAt    string `json:"created_at"`
}

type RiskEvent struct {
	ID        int64  `json:"id"`
	RoundID   int64  `json:"round_id"`
	TeamID    int64  `json:"team_id"`
	OrderID   *int64 `json:"order_id,omitempty"`
	Type      string `json:"type"`
	Message   string `json:"message"`
	CreatedAt string `json:"created_at"`
}

type ScoreSnapshot struct {
	ID               int64  `json:"id"`
	RoundID          int64  `json:"round_id"`
	TeamID           int64  `json:"team_id"`
	CompositeScore   int64  `json:"composite_score"`
	ReturnScore      int64  `json:"return_score"`
	RiskScore        int64  `json:"risk_score"`
	CalibrationScore int64  `json:"calibration_score"`
	ExecutionScore   int64  `json:"execution_score"`
	CostScore        int64  `json:"cost_score"`
	EquityCents      int64  `json:"equity_cents"`
	ReturnBPS        int64  `json:"return_bps"`
	MaxDrawdownBPS   int64  `json:"max_drawdown_bps"`
	BrierScoreBPS    int64  `json:"brier_score_bps"`
	TradeCount       int64  `json:"trade_count"`
	CreatedAt        string `json:"created_at"`
}

type AgentHeartbeat struct {
	ID           int64  `json:"id"`
	RoundID      int64  `json:"round_id"`
	TeamID       int64  `json:"team_id"`
	Status       string `json:"status"`
	MetadataJSON string `json:"metadata_json"`
	CreatedAt    string `json:"created_at"`
}

type AdminTeamStats struct {
	TeamID             int64  `json:"team_id"`
	TeamSlug           string `json:"team_slug"`
	TeamName           string `json:"team_name"`
	IsActive           bool   `json:"is_active"`
	Status             string `json:"status"`
	LastHeartbeat      string `json:"last_heartbeat,omitempty"`
	EquityCents        int64  `json:"equity_cents"`
	TradeCount         int64  `json:"trade_count"`
	RiskRejectionCount int64  `json:"risk_rejection_count"`
	GrossExposureCents int64  `json:"gross_exposure_cents"`
}

type LeaderboardRow struct {
	Rank               int64  `json:"rank"`
	TeamID             int64  `json:"team_id"`
	TeamSlug           string `json:"team_slug"`
	TeamName           string `json:"team_name"`
	CompositeScore     int64  `json:"composite_score"`
	EquityCents        int64  `json:"equity_cents"`
	ReturnBPS          int64  `json:"return_bps"`
	MaxDrawdownBPS     int64  `json:"max_drawdown_bps"`
	BrierScoreBPS      int64  `json:"brier_score_bps"`
	TradeCount         int64  `json:"trade_count"`
	GrossExposureCents int64  `json:"gross_exposure_cents"`
	LastHeartbeat      string `json:"last_heartbeat,omitempty"`
	Status             string `json:"status"`
}

type DecisionInput struct {
	RoundID                 int64
	TeamID                  int64
	MarketID                int64
	ObservedPriceBPS        int64
	EstimatedProbabilityBPS *int64
	EdgeBPS                 int64
	Action                  string
	Outcome                 string
	AmountCents             int64
	Confidence              string
	Reason                  string
	RawPayloadJSON          string
}

type OrderInput struct {
	RoundID         int64
	TeamID          int64
	MarketID        int64
	VenueOrderID    string
	Action          string
	Outcome         string
	AmountCents     int64
	LimitPriceBPS   int64
	Status          string
	RejectionReason string
}

type FillInput struct {
	RoundID      int64
	TeamID       int64
	OrderID      int64
	MarketID     int64
	Action       string
	Outcome      string
	AmountCents  int64
	FillPriceBPS int64
	FeeCents     int64
	SlippageBPS  int64
}

func boolInt(v bool) int {
	if v {
		return 1
	}
	return 0
}

func scanBool(v int64) bool {
	return v != 0
}

func normalizeErr(err error) error {
	if errors.Is(err, sql.ErrNoRows) {
		return ErrNotFound
	}
	return err
}

func quantityForAmount(amountCents, priceBPS int64) int64 {
	if priceBPS <= 0 {
		return 0
	}
	return (amountCents * 10000) / priceBPS
}

func markValue(quantityCents, priceBPS int64) int64 {
	return (quantityCents * priceBPS) / 10000
}

func nullString(value sql.NullString) string {
	if value.Valid {
		return value.String
	}
	return ""
}

func nullInt64Ptr(value sql.NullInt64) *int64 {
	if value.Valid {
		v := value.Int64
		return &v
	}
	return nil
}

func ptrToNullInt64(value *int64) sql.NullInt64 {
	if value == nil {
		return sql.NullInt64{}
	}
	return sql.NullInt64{Valid: true, Int64: *value}
}

func validateSlug(slug string) error {
	if slug == "" {
		return fmt.Errorf("%w: slug is required", ErrValidation)
	}
	return nil
}
