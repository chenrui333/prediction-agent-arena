package store

import (
	"context"
	"testing"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
)

func TestAverageCostAccountingAndSettlement(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	team, err := st.CreateTeam(ctx, "team-a", "Team A", auth.HashToken("a"))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:        "fake",
		ExternalID:   "market-1",
		Slug:         "market-1",
		Title:        "Market 1",
		Category:     "arena",
		Status:       "open",
		YesPriceBPS:  6000,
		NoPriceBPS:   4000,
		PricePathBPS: []int64{6000, 7000},
		FinalOutcome: "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, round.ID, market.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.WithTx(ctx, func(tx *Tx) error {
		order, err := tx.CreateOrder(ctx, OrderInput{RoundID: round.ID, TeamID: team.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 6000, LimitPriceBPS: 6000, Status: "filled"})
		if err != nil {
			return err
		}
		if _, err := tx.CreateFill(ctx, FillInput{RoundID: round.ID, TeamID: team.ID, OrderID: order.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 6000, FillPriceBPS: 6000}); err != nil {
			return err
		}
		order, err = tx.CreateOrder(ctx, OrderInput{RoundID: round.ID, TeamID: team.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 7000, LimitPriceBPS: 7000, Status: "filled"})
		if err != nil {
			return err
		}
		_, err = tx.CreateFill(ctx, FillInput{RoundID: round.ID, TeamID: team.ID, OrderID: order.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 7000, FillPriceBPS: 7000})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	pnl, err := st.ListPerMarketPnL(ctx, round.ID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(pnl) != 1 || pnl[0].QuantityCents != 20000 || pnl[0].AvgEntryPriceBPS != 6500 {
		t.Fatalf("unexpected average-cost position: %#v", pnl)
	}
	if _, err := st.ResolveSimulatedMarket(ctx, market.ID, "yes", "test"); err != nil {
		t.Fatal(err)
	}
	settlements, err := st.SettleRound(ctx, round.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(settlements) != 1 {
		t.Fatalf("settlements = %#v", settlements)
	}
	if settlements[0].CashDeltaCents != 20000 || settlements[0].RealizedPNLCents != 7000 {
		t.Fatalf("unexpected settlement: %#v", settlements[0])
	}
	portfolio, err := st.ComputePortfolio(ctx, round.ID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if portfolio.CashCents != 1007000 || portfolio.GrossExposureCents != 0 || portfolio.RealizedPNLCents != 7000 {
		t.Fatalf("unexpected settled portfolio: %#v", portfolio)
	}
	again, err := st.SettleRound(ctx, round.ID, "test")
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("settlement should be idempotent, got %#v", again)
	}
}

func TestFillOpenOrdersUsesCurrentPricePath(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	team, err := st.CreateTeam(ctx, "team-a", "Team A", auth.HashToken("a"))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:        "fake",
		ExternalID:   "market-1",
		Slug:         "market-1",
		Title:        "Market 1",
		Category:     "arena",
		Status:       "open",
		YesPriceBPS:  5700,
		NoPriceBPS:   4300,
		PricePathBPS: []int64{5700, 5500, 5200},
		FinalOutcome: "no",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, round.ID, market.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.WithTx(ctx, func(tx *Tx) error {
		_, err := tx.CreateOrder(ctx, OrderInput{RoundID: round.ID, TeamID: team.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 10000, LimitPriceBPS: 5500, Status: "open"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	fills, err := st.FillOpenOrders(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 0 {
		t.Fatalf("order should not fill before price advances: %#v", fills)
	}
	if _, err := st.AdvanceRoundSimulatedMarkets(ctx, round.ID); err != nil {
		t.Fatal(err)
	}
	fills, err = st.FillOpenOrders(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 1 || fills[0].Fill.FillPriceBPS != 5500 || fills[0].Order.Status != "filled" {
		t.Fatalf("unexpected delayed fill: %#v", fills)
	}
}

func TestFillOpenOrdersSkipsUndispatchedIdempotentOrders(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	team, err := st.CreateTeam(ctx, "team-a", "Team A", auth.HashToken("a"))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:        "fake",
		ExternalID:   "market-1",
		Slug:         "market-1",
		Title:        "Market 1",
		Category:     "arena",
		Status:       "open",
		YesPriceBPS:  5700,
		NoPriceBPS:   4300,
		PricePathBPS: []int64{5700},
		FinalOutcome: "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, round.ID, market.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.WithTx(ctx, func(tx *Tx) error {
		_, err := tx.CreateOrder(ctx, OrderInput{RoundID: round.ID, TeamID: team.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 10000, LimitPriceBPS: 5700, Status: "submitted", ClientOrderID: "pending-1", RequestHash: "hash"})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	fills, err := st.FillOpenOrders(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(fills) != 0 {
		t.Fatalf("undispatched order should not fill: %#v", fills)
	}
}
