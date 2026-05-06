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
