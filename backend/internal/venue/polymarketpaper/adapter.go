package polymarketpaper

import (
	"context"
	"errors"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

// Adapter is a v1 skeleton for wrapping agent-next/polymarket-paper-trader
// behind the same Venue interface used by the local fake venue. It is
// intentionally non-mutating until the bootcamp MVP is complete.
type Adapter struct{}

func New() *Adapter {
	return &Adapter{}
}

func (a *Adapter) ListMarkets(ctx context.Context) ([]venue.MarketSnapshot, error) {
	return nil, errNotImplemented()
}

func (a *Adapter) GetMarket(ctx context.Context, externalID string) (venue.MarketSnapshot, error) {
	return venue.MarketSnapshot{}, errNotImplemented()
}

func (a *Adapter) GetOrderBook(ctx context.Context, externalID string) (venue.OrderBook, error) {
	return venue.OrderBook{}, errNotImplemented()
}

func (a *Adapter) PlaceOrder(ctx context.Context, req venue.PlaceOrderRequest) (venue.PlaceOrderResult, error) {
	return venue.PlaceOrderResult{}, errNotImplemented()
}

func (a *Adapter) CancelOrder(ctx context.Context, venueOrderID string) error {
	return errNotImplemented()
}

func (a *Adapter) GetFills(ctx context.Context, teamSlug string, roundSlug string) ([]venue.FillSnapshot, error) {
	return nil, errNotImplemented()
}

func (a *Adapter) GetPortfolio(ctx context.Context, teamSlug string, roundSlug string) (venue.PortfolioSnapshot, error) {
	return venue.PortfolioSnapshot{}, errNotImplemented()
}

func errNotImplemented() error {
	return errors.New("polymarket paper venue adapter is a v1 skeleton")
}
