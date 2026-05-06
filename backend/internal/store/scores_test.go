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
