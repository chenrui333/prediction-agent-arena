package venue

import "context"

type Venue interface {
	ListMarkets(ctx context.Context) ([]MarketSnapshot, error)
	GetMarket(ctx context.Context, externalID string) (MarketSnapshot, error)
	GetOrderBook(ctx context.Context, externalID string) (OrderBook, error)
	PlaceOrder(ctx context.Context, req PlaceOrderRequest) (PlaceOrderResult, error)
	CancelOrder(ctx context.Context, venueOrderID string) error
	GetFills(ctx context.Context, teamSlug string, roundSlug string) ([]FillSnapshot, error)
	GetPortfolio(ctx context.Context, teamSlug string, roundSlug string) (PortfolioSnapshot, error)
}

type MarketSnapshot struct {
	Venue       string `json:"venue"`
	ExternalID  string `json:"external_id"`
	Slug        string `json:"slug"`
	Title       string `json:"title"`
	Category    string `json:"category"`
	Status      string `json:"status"`
	YesPriceBPS int64  `json:"yes_price_bps"`
	NoPriceBPS  int64  `json:"no_price_bps"`
	Metadata    string `json:"metadata_json"`
}

type OrderBook struct {
	ExternalID string      `json:"external_id"`
	YesBids    []BookLevel `json:"yes_bids"`
	YesAsks    []BookLevel `json:"yes_asks"`
	NoBids     []BookLevel `json:"no_bids"`
	NoAsks     []BookLevel `json:"no_asks"`
}

type BookLevel struct {
	PriceBPS    int64 `json:"price_bps"`
	AmountCents int64 `json:"amount_cents"`
}

type PlaceOrderRequest struct {
	TeamSlug      string
	RoundSlug     string
	ClientOrderID string
	ExternalID    string
	Action        string
	Outcome       string
	AmountCents   int64
	LimitPriceBPS int64
}

type PlaceOrderResult struct {
	VenueOrderID string
	Filled       bool
	FillPriceBPS int64
	FeeCents     int64
	SlippageBPS  int64
	Status       string
}

type FillSnapshot struct {
	VenueOrderID string `json:"venue_order_id"`
	ExternalID   string `json:"external_id"`
	Action       string `json:"action"`
	Outcome      string `json:"outcome"`
	AmountCents  int64  `json:"amount_cents"`
	FillPriceBPS int64  `json:"fill_price_bps"`
	FeeCents     int64  `json:"fee_cents"`
	SlippageBPS  int64  `json:"slippage_bps"`
	CreatedAt    string `json:"created_at"`
}

type PortfolioSnapshot struct {
	CashCents          int64 `json:"cash_cents"`
	EquityCents        int64 `json:"equity_cents"`
	RealizedPNLCents   int64 `json:"realized_pnl_cents"`
	UnrealizedPNLCents int64 `json:"unrealized_pnl_cents"`
	GrossExposureCents int64 `json:"gross_exposure_cents"`
	MaxDrawdownBPS     int64 `json:"max_drawdown_bps"`
}
