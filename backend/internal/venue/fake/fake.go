package fake

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

type Venue struct {
	nextID uint64
}

func New() *Venue {
	return &Venue{}
}

func (v *Venue) ListMarkets(ctx context.Context) ([]venue.MarketSnapshot, error) {
	return []venue.MarketSnapshot{
		{Venue: "fake", ExternalID: "bootcamp-demo-1", Slug: "ai-tool-usage-above-60", Title: "Will bootcamp agents average more than 60 percent tool-use accuracy?", Category: "bootcamp", Status: "open", YesPriceBPS: 5700, NoPriceBPS: 4300, Metadata: "{}"},
		{Venue: "fake", ExternalID: "bootcamp-demo-2", Slug: "leaderboard-return-positive", Title: "Will at least five teams finish practice round with positive return?", Category: "bootcamp", Status: "open", YesPriceBPS: 5100, NoPriceBPS: 4900, Metadata: "{}"},
		{Venue: "fake", ExternalID: "bootcamp-demo-3", Slug: "risk-rejections-under-20", Title: "Will total rejected orders stay under 20 by round end?", Category: "bootcamp", Status: "open", YesPriceBPS: 6300, NoPriceBPS: 3700, Metadata: "{}"},
		{Venue: "fake", ExternalID: "bootcamp-demo-4", Slug: "final-demo-on-time", Title: "Will every team submit a final demo before the deadline?", Category: "bootcamp", Status: "open", YesPriceBPS: 6900, NoPriceBPS: 3100, Metadata: "{}"},
	}, nil
}

func (v *Venue) GetMarket(ctx context.Context, externalID string) (venue.MarketSnapshot, error) {
	markets, _ := v.ListMarkets(ctx)
	for _, market := range markets {
		if market.ExternalID == externalID {
			return market, nil
		}
	}
	return venue.MarketSnapshot{}, errors.New("market not found")
}

func (v *Venue) GetOrderBook(ctx context.Context, externalID string) (venue.OrderBook, error) {
	market, err := v.GetMarket(ctx, externalID)
	if err != nil {
		return venue.OrderBook{}, err
	}
	return venue.OrderBook{
		ExternalID: externalID,
		YesBids:    []venue.BookLevel{{PriceBPS: market.YesPriceBPS - 50, AmountCents: 250000}},
		YesAsks:    []venue.BookLevel{{PriceBPS: market.YesPriceBPS, AmountCents: 250000}},
		NoBids:     []venue.BookLevel{{PriceBPS: market.NoPriceBPS - 50, AmountCents: 250000}},
		NoAsks:     []venue.BookLevel{{PriceBPS: market.NoPriceBPS, AmountCents: 250000}},
	}, nil
}

func (v *Venue) PlaceOrder(ctx context.Context, req venue.PlaceOrderRequest) (venue.PlaceOrderResult, error) {
	market, err := v.GetMarket(ctx, req.ExternalID)
	if err != nil {
		return venue.PlaceOrderResult{}, err
	}
	price := market.YesPriceBPS
	if req.Outcome == "no" {
		price = market.NoPriceBPS
	}
	if req.Action == "buy" && req.LimitPriceBPS < price {
		return venue.PlaceOrderResult{VenueOrderID: v.nextVenueOrderID(), Filled: false, Status: "open"}, nil
	}
	if req.Action == "sell" && req.LimitPriceBPS > price {
		return venue.PlaceOrderResult{VenueOrderID: v.nextVenueOrderID(), Filled: false, Status: "open"}, nil
	}
	return venue.PlaceOrderResult{
		VenueOrderID: v.nextVenueOrderID(),
		Filled:       true,
		FillPriceBPS: price,
		FeeCents:     0,
		SlippageBPS:  abs(req.LimitPriceBPS - price),
		Status:       "filled",
	}, nil
}

func (v *Venue) CancelOrder(ctx context.Context, venueOrderID string) error {
	if venueOrderID == "" {
		return errors.New("venue order id is required")
	}
	return nil
}

func (v *Venue) GetFills(ctx context.Context, teamSlug string, roundSlug string) ([]venue.FillSnapshot, error) {
	return []venue.FillSnapshot{}, nil
}

func (v *Venue) GetPortfolio(ctx context.Context, teamSlug string, roundSlug string) (venue.PortfolioSnapshot, error) {
	return venue.PortfolioSnapshot{}, nil
}

func (v *Venue) nextVenueOrderID() string {
	id := atomic.AddUint64(&v.nextID, 1)
	return fmt.Sprintf("fake-%08d", id)
}

func abs(v int64) int64 {
	if v < 0 {
		return -v
	}
	return v
}
