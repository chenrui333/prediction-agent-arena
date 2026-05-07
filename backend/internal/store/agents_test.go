package store

import (
	"context"
	"testing"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
)

func TestAgentTokenLookupAndLifecycleOrderPath(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	team, err := st.CreateTeam(ctx, "team-agent", "Team Agent", auth.HashToken("team-token"))
	if err != nil {
		t.Fatal(err)
	}
	agentToken := "paa_agent_lifecycle"
	agent, err := st.CreateAgent(ctx, AgentInput{TeamID: team.ID, Slug: "submitted", Name: "Submitted Agent"}, auth.HashToken(agentToken))
	if err != nil {
		t.Fatal(err)
	}
	foundAgent, foundTeam, err := st.FindAgentByTokenHash(ctx, auth.HashToken(agentToken))
	if err != nil {
		t.Fatal(err)
	}
	if foundAgent.ID != agent.ID || foundTeam.ID != team.ID {
		t.Fatalf("lookup got agent=%#v team=%#v", foundAgent, foundTeam)
	}
	round, err := st.CreateRound(ctx, RoundInput{Slug: "practice-agent", Name: "Practice Agent", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, MarketInput{Venue: "fake", ExternalID: "agent-market", Slug: "agent-market", Title: "Agent Market", Category: "arena", Status: "open", YesPriceBPS: 5000, NoPriceBPS: 5000})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddRoundMarket(ctx, round.ID, market.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.WithTx(ctx, func(tx *Tx) error {
		order, err := tx.CreateOrder(ctx, OrderInput{RoundID: round.ID, TeamID: team.ID, AgentID: &agent.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 5000, LimitPriceBPS: 5000, Status: "filled"})
		if err != nil {
			return err
		}
		_, err = tx.CreateFill(ctx, FillInput{RoundID: round.ID, TeamID: team.ID, OrderID: order.ID, MarketID: market.ID, Action: "buy", Outcome: "yes", AmountCents: 5000, FillPriceBPS: 5000})
		return err
	}); err != nil {
		t.Fatal(err)
	}
	portfolio, err := st.ComputePortfolio(ctx, round.ID, team.ID)
	if err != nil {
		t.Fatal(err)
	}
	if portfolio.GrossExposureCents == 0 {
		t.Fatalf("expected filled order exposure, got %#v", portfolio)
	}
}

func TestRoundAgentLockReplacesTeamSubmission(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	team, err := st.CreateTeam(ctx, "team-lock", "Team Lock", auth.HashToken("team-token"))
	if err != nil {
		t.Fatal(err)
	}
	agentOne, err := st.CreateAgent(ctx, AgentInput{TeamID: team.ID, Slug: "agent-one", Name: "Agent One"}, auth.HashToken("agent-one"))
	if err != nil {
		t.Fatal(err)
	}
	agentTwo, err := st.CreateAgent(ctx, AgentInput{TeamID: team.ID, Slug: "agent-two", Name: "Agent Two"}, auth.HashToken("agent-two"))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, RoundInput{Slug: "eval-1", Name: "Evaluation 1", Mode: "replay", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	locked, err := st.LockRoundAgent(ctx, RoundAgentInput{RoundID: round.ID, AgentID: agentOne.ID, CommitSHA: "abc123", DockerImage: "agent:one", LockedBy: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if locked.AgentID != agentOne.ID || locked.TeamID != team.ID {
		t.Fatalf("unexpected locked agent: %#v", locked)
	}
	ok, err := st.RoundAgentLocked(ctx, round.ID, agentOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		t.Fatal("expected first agent to be locked")
	}
	locked, err = st.LockRoundAgent(ctx, RoundAgentInput{RoundID: round.ID, AgentID: agentTwo.ID, CommitSHA: "def456", DockerImage: "agent:two", LockedBy: "test"})
	if err != nil {
		t.Fatal(err)
	}
	if locked.AgentID != agentTwo.ID || locked.CommitSHA != "def456" {
		t.Fatalf("lock did not replace team submission: %#v", locked)
	}
	ok, err = st.RoundAgentLocked(ctx, round.ID, agentOne.ID)
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Fatal("first agent should no longer be locked")
	}
	items, err := st.ListRoundAgents(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 1 || items[0].AgentID != agentTwo.ID {
		t.Fatalf("unexpected round agent list: %#v", items)
	}
}

func TestRoundTeamEnrollmentControlsLockPreflight(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t, ctx)
	teamOne, err := st.CreateTeam(ctx, "team-one", "Team One", auth.HashToken("team-one-token"))
	if err != nil {
		t.Fatal(err)
	}
	teamTwo, err := st.CreateTeam(ctx, "team-two", "Team Two", auth.HashToken("team-two-token"))
	if err != nil {
		t.Fatal(err)
	}
	agentOne, err := st.CreateAgent(ctx, AgentInput{TeamID: teamOne.ID, Slug: "default", Name: "Default"}, auth.HashToken("agent-one-token"))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, RoundInput{Slug: "eval-enrollment", Name: "Evaluation Enrollment", Mode: "replay", Status: "draft", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollRoundTeam(ctx, RoundTeamInput{RoundID: round.ID, TeamID: teamOne.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.EnrollRoundTeam(ctx, RoundTeamInput{RoundID: round.ID, TeamID: teamTwo.ID}); err != nil {
		t.Fatal(err)
	}
	preflight, err := st.CheckRoundAgentLocks(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(preflight.MissingTeams) != 2 {
		t.Fatalf("missing teams = %#v, want both enrolled teams", preflight.MissingTeams)
	}
	if _, err := st.SetRoundTeamStatus(ctx, round.ID, teamTwo.ID, "withdrawn"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.LockRoundAgent(ctx, RoundAgentInput{RoundID: round.ID, AgentID: agentOne.ID}); err != nil {
		t.Fatal(err)
	}
	preflight, err = st.CheckRoundAgentLocks(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !preflight.OK() || preflight.ActiveEnrolledTeamCount != 1 {
		t.Fatalf("preflight = %#v, want one active enrolled team with valid lock", preflight)
	}
	items, err := st.ListRoundTeams(ctx, round.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 2 {
		t.Fatalf("round teams = %#v, want 2", items)
	}
}
