package worker

import (
	"context"
	"testing"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/db"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

func TestTickAdvancesSimulatedMarketPrices(t *testing.T) {
	ctx := context.Background()
	conn, err := db.Open(ctx, t.TempDir()+"/arena.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	st := store.New(conn)
	team, err := st.CreateTeam(ctx, "team-a", "Team A", auth.HashToken("token"))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, store.RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, store.MarketInput{
		Venue:        "fake",
		ExternalID:   "demo-1",
		Slug:         "demo-1",
		Title:        "Demo",
		Category:     "bootcamp",
		Status:       "open",
		YesPriceBPS:  5100,
		NoPriceBPS:   4900,
		PricePathBPS: []int64{5100, 5400},
		FinalOutcome: "yes",
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, round.ID, market.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePortfolioSnapshot(ctx, round.ID, team.ID); err != nil {
		t.Fatal(err)
	}

	worker := &SnapshotWorker{Store: st}
	if err := worker.tick(ctx); err != nil {
		t.Fatal(err)
	}
	market, err = st.GetMarket(ctx, market.ID)
	if err != nil {
		t.Fatal(err)
	}
	if market.YesPriceBPS != 5400 || market.NoPriceBPS != 4600 {
		t.Fatalf("market price after tick = %d/%d, want 5400/4600", market.YesPriceBPS, market.NoPriceBPS)
	}
}
