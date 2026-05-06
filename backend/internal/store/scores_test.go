package store

import (
	"context"
	"testing"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/db"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/scoring"
)

func TestLeaderboardReturnsScoreOrder(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	teamA, _ := st.CreateTeam(ctx, "team-a", "Team A", auth.HashToken("a"))
	teamB, _ := st.CreateTeam(ctx, "team-b", "Team B", auth.HashToken("b"))
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePortfolioSnapshot(ctx, round.ID, teamA.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreatePortfolioSnapshot(ctx, round.ID, teamB.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateScoreSnapshot(ctx, round.ID, teamA.ID, scoring.Snapshot{CompositeScore: 45, EquityCents: 990000, BrierScoreBPS: 5000}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateScoreSnapshot(ctx, round.ID, teamB.ID, scoring.Snapshot{CompositeScore: 80, EquityCents: 1010000, BrierScoreBPS: 5000}); err != nil {
		t.Fatal(err)
	}
	rows, err := st.ListLeaderboard(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 || rows[0].TeamSlug != "team-b" || rows[0].Rank != 1 {
		t.Fatalf("unexpected rows: %#v", rows)
	}
}

func TestScoreStatsComputesBrierFromResolvedDecisions(t *testing.T) {
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
	marketYes := createResolvedMarket(t, ctx, st, round.ID, "market-yes", "yes")
	marketNo := createResolvedMarket(t, ctx, st, round.ID, "market-no", "no")
	probYes := int64(8000)
	probNo := int64(3000)
	if err := st.WithTx(ctx, func(tx *Tx) error {
		if _, err := tx.CreateDecision(ctx, DecisionInput{
			RoundID:                 round.ID,
			TeamID:                  team.ID,
			MarketID:                marketYes.ID,
			ObservedPriceBPS:        5700,
			EstimatedProbabilityBPS: &probYes,
			EdgeBPS:                 2300,
			Action:                  "buy",
			Outcome:                 "yes",
			AmountCents:             1000,
			Confidence:              "medium",
			Reason:                  "calibrated yes",
			RawPayloadJSON:          "{}",
		}); err != nil {
			return err
		}
		if _, err := tx.CreateDecision(ctx, DecisionInput{
			RoundID:                 round.ID,
			TeamID:                  team.ID,
			MarketID:                marketNo.ID,
			ObservedPriceBPS:        5700,
			EstimatedProbabilityBPS: &probNo,
			EdgeBPS:                 -2700,
			Action:                  "buy",
			Outcome:                 "yes",
			AmountCents:             1000,
			Confidence:              "low",
			Reason:                  "wrong side",
			RawPayloadJSON:          "{}",
		}); err != nil {
			return err
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}

	stats, err := st.ScoreStats(ctx, round.ID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.BrierScoreBPS == nil {
		t.Fatal("expected brier score")
	}
	if *stats.BrierScoreBPS != 650 {
		t.Fatalf("brier = %d, want 650", *stats.BrierScoreBPS)
	}
	score, err := st.RefreshScore(ctx, round.ID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if score.BrierScoreBPS != 650 || score.CalibrationScore != 94 {
		t.Fatalf("unexpected score snapshot: %#v", score)
	}
}

func TestScoreStatsKeepsBrierNeutralWithoutResolvedOutcomes(t *testing.T) {
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
	if _, err := st.CreatePortfolioSnapshot(ctx, round.ID, team.ID); err != nil {
		t.Fatal(err)
	}
	stats, err := st.ScoreStats(ctx, round.ID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if stats.BrierScoreBPS != nil {
		t.Fatalf("brier = %d, want nil", *stats.BrierScoreBPS)
	}
}

func createResolvedMarket(t *testing.T, ctx context.Context, st *Store, roundID int64, slug, outcome string) Market {
	t.Helper()
	market, err := st.UpsertMarket(ctx, MarketInput{
		Venue:        "fake",
		ExternalID:   slug,
		Slug:         slug,
		Title:        slug,
		Category:     "bootcamp",
		Status:       "open",
		YesPriceBPS:  5700,
		NoPriceBPS:   4300,
		PricePathBPS: []int64{5700, 6100},
		FinalOutcome: outcome,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, roundID, market.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ResolveSimulatedMarket(ctx, market.ID, outcome, "test"); err != nil {
		t.Fatal(err)
	}
	return market
}

func newTestStore(t *testing.T, ctx context.Context) *Store {
	t.Helper()
	conn, err := db.Open(ctx, t.TempDir()+"/arena.db")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	if err := db.Migrate(ctx, conn); err != nil {
		t.Fatal(err)
	}
	return New(conn)
}
