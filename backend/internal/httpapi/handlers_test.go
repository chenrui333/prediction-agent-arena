package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
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
		name  string
		token string
	}{
		{name: "missing"},
		{name: "invalid", token: "wrong"},
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
	rec = fixture.postAdmin("/api/v1/admin/teams/1/reset", nil)
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
		Venue:          fake.New(),
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
