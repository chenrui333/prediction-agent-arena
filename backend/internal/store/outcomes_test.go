package store

import (
	"context"
	"testing"
)

func TestSetMarketOutcomeResolvesGenericMarket(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:        "fake",
		ExternalID:   "generic-1",
		Slug:         "generic-1",
		Title:        "Generic",
		Category:     "arena",
		Status:       "open",
		YesPriceBPS:  5200,
		NoPriceBPS:   4800,
		MetadataJSON: "{}",
	})
	if err != nil {
		t.Fatal(err)
	}
	outcome, err := st.SetMarketOutcome(ctx, market.ID, "yes", "test")
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Outcome != "yes" || outcome.ResolvedAt == "" || outcome.ResolvedBy != "test" {
		t.Fatalf("unexpected outcome: %#v", outcome)
	}
	market, err = st.GetMarket(ctx, market.ID)
	if err != nil {
		t.Fatal(err)
	}
	if market.Status != "resolved" || market.YesPriceBPS != 10000 || market.NoPriceBPS != 0 {
		t.Fatalf("unexpected resolved market: %#v", market)
	}
}
