package store

import (
	"context"
	"os"
	"strings"
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
	if _, err := st.EnrollRoundTeam(ctx, RoundTeamInput{RoundID: round.ID, TeamID: teamA.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollRoundTeam(ctx, RoundTeamInput{RoundID: round.ID, TeamID: teamB.ID}); err != nil {
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

func TestLeaderboardFiltersRoundEligibilityBeforeRanking(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	eligibleHigh := createLeaderboardTeam(t, ctx, st, round.ID, "eligible-high", 80, "active", true)
	eligibleLow := createLeaderboardTeam(t, ctx, st, round.ID, "eligible-low", 55, "active", true)
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "not-enrolled", 99, "", true)
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "paused-enrollment", 98, "paused", true)
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "withdrawn-enrollment", 97, "withdrawn", true)
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "globally-inactive", 96, "active", false)

	rows, err := st.ListLeaderboard(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("rows = %#v, want only 2 eligible teams", rows)
	}
	if rows[0].TeamID != eligibleHigh.ID || rows[0].Rank != 1 {
		t.Fatalf("first row = %#v, want eligible-high rank 1", rows[0])
	}
	if rows[1].TeamID != eligibleLow.ID || rows[1].Rank != 2 {
		t.Fatalf("second row = %#v, want eligible-low rank 2", rows[1])
	}
}

func TestLeaderboardKeepsCompletedRoundHistoryForDisabledTeams(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	round, err := st.CreateRound(ctx, RoundInput{Slug: "completed-1", Name: "Completed", Status: "completed", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	team := createLeaderboardTeam(t, ctx, st, round.ID, "completed-team", 70, "active", false)

	rows, err := st.ListLeaderboard(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].TeamID != team.ID || rows[0].Rank != 1 {
		t.Fatalf("rows = %#v, want completed disabled team retained", rows)
	}
}

func TestExportRoundUsesEligibleLeaderboardRows(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	round, err := st.CreateRound(ctx, RoundInput{Slug: "export-1", Name: "Export", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "export-eligible", 75, "active", true)
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "export-withdrawn", 95, "withdrawn", true)
	_ = createLeaderboardTeam(t, ctx, st, round.ID, "export-not-enrolled", 90, "", true)

	result, err := st.ExportRound(ctx, round.ID, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(result.LeaderboardCSV)
	if err != nil {
		t.Fatal(err)
	}
	content := string(raw)
	if !strings.Contains(content, "export-eligible") {
		t.Fatalf("leaderboard export missing eligible team:\n%s", content)
	}
	if strings.Contains(content, "export-withdrawn") || strings.Contains(content, "export-not-enrolled") {
		t.Fatalf("leaderboard export contains ineligible team:\n%s", content)
	}
}

func createLeaderboardTeam(t *testing.T, ctx context.Context, st *Store, roundID int64, slug string, score int64, enrollmentStatus string, active bool) Team {
	t.Helper()
	team, err := st.CreateTeam(ctx, slug, slug, auth.HashToken(slug))
	if err != nil {
		t.Fatal(err)
	}
	if !active {
		team, err = st.SetTeamActive(ctx, team.ID, false)
		if err != nil {
			t.Fatal(err)
		}
	}
	if enrollmentStatus != "" {
		if _, err := st.EnrollRoundTeam(ctx, RoundTeamInput{RoundID: roundID, TeamID: team.ID, Status: enrollmentStatus}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreatePortfolioSnapshot(ctx, roundID, team.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateScoreSnapshot(ctx, roundID, team.ID, scoring.Snapshot{CompositeScore: score, EquityCents: 1000000 + score, BrierScoreBPS: 5000}); err != nil {
		t.Fatal(err)
	}
	return team
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
		Category:     "arena",
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
