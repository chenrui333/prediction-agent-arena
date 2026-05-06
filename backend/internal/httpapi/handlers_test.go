package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/auth"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/cache"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/db"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/events"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/risk"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue/fake"
)

func TestAuthRejectsMissingAndInvalidToken(t *testing.T) {
	fixture := newHTTPFixture(t)
	for _, tt := range []struct {
		name     string
		token    string
		wantCode string
	}{
		{name: "missing", wantCode: "missing_token"},
		{name: "invalid", token: "wrong", wantCode: "invalid_token"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			fixture.server.Router().ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			var response apiError
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error.Code != tt.wantCode {
				t.Fatalf("code = %q, want %s", response.Error.Code, tt.wantCode)
			}
		})
	}
}

func TestAuthRejectsInactiveTeam(t *testing.T) {
	fixture := newHTTPFixture(t)
	if _, err := fixture.server.Store.SetTeamActive(context.Background(), 1, false); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	req.Header.Set("Authorization", "Bearer "+fixture.token)
	rec := httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "inactive_team" {
		t.Fatalf("code = %q, want inactive_team", response.Error.Code)
	}
}

func TestAdminAuthRejectsMissingAndInvalidToken(t *testing.T) {
	fixture := newHTTPFixture(t)
	for _, tt := range []struct {
		name  string
		token string
	}{
		{name: "missing"},
		{name: "invalid", token: "wrong"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/v1/admin/teams", nil)
			if tt.token != "" {
				req.Header.Set("Authorization", "Bearer "+tt.token)
			}
			rec := httptest.NewRecorder()
			fixture.server.Router().ServeHTTP(rec, req)
			if rec.Code != http.StatusUnauthorized {
				t.Fatalf("status = %d, want 401", rec.Code)
			}
			var response apiError
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error.Code != "admin_auth_required" {
				t.Fatalf("code = %q, want admin_auth_required", response.Error.Code)
			}
		})
	}
}

func TestOrderRiskRejections(t *testing.T) {
	tests := []struct {
		name     string
		body     map[string]interface{}
		wantRisk string
		wantCode string
	}{
		{
			name: "too large",
			body: map[string]interface{}{
				"market_id":                 1,
				"outcome":                   "yes",
				"action":                    "buy",
				"amount_cents":              50001,
				"limit_price_bps":           5700,
				"estimated_probability_bps": 6400,
				"confidence":                "medium",
				"reason":                    "edge",
			},
			wantRisk: "order_value_limit",
			wantCode: "amount_too_large",
		},
		{
			name: "missing probability",
			body: map[string]interface{}{
				"market_id":       1,
				"outcome":         "yes",
				"action":          "buy",
				"amount_cents":    10000,
				"limit_price_bps": 5700,
				"confidence":      "medium",
				"reason":          "edge",
			},
			wantRisk: "estimated_probability_required",
			wantCode: "missing_estimated_probability",
		},
		{
			name: "missing reason",
			body: map[string]interface{}{
				"market_id":                 1,
				"outcome":                   "yes",
				"action":                    "buy",
				"amount_cents":              10000,
				"limit_price_bps":           5700,
				"estimated_probability_bps": 6400,
				"confidence":                "medium",
			},
			wantRisk: "reason_required",
			wantCode: "missing_reason",
		},
		{
			name: "missing limit price",
			body: map[string]interface{}{
				"market_id":                 1,
				"outcome":                   "yes",
				"action":                    "buy",
				"amount_cents":              10000,
				"estimated_probability_bps": 6400,
				"confidence":                "medium",
				"reason":                    "edge",
			},
			wantRisk: "limit_price_required",
			wantCode: "limit_price_required",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newHTTPFixture(t)
			rec := fixture.postStudent("/api/v1/orders", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			var response orderResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			var errResponse apiError
			if err := json.Unmarshal(rec.Body.Bytes(), &errResponse); err != nil {
				t.Fatal(err)
			}
			if errResponse.Error.Code != tt.wantCode {
				t.Fatalf("structured error code = %q, want %s", errResponse.Error.Code, tt.wantCode)
			}
			if response.Order.Status != "rejected" || response.Violation == nil || response.Violation.Type != tt.wantRisk {
				t.Fatalf("unexpected response: %#v", response)
			}
		})
	}
}

func TestTradeShapeValidationRejectsBeforeDBWrite(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		mutate   func(map[string]interface{})
		wantCode string
	}{
		{
			name: "invalid order action",
			path: "/api/v1/orders",
			mutate: func(payload map[string]interface{}) {
				payload["action"] = "hold"
			},
			wantCode: "invalid_action",
		},
		{
			name: "invalid decision probability",
			path: "/api/v1/decisions",
			mutate: func(payload map[string]interface{}) {
				payload["estimated_probability_bps"] = 0
			},
			wantCode: "malformed_probability",
		},
		{
			name: "invalid order limit price",
			path: "/api/v1/orders",
			mutate: func(payload map[string]interface{}) {
				payload["limit_price_bps"] = 10000
			},
			wantCode: "malformed_limit_price",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newHTTPFixture(t)
			payload := validOrderPayload()
			tt.mutate(payload)
			rec := fixture.postStudent(tt.path, payload)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
			}
			var response apiError
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if response.Error.Code != tt.wantCode {
				t.Fatalf("code = %q, want %s", response.Error.Code, tt.wantCode)
			}
			if got := fixture.countRows(t, "decisions"); got != 0 {
				t.Fatalf("decisions count = %d, want 0", got)
			}
			if got := fixture.countRows(t, "orders"); got != 0 {
				t.Fatalf("orders count = %d, want 0", got)
			}
			if got := fixture.countRows(t, "risk_events"); got != 0 {
				t.Fatalf("risk_events count = %d, want 0", got)
			}
		})
	}
}

func TestAcceptedOrderCreatesDecisionOrderFillAndPortfolio(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postStudent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Decision == nil || response.Order.ID == 0 || response.Fill == nil {
		t.Fatalf("missing decision/order/fill: %#v", response)
	}
	if response.Portfolio == nil || response.Portfolio.GrossExposureCents <= 0 {
		t.Fatalf("portfolio not updated: %#v", response.Portfolio)
	}
}

func TestOrderRateLimitCreatesRejectedOrderAndRiskEvent(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Policy.MaxOrdersPerMinute = 1
	if rec := fixture.postStudent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("first order status = %d: %s", rec.Code, rec.Body.String())
	}
	rec := fixture.postStudent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("second order status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Order.Status != "rejected" || response.Violation == nil || response.Violation.Type != "rate_limit" {
		t.Fatalf("unexpected rate limit response: %#v", response)
	}
	var errResponse apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &errResponse); err != nil {
		t.Fatal(err)
	}
	if errResponse.Error.Code != "rate_limit_exceeded" {
		t.Fatalf("code = %q, want rate_limit_exceeded", errResponse.Error.Code)
	}
	if got := fixture.countRows(t, "risk_events"); got != 1 {
		t.Fatalf("risk_events count = %d, want 1", got)
	}
}

func TestMaxOpenOrdersCreatesRejectedOrderAndRiskEvent(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Policy.MaxOpenOrders = 1
	payload := validOrderPayload()
	payload["limit_price_bps"] = 5600
	rec := fixture.postStudent("/api/v1/orders", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("first order status = %d: %s", rec.Code, rec.Body.String())
	}
	var first orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &first); err != nil {
		t.Fatal(err)
	}
	if first.Order.Status != "open" || first.Fill != nil {
		t.Fatalf("first order should remain open without fill: %#v", first)
	}

	rec = fixture.postStudent("/api/v1/orders", payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("second order status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Order.Status != "rejected" || response.Violation == nil || response.Violation.Type != "too_many_open_orders" {
		t.Fatalf("unexpected open-order response: %#v", response)
	}
	var errResponse apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &errResponse); err != nil {
		t.Fatal(err)
	}
	if errResponse.Error.Code != "max_open_orders_exceeded" {
		t.Fatalf("code = %q, want max_open_orders_exceeded", errResponse.Error.Code)
	}
	if got := fixture.countRows(t, "risk_events"); got != 1 {
		t.Fatalf("risk_events count = %d, want 1", got)
	}
}

func TestOpenOrderCanBeCanceled(t *testing.T) {
	fixture := newHTTPFixture(t)
	payload := validOrderPayload()
	payload["limit_price_bps"] = 5600
	rec := fixture.postStudent("/api/v1/orders", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("order status = %d: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Order.Status != "open" {
		t.Fatalf("order status = %q, want open", response.Order.Status)
	}

	rec = fixture.postStudent(fmt.Sprintf("/api/v1/orders/%d/cancel", response.Order.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("cancel status = %d: %s", rec.Code, rec.Body.String())
	}
	var canceled store.Order
	if err := json.Unmarshal(rec.Body.Bytes(), &canceled); err != nil {
		t.Fatal(err)
	}
	if canceled.Status != "canceled" {
		t.Fatalf("status = %q, want canceled", canceled.Status)
	}
}

func TestRedisUnavailableDoesNotBlockAcceptedOrder(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Cache = cache.New("127.0.0.1:1", "", nil)
	t.Cleanup(func() { _ = fixture.server.Cache.Close() })
	rec := fixture.postStudent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestLeaderboardCacheMissFallsBackToDB(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
	var response leaderboardResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Rows) != 1 || response.Rows[0].TeamSlug != "team-01" {
		t.Fatalf("unexpected leaderboard: %#v", response)
	}
}

func TestLeaderboardReadDoesNotCreateSnapshots(t *testing.T) {
	fixture := newHTTPFixture(t)
	beforePortfolio := fixture.countRows(t, "portfolio_snapshots")
	beforeScore := fixture.countRows(t, "score_snapshots")

	req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
	rec := httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := fixture.countRows(t, "portfolio_snapshots"); got != beforePortfolio {
		t.Fatalf("portfolio snapshots = %d, want %d", got, beforePortfolio)
	}
	if got := fixture.countRows(t, "score_snapshots"); got != beforeScore {
		t.Fatalf("score snapshots = %d, want %d", got, beforeScore)
	}
}

func TestAdminSummaryReadDoesNotCreateSnapshots(t *testing.T) {
	fixture := newHTTPFixture(t)
	beforePortfolio := fixture.countRows(t, "portfolio_snapshots")
	beforeScore := fixture.countRows(t, "score_snapshots")

	rec := fixture.getAdmin("/api/v1/admin/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	if got := fixture.countRows(t, "portfolio_snapshots"); got != beforePortfolio {
		t.Fatalf("portfolio snapshots = %d, want %d", got, beforePortfolio)
	}
	if got := fixture.countRows(t, "score_snapshots"); got != beforeScore {
		t.Fatalf("score snapshots = %d, want %d", got, beforeScore)
	}
}

func TestInvalidMarketStructuredError(t *testing.T) {
	fixture := newHTTPFixture(t)
	payload := validOrderPayload()
	payload["market_id"] = int64(999)
	rec := fixture.postStudent("/api/v1/orders", payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "invalid_market" {
		t.Fatalf("code = %q, want invalid_market", response.Error.Code)
	}
}

func TestResolvedMarketRejectsNewOrder(t *testing.T) {
	fixture := newHTTPFixture(t)
	if _, err := fixture.server.Store.ResolveSimulatedMarket(context.Background(), 1, "yes", "test"); err != nil {
		t.Fatal(err)
	}
	rec := fixture.postStudent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusConflict {
		t.Fatalf("status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "market_not_open" {
		t.Fatalf("code = %q, want market_not_open", response.Error.Code)
	}
}

func TestBuyOrderRejectsInsufficientCash(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Policy.MaxOrderValueCents = 2000000
	fixture.server.Policy.MaxTotalExposureCents = 2000000
	fixture.server.Policy.MaxPositionPerMarketCents = 2000000
	payload := validOrderPayload()
	payload["amount_cents"] = int64(1000001)
	rec := fixture.postStudent("/api/v1/orders", payload)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Violation == nil || response.Violation.Type != "insufficient_cash" {
		t.Fatalf("unexpected response: %#v", response)
	}
}

func TestAdminSummaryAndResetTeam(t *testing.T) {
	fixture := newHTTPFixture(t)
	if rec := fixture.postStudent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("order status = %d: %s", rec.Code, rec.Body.String())
	}
	rec := fixture.getAdmin("/api/v1/admin/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("summary status = %d: %s", rec.Code, rec.Body.String())
	}
	var summary adminSummaryResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Teams) != 1 || summary.Teams[0].TradeCount != 1 {
		t.Fatalf("unexpected summary: %#v", summary)
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/teams/1/reset", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.getAdmin("/api/v1/admin/summary")
	if rec.Code != http.StatusOK {
		t.Fatalf("summary status = %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &summary); err != nil {
		t.Fatal(err)
	}
	if len(summary.Teams) != 1 || summary.Teams[0].TradeCount != 0 {
		t.Fatalf("reset did not clear trade count: %#v", summary.Teams)
	}
}

func TestRoundScopedResetPreservesPriorRound(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	if rec := fixture.postStudent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("round 1 order status = %d: %s", rec.Code, rec.Body.String())
	}
	round2, err := fixture.server.Store.CreateRound(ctx, store.RoundInput{Slug: "practice-2", Name: "Practice 2", Status: "draft", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.server.Store.AddRoundMarket(ctx, round2.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.SetRoundStatus(ctx, round2.ID, "active"); err != nil {
		t.Fatal(err)
	}
	if rec := fixture.postStudent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("round 2 order status = %d: %s", rec.Code, rec.Body.String())
	}
	rec := fixture.postAdmin("/api/v1/admin/rounds/2/teams/1/reset", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reset status = %d: %s", rec.Code, rec.Body.String())
	}
	var round1Orders int
	if err := fixture.server.Store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE round_id = 1 AND team_id = 1").Scan(&round1Orders); err != nil {
		t.Fatal(err)
	}
	var round2Orders int
	if err := fixture.server.Store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM orders WHERE round_id = 2 AND team_id = 1").Scan(&round2Orders); err != nil {
		t.Fatal(err)
	}
	if round1Orders != 1 || round2Orders != 0 {
		t.Fatalf("round scoped reset counts round1=%d round2=%d", round1Orders, round2Orders)
	}
}

func TestRotateTeamTokenInvalidatesOldToken(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/teams/1/rotate-token", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate status = %d: %s", rec.Code, rec.Body.String())
	}
	var response createTeamResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.APIToken == "" || response.APIToken == fixture.token {
		t.Fatalf("unexpected token response: %#v", response)
	}
	oldToken := fixture.token
	fixture.token = oldToken
	rec = fixture.postStudent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old token status = %d, want 401: %s", rec.Code, rec.Body.String())
	}
	fixture.token = response.APIToken
	rec = fixture.postStudent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("new token status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	if got := fixture.countRows(t, "admin_actions"); got == 0 {
		t.Fatal("expected admin action")
	}
}

func TestHealthAndCompactSnapshots(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	for i := 0; i < 3; i++ {
		if _, err := fixture.server.Store.CreatePortfolioSnapshot(ctx, 1, 1); err != nil {
			t.Fatal(err)
		}
		if _, err := fixture.server.Store.RefreshScore(ctx, 1, 1); err != nil {
			t.Fatal(err)
		}
	}
	rec := fixture.getAdmin("/api/v1/admin/health")
	if rec.Code != http.StatusOK {
		t.Fatalf("health status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin("/api/v1/admin/snapshots/compact", map[string]interface{}{"round_id": 1, "keep_every": "1h"})
	if rec.Code != http.StatusOK {
		t.Fatalf("compact status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAdminResolveMarketUpdatesPublicPrice(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/markets/1/resolve", map[string]interface{}{"outcome": "no", "resolved_by": "test"})
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d: %s", rec.Code, rec.Body.String())
	}
	var state store.SimulatedMarketState
	if err := json.Unmarshal(rec.Body.Bytes(), &state); err != nil {
		t.Fatal(err)
	}
	if state.FinalOutcome != "no" || state.ResolvedAt == "" {
		t.Fatalf("unexpected resolved state: %#v", state)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/markets/1", nil)
	rec = httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("get market status = %d: %s", rec.Code, rec.Body.String())
	}
	var market store.Market
	if err := json.Unmarshal(rec.Body.Bytes(), &market); err != nil {
		t.Fatal(err)
	}
	if market.Status != "resolved" || market.YesPriceBPS != 0 || market.NoPriceBPS != 10000 {
		t.Fatalf("market after resolve = %#v", market)
	}
	outcome, err := fixture.server.Store.GetMarketOutcome(context.Background(), 1)
	if err != nil {
		t.Fatal(err)
	}
	if outcome.Outcome != "no" || outcome.ResolvedAt == "" {
		t.Fatalf("market outcome after resolve = %#v", outcome)
	}
}

type httpFixture struct {
	server *Server
	token  string
}

func newHTTPFixture(t *testing.T) httpFixture {
	t.Helper()
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
	token := "paa_test"
	team, err := st.CreateTeam(ctx, "team-01", "Team 01", auth.HashToken(token))
	if err != nil {
		t.Fatal(err)
	}
	round, err := st.CreateRound(ctx, store.RoundInput{Slug: "practice-1", Name: "Practice", Status: "active", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	market, err := st.UpsertMarket(ctx, store.MarketInput{
		Venue:        "fake",
		ExternalID:   "bootcamp-demo-1",
		Slug:         "ai-tool-usage-above-60",
		Title:        "Demo market",
		Category:     "bootcamp",
		Status:       "open",
		YesPriceBPS:  5700,
		NoPriceBPS:   4300,
		MetadataJSON: "{}",
		PricePathBPS: []int64{5700, 5900, 6100},
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
	server := &Server{
		Store:          st,
		Venue:          fake.NewStoreBacked(st),
		Cache:          cache.New("", "", nil),
		Events:         events.NewWriter(t.TempDir()),
		Policy:         risk.DefaultPolicy(),
		AdminToken:     "admin",
		LeaderboardTTL: time.Second,
		ExportDir:      t.TempDir(),
		CORSOrigin:     "*",
	}
	return httpFixture{server: server, token: token}
}

func (f httpFixture) postStudent(path string, payload map[string]interface{}) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(payload)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.server.Router().ServeHTTP(rec, req)
	return rec
}

func (f httpFixture) getAdmin(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer admin")
	rec := httptest.NewRecorder()
	f.server.Router().ServeHTTP(rec, req)
	return rec
}

func (f httpFixture) postAdmin(path string, payload map[string]interface{}) *httptest.ResponseRecorder {
	var body *bytes.Reader
	if payload == nil {
		body = bytes.NewReader(nil)
	} else {
		raw, _ := json.Marshal(payload)
		body = bytes.NewReader(raw)
	}
	req := httptest.NewRequest(http.MethodPost, path, body)
	req.Header.Set("Authorization", "Bearer admin")
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.server.Router().ServeHTTP(rec, req)
	return rec
}

func (f httpFixture) countRows(t *testing.T, table string) int {
	t.Helper()
	switch table {
	case "admin_actions", "decisions", "orders", "portfolio_snapshots", "risk_events", "score_snapshots":
	default:
		t.Fatalf("unsupported table %q", table)
	}
	var count int
	if err := f.server.Store.DB().QueryRowContext(context.Background(), "SELECT COUNT(*) FROM "+table).Scan(&count); err != nil {
		t.Fatal(err)
	}
	return count
}

func validOrderPayload() map[string]interface{} {
	return map[string]interface{}{
		"market_id":                 1,
		"outcome":                   "yes",
		"action":                    "buy",
		"amount_cents":              10000,
		"limit_price_bps":           5700,
		"estimated_probability_bps": 6400,
		"confidence":                "medium",
		"reason":                    "My estimate is above the market implied probability.",
	}
}
