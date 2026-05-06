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
	market, err := st.UpsertMarket(ctx, MarketInput{Venue: "fake", ExternalID: "agent-market", Slug: "agent-market", Title: "Agent Market", Category: "bootcamp", Status: "open", YesPriceBPS: 5000, NoPriceBPS: 5000})
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
	round, err := st.CreateRound(ctx, RoundInput{Slug: "final-1", Name: "Final 1", Mode: "replay", Status: "active", InitialBalanceCents: 1000000})
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
