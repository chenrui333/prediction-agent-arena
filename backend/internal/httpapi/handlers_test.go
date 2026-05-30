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
	"github.com/chenrui333/prediction-agent-arena/backend/internal/config"
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

func TestLegacyTeamTokenAuthCanBeDisabledAndEnabled(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.token = fixture.teamToken
	req := httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	req.Header.Set("Authorization", "Bearer "+fixture.token)
	rec := httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("legacy disabled status = %d, want 401: %s", rec.Code, rec.Body.String())
	}

	fixture.server.LegacyTeamAuth = true
	req = httptest.NewRequest(http.MethodGet, "/api/v1/portfolio", nil)
	req.Header.Set("Authorization", "Bearer "+fixture.token)
	rec = httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy enabled status = %d, want 200: %s", rec.Code, rec.Body.String())
	}
}

func TestMeReturnsAgentIdentityAndLegacyMode(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.getAgent("/api/v1/me")
	if rec.Code != http.StatusOK {
		t.Fatalf("agent me status = %d: %s", rec.Code, rec.Body.String())
	}
	var agentMe meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &agentMe); err != nil {
		t.Fatal(err)
	}
	if agentMe.Team.ID != 1 || agentMe.Agent == nil || agentMe.Agent.ID != fixture.agentID || agentMe.LegacyTeamAuth {
		t.Fatalf("unexpected agent me response: %#v", agentMe)
	}

	fixture.server.LegacyTeamAuth = true
	fixture.token = fixture.teamToken
	rec = fixture.getAgent("/api/v1/me")
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy me status = %d: %s", rec.Code, rec.Body.String())
	}
	var legacyMe meResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &legacyMe); err != nil {
		t.Fatal(err)
	}
	if legacyMe.Team.ID != 1 || legacyMe.Agent != nil || !legacyMe.LegacyTeamAuth {
		t.Fatalf("unexpected legacy me response: %#v", legacyMe)
	}
}

func TestPausedAgentCanHeartbeatButCannotTrade(t *testing.T) {
	fixture := newHTTPFixture(t)
	if _, err := fixture.server.Store.SetAgentStatus(context.Background(), fixture.agentID, "paused"); err != nil {
		t.Fatal(err)
	}
	rec := fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("heartbeat status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("order status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "paused_agent" {
		t.Fatalf("code = %q, want paused_agent", response.Error.Code)
	}
}

func TestReplayRoundRequiresLockedAgentForMutations(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	round, err := fixture.server.Store.CreateRound(ctx, store.RoundInput{Slug: "replay-1", Name: "Replay 1", Mode: "replay", Status: "draft", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.server.Store.AddRoundMarket(ctx, round.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.CreateTeam(ctx, "team-02", "Team 02", auth.HashToken("paa_team_02")); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.EnrollRoundTeam(ctx, store.RoundTeamInput{RoundID: round.ID, TeamID: 1}); err != nil {
		t.Fatal(err)
	}
	round, err = fixture.server.Store.SetRoundStatus(ctx, round.ID, "active")
	if err != nil {
		t.Fatal(err)
	}
	rec := fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unlocked order status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	var lockedError apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &lockedError); err != nil {
		t.Fatal(err)
	}
	if lockedError.Error.Code != "agent_not_locked_for_round" {
		t.Fatalf("code = %q, want agent_not_locked_for_round", lockedError.Error.Code)
	}

	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/%d/agents/%d/lock", round.ID, fixture.agentID), map[string]interface{}{"commit_sha": "abc123", "docker_image": "team-01:evaluation", "confirm": "replace_active_round_lock"})
	if rec.Code != http.StatusOK {
		t.Fatalf("lock status = %d: %s", rec.Code, rec.Body.String())
	}
	var locked store.RoundAgent
	if err := json.Unmarshal(rec.Body.Bytes(), &locked); err != nil {
		t.Fatal(err)
	}
	if locked.AgentID != fixture.agentID || locked.CommitSHA != "abc123" {
		t.Fatalf("unexpected lock response: %#v", locked)
	}
	rec = fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusCreated {
		t.Fatalf("locked order status = %d, want 201: %s", rec.Code, rec.Body.String())
	}
}

func TestPracticeRoundCanRequireLockedAgents(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/rounds/1/require-locked-agents", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("require locked status = %d: %s", rec.Code, rec.Body.String())
	}
	var round store.Round
	if err := json.Unmarshal(rec.Body.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	if !round.RequireLockedAgents {
		t.Fatalf("require_locked_agents = false, want true")
	}
	rec = fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("unlocked order status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", fixture.agentID), map[string]interface{}{"commit_sha": "abc123", "confirm": "replace_active_round_lock"})
	if rec.Code != http.StatusOK {
		t.Fatalf("lock status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusCreated {
		t.Fatalf("locked order status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/allow-unlocked-agents", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("allow unlocked status = %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &round); err != nil {
		t.Fatal(err)
	}
	if round.RequireLockedAgents {
		t.Fatalf("require_locked_agents = true, want false")
	}
}

func TestRoundTeamEnrollmentControlsAgentAccess(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/rounds/1/teams/1/pause", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pause round team status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.getAgent("/api/v1/portfolio")
	if rec.Code != http.StatusForbidden {
		t.Fatalf("paused enrollment portfolio status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_team_not_active" {
		t.Fatalf("code = %q, want round_team_not_active", response.Error.Code)
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/teams/1/resume", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("resume round team status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.getAgent("/api/v1/portfolio")
	if rec.Code != http.StatusOK {
		t.Fatalf("resumed enrollment portfolio status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/teams/1/withdraw", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("withdraw round team status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusForbidden {
		t.Fatalf("withdrawn enrollment order status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_team_not_active" {
		t.Fatalf("code = %q, want round_team_not_active", response.Error.Code)
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/teams/1/enroll", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("reenroll round team status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAgent("/api/v1/orders", validOrderPayload())
	if rec.Code != http.StatusCreated {
		t.Fatalf("reenrolled order status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestLockedRoundActivationPreflightsAgentLocks(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	round, err := fixture.server.Store.CreateRound(ctx, store.RoundInput{Slug: "eval-1", Name: "Evaluation 1", Status: "draft", RequireLockedAgents: true, InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.server.Store.AddRoundMarket(ctx, round.ID, 1); err != nil {
		t.Fatal(err)
	}
	rec := fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/%d/activate", round.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("activate missing enrollment status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_enrollment_empty" {
		t.Fatalf("code = %q, want round_enrollment_empty", response.Error.Code)
	}
	if _, err := fixture.server.Store.EnrollRoundTeam(ctx, store.RoundTeamInput{RoundID: round.ID, TeamID: 1}); err != nil {
		t.Fatal(err)
	}

	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/%d/activate", round.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("activate missing lock status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_agent_locks_incomplete" || !bytes.Contains(rec.Body.Bytes(), []byte("team-01")) || bytes.Contains(rec.Body.Bytes(), []byte("team-02")) {
		t.Fatalf("unexpected preflight response: %#v body=%s", response, rec.Body.String())
	}

	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/%d/agents/%d/lock", round.ID, fixture.agentID), map[string]interface{}{"commit_sha": "abc123"})
	if rec.Code != http.StatusOK {
		t.Fatalf("lock draft round status = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := fixture.server.Store.SetAgentStatus(ctx, fixture.agentID, "paused"); err != nil {
		t.Fatal(err)
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/%d/activate", round.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("activate paused agent status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_agent_locks_incomplete" || !bytes.Contains(rec.Body.Bytes(), []byte("locked agent is not active")) {
		t.Fatalf("unexpected paused-agent preflight response: %#v body=%s", response, rec.Body.String())
	}

	if _, err := fixture.server.Store.SetAgentStatus(ctx, fixture.agentID, "active"); err != nil {
		t.Fatal(err)
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/%d/activate", round.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("activate locked round status = %d: %s", rec.Code, rec.Body.String())
	}
}

func TestRoundAgentLockMutationSafety(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	rec := fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", fixture.agentID), map[string]interface{}{"commit_sha": "abc123"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("active lock without confirm status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "active_round_lock_confirm_required" {
		t.Fatalf("code = %q, want active_round_lock_confirm_required", response.Error.Code)
	}

	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", fixture.agentID), map[string]interface{}{"commit_sha": "abc123", "confirm": "replace_active_round_lock"})
	if rec.Code != http.StatusOK {
		t.Fatalf("active lock with confirm status = %d: %s", rec.Code, rec.Body.String())
	}
	second, err := fixture.server.Store.CreateAgent(ctx, store.AgentInput{TeamID: 1, Slug: "second", Name: "Second Agent"}, auth.HashToken("paa_agent_second"))
	if err != nil {
		t.Fatal(err)
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", second.ID), map[string]interface{}{"commit_sha": "def456"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("active replace without confirm status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", second.ID), map[string]interface{}{"commit_sha": "def456", "confirm": "replace_active_round_lock"})
	if rec.Code != http.StatusOK {
		t.Fatalf("active replace with confirm status = %d: %s", rec.Code, rec.Body.String())
	}
	var metadataJSON string
	if err := fixture.server.Store.DB().QueryRowContext(ctx, `
		SELECT metadata_json
		FROM admin_actions
		WHERE action = 'lock_round_agent'
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&metadataJSON); err != nil {
		t.Fatal(err)
	}
	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(metadataJSON), &metadata); err != nil {
		t.Fatal(err)
	}
	if int64(metadata["old_agent_id"].(float64)) != fixture.agentID || int64(metadata["new_agent_id"].(float64)) != second.ID {
		t.Fatalf("lock metadata missing old/new agent ids: %s", metadataJSON)
	}

	if _, err := fixture.server.Store.SetRoundStatus(ctx, 1, "completed"); err != nil {
		t.Fatal(err)
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", fixture.agentID), map[string]interface{}{"confirm": "replace_active_round_lock"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("completed lock status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_agent_lock_immutable" {
		t.Fatalf("code = %q, want round_agent_lock_immutable", response.Error.Code)
	}
}

func TestRoundAgentUnlockRemovesLock(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	rec := fixture.postAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", fixture.agentID), map[string]interface{}{"commit_sha": "abc123", "confirm": "replace_active_round_lock"})
	if rec.Code != http.StatusOK {
		t.Fatalf("lock status = %d: %s", rec.Code, rec.Body.String())
	}

	rec = fixture.deleteAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock", fixture.agentID))
	if rec.Code != http.StatusConflict {
		t.Fatalf("active unlock without confirm status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var activeUnlockError apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &activeUnlockError); err != nil {
		t.Fatal(err)
	}
	if activeUnlockError.Error.Code != "active_round_lock_confirm_required" {
		t.Fatalf("code = %q, want active_round_lock_confirm_required", activeUnlockError.Error.Code)
	}

	rec = fixture.deleteAdmin(fmt.Sprintf("/api/v1/admin/rounds/1/agents/%d/lock?confirm=replace_active_round_lock", fixture.agentID))
	if rec.Code != http.StatusOK {
		t.Fatalf("unlock status = %d: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", response.Deleted)
	}
	locked, err := fixture.server.Store.RoundAgentLocked(ctx, 1, fixture.agentID)
	if err != nil {
		t.Fatal(err)
	}
	if locked {
		t.Fatal("agent should no longer be locked")
	}
	if count := fixture.countRows(t, "round_agents"); count != 0 {
		t.Fatalf("round_agents rows = %d, want 0", count)
	}

	var metadataJSON string
	if err := fixture.server.Store.DB().QueryRowContext(ctx, "SELECT metadata_json FROM admin_actions WHERE action = 'unlock_round_agent' ORDER BY id DESC LIMIT 1").Scan(&metadataJSON); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains([]byte(metadataJSON), []byte("\"deleted\":1")) {
		t.Fatalf("unlock metadata missing delete count: %s", metadataJSON)
	}
}

func TestRevokedAgentCannotCallAgentAPI(t *testing.T) {
	fixture := newHTTPFixture(t)
	if _, err := fixture.server.Store.SetAgentStatus(context.Background(), fixture.agentID, "revoked"); err != nil {
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
	if response.Error.Code != "revoked_agent" {
		t.Fatalf("code = %q, want revoked_agent", response.Error.Code)
	}
}

func TestAdminAgentLifecycleAndRotation(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/teams/1/agents", map[string]interface{}{"slug": "bot-2", "name": "Bot 2"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("create agent status = %d: %s", rec.Code, rec.Body.String())
	}
	var created agentTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatal(err)
	}
	if created.APIToken == "" || created.Agent.ID == 0 {
		t.Fatalf("missing agent token response: %#v", created)
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/agents/%d/pause", created.Agent.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("pause agent status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/agents/%d/resume", created.Agent.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("resume agent status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin(fmt.Sprintf("/api/v1/admin/agents/%d/rotate-token", created.Agent.ID), nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("rotate agent status = %d: %s", rec.Code, rec.Body.String())
	}
	var rotated agentTokenResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &rotated); err != nil {
		t.Fatal(err)
	}
	if rotated.APIToken == "" || rotated.APIToken == created.APIToken {
		t.Fatalf("unexpected rotated token: %#v", rotated)
	}
}

func TestRateLimitMiddlewareReturns429AndAudits(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Cache = cache.NewMemory(nil)
	fixture.server.RateLimits = config.RateLimits{Enabled: true, AgentHeartbeatPerMinute: 1, AuthFailurePerMinute: 100}
	if rec := fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"}); rec.Code != http.StatusCreated {
		t.Fatalf("first heartbeat status = %d: %s", rec.Code, rec.Body.String())
	}
	rec := fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("second heartbeat status = %d, want 429: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "rate_limit_exceeded" {
		t.Fatalf("code = %q, want rate_limit_exceeded", response.Error.Code)
	}
	if got := fixture.countRows(t, "api_requests"); got != 2 {
		t.Fatalf("api_requests count = %d, want 2", got)
	}
}

func TestRequestAuditStoresAgentAndHashedClientData(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusCreated {
		t.Fatalf("heartbeat status = %d: %s", rec.Code, rec.Body.String())
	}
	var teamID, agentID int64
	var status int
	var ipHash, userAgentHash string
	if err := fixture.server.Store.DB().QueryRowContext(context.Background(), `
		SELECT team_id, agent_id, status, ip_hash, user_agent_hash
		FROM api_requests
		ORDER BY id DESC
		LIMIT 1
	`).Scan(&teamID, &agentID, &status, &ipHash, &userAgentHash); err != nil {
		t.Fatal(err)
	}
	if teamID != 1 || agentID != fixture.agentID || status != http.StatusCreated {
		t.Fatalf("unexpected audit row team=%d agent=%d status=%d", teamID, agentID, status)
	}
	if ipHash == "" || userAgentHash == "" || ipHash == "192.0.2.1" {
		t.Fatalf("audit hashes not populated or raw IP stored: ip=%q ua=%q", ipHash, userAgentHash)
	}
}

func TestClientIPTrustsProxyHeadersOnlyWhenConfigured(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/leaderboard", nil)
	req.RemoteAddr = "198.51.100.10:12345"
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.10")
	server := &Server{}
	if got := server.clientIP(req); got != "198.51.100.10" {
		t.Fatalf("default clientIP = %q, want remote addr", got)
	}
	server.TrustProxyHeaders = true
	server.TrustedProxyCIDRs = []string{"198.51.100.0/24"}
	if got := server.clientIP(req); got != "203.0.113.9" {
		t.Fatalf("trusted proxy clientIP = %q, want forwarded client", got)
	}
	server.TrustedProxyCIDRs = []string{"192.0.2.0/24"}
	if got := server.clientIP(req); got != "198.51.100.10" {
		t.Fatalf("untrusted proxy clientIP = %q, want remote addr", got)
	}
}

func TestRedisRateLimiterFailClosed(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Cache = cache.New("127.0.0.1:1", "", nil)
	t.Cleanup(func() { _ = fixture.server.Cache.Close() })
	fixture.server.RateLimits = config.RateLimits{Enabled: true, FailClosed: true, AgentHeartbeatPerMinute: 1, AuthFailurePerMinute: 100}
	rec := fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("status = %d, want 429: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "rate_limiter_unavailable" {
		t.Fatalf("code = %q, want rate_limiter_unavailable", response.Error.Code)
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
			rec := fixture.postAgent("/api/v1/orders", tt.body)
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
			rec := fixture.postAgent(tt.path, payload)
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

func TestJSONDecodeRejectsUnknownFieldsAndOversizedBodies(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAgentRaw("/api/v1/orders", []byte("{\"market_id\":1,\"outcome\":\"yes\",\"action\":\"buy\",\"amount_cents\":1000,\"limit_price_bps\":5700,\"estimated_probability_bps\":6400,\"confidence\":\"medium\",\"reason\":\"edge\",\"unexpected\":true}"))
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("unknown field status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "invalid_json" || !bytes.Contains(rec.Body.Bytes(), []byte("unknown field")) {
		t.Fatalf("unexpected unknown field response: %#v body=%s", response, rec.Body.String())
	}
	if got := fixture.countRows(t, "decisions"); got != 0 {
		t.Fatalf("decisions count = %d, want 0", got)
	}
	if got := fixture.countRows(t, "orders"); got != 0 {
		t.Fatalf("orders count = %d, want 0", got)
	}

	largeReason := bytes.Repeat([]byte("x"), int(maxJSONBodyBytes))
	largeBody := append([]byte("{\"market_id\":1,\"outcome\":\"yes\",\"action\":\"buy\",\"amount_cents\":1000,\"limit_price_bps\":5700,\"estimated_probability_bps\":6400,\"confidence\":\"medium\",\"reason\":\""), largeReason...)
	largeBody = append(largeBody, []byte("\"}")...)
	rec = fixture.postAgentRaw("/api/v1/orders", largeBody)
	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("oversized body status = %d, want 413: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "request_body_too_large" {
		t.Fatalf("code = %q, want request_body_too_large", response.Error.Code)
	}
	if got := fixture.countRows(t, "decisions"); got != 0 {
		t.Fatalf("decisions count after oversized body = %d, want 0", got)
	}
	if got := fixture.countRows(t, "orders"); got != 0 {
		t.Fatalf("orders count after oversized body = %d, want 0", got)
	}
}

func TestAcceptedOrderCreatesDecisionOrderFillAndPortfolio(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAgent("/api/v1/orders", validOrderPayload())
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
	if response.Decision.AgentID == nil || *response.Decision.AgentID != fixture.agentID {
		t.Fatalf("decision agent id = %#v, want %d", response.Decision.AgentID, fixture.agentID)
	}
	if response.Order.AgentID == nil || *response.Order.AgentID != fixture.agentID {
		t.Fatalf("order agent id = %#v, want %d", response.Order.AgentID, fixture.agentID)
	}
	if response.Portfolio == nil || response.Portfolio.GrossExposureCents <= 0 {
		t.Fatalf("portfolio not updated: %#v", response.Portfolio)
	}
}

func TestOrderRateLimitCreatesRejectedOrderAndRiskEvent(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Policy.MaxOrdersPerMinute = 1
	if rec := fixture.postAgent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("first order status = %d: %s", rec.Code, rec.Body.String())
	}
	rec := fixture.postAgent("/api/v1/orders", validOrderPayload())
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
	rec := fixture.postAgent("/api/v1/orders", payload)
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

	rec = fixture.postAgent("/api/v1/orders", payload)
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
	rec := fixture.postAgent("/api/v1/orders", payload)
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

	rec = fixture.postAgent(fmt.Sprintf("/api/v1/orders/%d/cancel", response.Order.ID), nil)
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

func TestCancelOrderRequiresActiveRoundIsolation(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	payload := validOrderPayload()
	payload["limit_price_bps"] = 5600
	rec := fixture.postAgent("/api/v1/orders", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("round 1 order status = %d: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	round2, err := fixture.server.Store.CreateRound(ctx, store.RoundInput{Slug: "practice-2", Name: "Practice 2", Status: "draft", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.server.Store.AddRoundMarket(ctx, round2.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.EnrollRoundTeam(ctx, store.RoundTeamInput{RoundID: round2.ID, TeamID: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.SetRoundStatus(ctx, round2.ID, "active"); err != nil {
		t.Fatal(err)
	}
	rec = fixture.postAgent(fmt.Sprintf("/api/v1/orders/%d/cancel", response.Order.ID), nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("cancel old-round status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var errResponse apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &errResponse); err != nil {
		t.Fatal(err)
	}
	if errResponse.Error.Code != "order_not_in_active_round" {
		t.Fatalf("code = %q, want order_not_in_active_round", errResponse.Error.Code)
	}
}

func TestLockedRoundCancelRequiresCreatingAgent(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	payload := validOrderPayload()
	payload["limit_price_bps"] = 5600
	rec := fixture.postAgent("/api/v1/orders", payload)
	if rec.Code != http.StatusCreated {
		t.Fatalf("order status = %d: %s", rec.Code, rec.Body.String())
	}
	var response orderResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	secondToken := "paa_agent_second"
	second, err := fixture.server.Store.CreateAgent(ctx, store.AgentInput{TeamID: 1, Slug: "second", Name: "Second Agent"}, auth.HashToken(secondToken))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.SetRoundRequireLockedAgents(ctx, 1, true); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.LockRoundAgent(ctx, store.RoundAgentInput{RoundID: 1, AgentID: second.ID, LockedBy: "test"}); err != nil {
		t.Fatal(err)
	}
	fixture.token = secondToken
	rec = fixture.postAgent(fmt.Sprintf("/api/v1/orders/%d/cancel", response.Order.ID), nil)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("cancel mismatch status = %d, want 403: %s", rec.Code, rec.Body.String())
	}
	var errResponse apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &errResponse); err != nil {
		t.Fatal(err)
	}
	if errResponse.Error.Code != "order_agent_mismatch" {
		t.Fatalf("code = %q, want order_agent_mismatch", errResponse.Error.Code)
	}
}

func TestRedisUnavailableDoesNotBlockAcceptedOrder(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.Cache = cache.New("127.0.0.1:1", "", nil)
	t.Cleanup(func() { _ = fixture.server.Cache.Close() })
	rec := fixture.postAgent("/api/v1/orders", validOrderPayload())
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

func TestPublicTeamActivityRedactsActiveRoundStrategy(t *testing.T) {
	fixture := newHTTPFixture(t)
	if rec := fixture.postAgent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("order status = %d: %s", rec.Code, rec.Body.String())
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/teams/team-01", nil)
	rec := httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public team activity status = %d: %s", rec.Code, rec.Body.String())
	}
	if bytes.Contains(rec.Body.Bytes(), []byte("My estimate is above")) {
		t.Fatalf("public team activity leaked decision reason: %s", rec.Body.String())
	}
	var public teamActivityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	if !public.DetailRedacted || public.Visibility != "summary" {
		t.Fatalf("unexpected public visibility: %#v", public)
	}
	if len(public.Decisions) != 0 || len(public.Orders) != 0 || len(public.Fills) != 0 || len(public.RiskEvents) != 0 {
		t.Fatalf("public detail should be empty: decisions=%d orders=%d fills=%d risks=%d", len(public.Decisions), len(public.Orders), len(public.Fills), len(public.RiskEvents))
	}
	if public.TradeCount != 1 {
		t.Fatalf("trade_count = %d, want 1", public.TradeCount)
	}

	rec = fixture.getAdmin("/api/v1/admin/rounds/1/teams/1/activity")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin team activity status = %d: %s", rec.Code, rec.Body.String())
	}
	var admin teamActivityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &admin); err != nil {
		t.Fatal(err)
	}
	if len(admin.Decisions) != 1 || admin.Decisions[0].Reason == "" || len(admin.Orders) != 1 || len(admin.Fills) != 1 {
		t.Fatalf("admin detail missing strategy data: %#v", admin)
	}
}

func TestPublicTeamActivityCanShowCompletedPostmortemWhenEnabled(t *testing.T) {
	fixture := newHTTPFixture(t)
	fixture.server.PublicTeamActivity = "full"
	if rec := fixture.postAgent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("order status = %d: %s", rec.Code, rec.Body.String())
	}
	if _, err := fixture.server.Store.SetRoundStatus(context.Background(), 1, "completed"); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/teams/team-01?round_id=1", nil)
	rec := httptest.NewRecorder()
	fixture.server.Router().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("public completed team activity status = %d: %s", rec.Code, rec.Body.String())
	}
	var public teamActivityResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &public); err != nil {
		t.Fatal(err)
	}
	if public.DetailRedacted || public.Visibility != "full" || len(public.Decisions) != 1 || public.Decisions[0].Reason == "" {
		t.Fatalf("completed full activity not returned: %#v", public)
	}
}

func TestInvalidMarketStructuredError(t *testing.T) {
	fixture := newHTTPFixture(t)
	payload := validOrderPayload()
	payload["market_id"] = int64(999)
	rec := fixture.postAgent("/api/v1/orders", payload)
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
	rec := fixture.postAgent("/api/v1/orders", validOrderPayload())
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
	rec := fixture.postAgent("/api/v1/orders", payload)
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
	if rec := fixture.postAgent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
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
	if rec := fixture.postAgent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
		t.Fatalf("round 1 order status = %d: %s", rec.Code, rec.Body.String())
	}
	round2, err := fixture.server.Store.CreateRound(ctx, store.RoundInput{Slug: "practice-2", Name: "Practice 2", Status: "draft", InitialBalanceCents: 1000000})
	if err != nil {
		t.Fatal(err)
	}
	if err := fixture.server.Store.AddRoundMarket(ctx, round2.ID, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.EnrollRoundTeam(ctx, store.RoundTeamInput{RoundID: round2.ID, TeamID: 1}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.SetRoundStatus(ctx, round2.ID, "active"); err != nil {
		t.Fatal(err)
	}
	if rec := fixture.postAgent("/api/v1/orders", validOrderPayload()); rec.Code != http.StatusCreated {
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
	fixture.server.LegacyTeamAuth = true
	fixture.token = fixture.teamToken
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
	rec = fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("old token status = %d, want 401: %s", rec.Code, rec.Body.String())
	}
	fixture.token = response.APIToken
	rec = fixture.postAgent("/api/v1/heartbeat", map[string]interface{}{"status": "online"})
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

func TestCompactAuditDeletesOldRows(t *testing.T) {
	fixture := newHTTPFixture(t)
	ctx := context.Background()
	if err := fixture.server.Store.CreateAPIRequest(ctx, store.APIRequestInput{Method: "POST", Path: "/old", Status: 201, IPHash: "old", UserAgentHash: "old"}); err != nil {
		t.Fatal(err)
	}
	if _, err := fixture.server.Store.DB().ExecContext(ctx, "UPDATE api_requests SET created_at = '2000-01-01T00:00:00Z' WHERE path = '/old'"); err != nil {
		t.Fatal(err)
	}
	if err := fixture.server.Store.CreateAPIRequest(ctx, store.APIRequestInput{Method: "POST", Path: "/new", Status: 201, IPHash: "new", UserAgentHash: "new"}); err != nil {
		t.Fatal(err)
	}
	rec := fixture.postAdmin("/api/v1/admin/audit/compact", map[string]interface{}{"older_than": "1d"})
	if rec.Code != http.StatusOK {
		t.Fatalf("compact audit status = %d: %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Deleted int64 `json:"deleted"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Deleted != 1 {
		t.Fatalf("deleted = %d, want 1", result.Deleted)
	}
	var oldRows int
	if err := fixture.server.Store.DB().QueryRowContext(ctx, "SELECT COUNT(*) FROM api_requests WHERE path = '/old'").Scan(&oldRows); err != nil {
		t.Fatal(err)
	}
	if oldRows != 0 {
		t.Fatalf("old audit rows = %d, want 0", oldRows)
	}
}

func TestCompactAuditRejectsInvalidDuration(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/audit/compact", map[string]interface{}{"older_than": "0d"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "invalid_older_than" {
		t.Fatalf("code = %q, want invalid_older_than", response.Error.Code)
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

func TestSettleRoundPreflightAndCompletion(t *testing.T) {
	fixture := newHTTPFixture(t)
	rec := fixture.postAdmin("/api/v1/admin/rounds/1/settle", map[string]interface{}{"confirm": "settle_active_round"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("unresolved settle status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	var response apiError
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "round_markets_unresolved" {
		t.Fatalf("code = %q, want round_markets_unresolved", response.Error.Code)
	}
	rec = fixture.postAdmin("/api/v1/admin/markets/1/resolve", map[string]interface{}{"outcome": "yes", "resolved_by": "test"})
	if rec.Code != http.StatusOK {
		t.Fatalf("resolve status = %d: %s", rec.Code, rec.Body.String())
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/settle", nil)
	if rec.Code != http.StatusConflict {
		t.Fatalf("active settle without confirm status = %d, want 409: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	if response.Error.Code != "settle_active_round_confirm_required" {
		t.Fatalf("code = %q, want settle_active_round_confirm_required", response.Error.Code)
	}
	rec = fixture.postAdmin("/api/v1/admin/rounds/1/settle", map[string]interface{}{"confirm": "settle_active_round", "complete_after_settlement": true})
	if rec.Code != http.StatusOK {
		t.Fatalf("confirmed settle status = %d: %s", rec.Code, rec.Body.String())
	}
	var result struct {
		Round store.Round `json:"round"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result.Round.Status != "completed" {
		t.Fatalf("round status = %q, want completed", result.Round.Status)
	}
}

func TestPublicMarketsRedactMetadata(t *testing.T) {
	fixture := newHTTPFixture(t)
	if _, err := fixture.server.Store.UpsertMarket(context.Background(), store.MarketInput{
		Venue:        "fake",
		ExternalID:   "bootcamp-demo-1",
		Slug:         "ai-tool-usage-above-60",
		Title:        "Demo market",
		Category:     "arena",
		Status:       "open",
		YesPriceBPS:  5700,
		NoPriceBPS:   4300,
		MetadataJSON: `{"private_notes":"secret","true_probability_bps":7200}`,
	}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/api/v1/markets", "/api/v1/markets/1"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		fixture.server.Router().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s status = %d: %s", path, rec.Code, rec.Body.String())
		}
		if bytes.Contains(rec.Body.Bytes(), []byte("metadata_json")) || bytes.Contains(rec.Body.Bytes(), []byte("private_notes")) {
			t.Fatalf("%s leaked metadata: %s", path, rec.Body.String())
		}
	}
	rec := fixture.getAdmin("/api/v1/admin/markets")
	if rec.Code != http.StatusOK {
		t.Fatalf("admin markets status = %d: %s", rec.Code, rec.Body.String())
	}
	if !bytes.Contains(rec.Body.Bytes(), []byte("metadata_json")) || !bytes.Contains(rec.Body.Bytes(), []byte("private_notes")) {
		t.Fatalf("admin markets should include metadata: %s", rec.Body.String())
	}
}

type httpFixture struct {
	server    *Server
	token     string
	teamToken string
	agentID   int64
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
	teamToken := "paa_team_test"
	team, err := st.CreateTeam(ctx, "team-01", "Team 01", auth.HashToken(teamToken))
	if err != nil {
		t.Fatal(err)
	}
	agentToken := "paa_agent_test"
	agent, err := st.CreateAgent(ctx, store.AgentInput{TeamID: team.ID, Slug: "default", Name: "Default Agent"}, auth.HashToken(agentToken))
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
		Category:     "arena",
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
	if _, err := st.EnrollRoundTeam(ctx, store.RoundTeamInput{RoundID: round.ID, TeamID: team.ID}); err != nil {
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
		CORSOrigins:    []string{"*"},
		AuditSalt:      "test-audit-salt",
	}
	return httpFixture{server: server, token: agentToken, teamToken: teamToken, agentID: agent.ID}
}

func (f httpFixture) postAgent(path string, payload map[string]interface{}) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(payload)
	return f.postAgentRaw(path, raw)
}

func (f httpFixture) postAgentRaw(path string, raw []byte) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+f.token)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	f.server.Router().ServeHTTP(rec, req)
	return rec
}

func (f httpFixture) getAgent(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, path, nil)
	req.Header.Set("Authorization", "Bearer "+f.token)
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

func (f httpFixture) deleteAdmin(path string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodDelete, path, nil)
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
	case "admin_actions", "agents", "api_requests", "decisions", "orders", "portfolio_snapshots", "risk_events", "round_agents", "score_snapshots":
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
