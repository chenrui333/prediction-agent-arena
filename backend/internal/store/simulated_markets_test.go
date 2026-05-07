package store

import (
	"context"
	"encoding/json"
	"testing"
)

func TestSimulatedMarketAdvancesDeterministicPricePath(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:              "fake",
		ExternalID:         "demo-1",
		Slug:               "demo-1",
		Title:              "Demo 1",
		Category:           "arena",
		Status:             "open",
		YesPriceBPS:        4100,
		NoPriceBPS:         5900,
		TrueProbabilityBPS: int64Ptr(5800),
		PricePathBPS:       []int64{4100, 4300, 4700},
		FinalOutcome:       "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, round.ID, market.ID); err != nil {
		t.Fatal(err)
	}
	state, err := st.GetSimulatedMarketState(ctx, market.ID)
	if err != nil {
		t.Fatal(err)
	}
	assertPricePath(t, state.PricePathJSON, []int64{4100, 4300, 4700})
	market, err = st.GetMarket(ctx, market.ID)
	if err != nil {
		t.Fatal(err)
	}
	if market.YesPriceBPS != 4100 || market.NoPriceBPS != 5900 {
		t.Fatalf("initial market price = %d/%d, want 4100/5900", market.YesPriceBPS, market.NoPriceBPS)
	}

	ticks, err := st.AdvanceRoundSimulatedMarkets(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 1 || ticks[0].Tick != 1 || ticks[0].YesPriceBPS != 4300 || ticks[0].NoPriceBPS != 5700 {
		t.Fatalf("unexpected first ticks: %#v", ticks)
	}
	ticks, err = st.AdvanceRoundSimulatedMarkets(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 1 || ticks[0].Tick != 2 || ticks[0].YesPriceBPS != 4700 || ticks[0].NoPriceBPS != 5300 {
		t.Fatalf("unexpected second ticks: %#v", ticks)
	}
	ticks, err = st.AdvanceRoundSimulatedMarkets(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(ticks) != 0 {
		t.Fatalf("expected no ticks after path end, got %#v", ticks)
	}
	market, err = st.GetMarket(ctx, market.ID)
	if err != nil {
		t.Fatal(err)
	}
	if market.YesPriceBPS != 4700 || market.NoPriceBPS != 5300 {
		t.Fatalf("last path market price = %d/%d, want 4700/5300", market.YesPriceBPS, market.NoPriceBPS)
	}
}

func TestResolveSimulatedMarketUpdatesOutcomeAndTerminalPrice(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:        "fake",
		ExternalID:   "demo-2",
		Slug:         "demo-2",
		Title:        "Demo 2",
		Category:     "arena",
		Status:       "open",
		YesPriceBPS:  6200,
		NoPriceBPS:   3800,
		PricePathBPS: []int64{6200, 6400},
		FinalOutcome: "unknown",
	})
	if err != nil {
		t.Fatal(err)
	}
	state, err := st.ResolveSimulatedMarket(ctx, market.ID, "no", "test")
	if err != nil {
		t.Fatal(err)
	}
	if state.FinalOutcome != "no" || state.ResolvedAt == "" || state.ResolvedBy != "test" {
		t.Fatalf("unexpected resolved state: %#v", state)
	}
	market, err = st.GetMarket(ctx, market.ID)
	if err != nil {
		t.Fatal(err)
	}
	if market.Status != "resolved" || market.YesPriceBPS != 0 || market.NoPriceBPS != 10000 {
		t.Fatalf("resolved market = status %q price %d/%d, want resolved 0/10000", market.Status, market.YesPriceBPS, market.NoPriceBPS)
	}
}

func assertPricePath(t *testing.T, raw string, want []int64) {
	t.Helper()
	var got []int64
	if err := json.Unmarshal([]byte(raw), &got); err != nil {
		t.Fatal(err)
	}
	if len(got) != len(want) {
		t.Fatalf("price path len = %d, want %d", len(got), len(want))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("price path[%d] = %d, want %d", i, got[i], want[i])
		}
	}
}

func int64Ptr(v int64) *int64 {
	return &v
}
