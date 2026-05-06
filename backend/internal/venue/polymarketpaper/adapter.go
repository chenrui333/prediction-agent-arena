package polymarketpaper

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

// Adapter is a v1 skeleton for wrapping agent-next/polymarket-paper-trader
// behind the same Venue interface used by the local fake venue. It is
// intentionally non-mutating until the arena MVP is complete.
type Adapter struct {
	config Config
}

type Config struct {
	Bin           string
	AccountPrefix string
	Timeout       time.Duration
	DataDir       string
}

func New(config Config) (*Adapter, error) {
	if config.Bin == "" {
		config.Bin = "pm-trader"
	}
	if config.AccountPrefix == "" {
		config.AccountPrefix = "arena"
	}
	if config.Timeout <= 0 {
		config.Timeout = 10 * time.Second
	}
	if config.DataDir == "" {
		config.DataDir = "./data/pm-trader"
	}
	if strings.Contains(config.AccountPrefix, "/") || strings.Contains(config.AccountPrefix, "..") {
		return nil, errors.New("polymarket paper account prefix must not contain path separators")
	}
	if _, err := exec.LookPath(config.Bin); err != nil {
		return nil, fmt.Errorf("polymarket paper binary %q not found: %w", config.Bin, err)
	}
	if err := os.MkdirAll(config.DataDir, 0o755); err != nil {
		return nil, fmt.Errorf("create polymarket paper data dir %s: %w", config.DataDir, err)
	}
	return &Adapter{config: config}, nil
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
