package config

import "testing"

func TestValidateExposedRejectsWeakAdminToken(t *testing.T) {
	cfg := Config{
		Env:            "exposed",
		AdminToken:     "dev-admin-token",
		AuditSalt:      "strong-audit-salt",
		AllowedOrigins: []string{"http://localhost:3000"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected weak admin token error")
	}
}

func TestValidateExposedRejectsWildcardOrigin(t *testing.T) {
	cfg := Config{
		Env:            "exposed",
		AdminToken:     "0123456789abcdef0123456789abcdef",
		AuditSalt:      "strong-audit-salt",
		AllowedOrigins: []string{"*"},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard origin error")
	}
}

func TestLoadRateLimitAndOriginEnv(t *testing.T) {
	t.Setenv("ARENA_ALLOWED_ORIGINS", "http://localhost:3000,http://127.0.0.1:3000")
	t.Setenv("ARENA_LEGACY_TEAM_TOKEN_AUTH", "true")
	t.Setenv("ARENA_RATE_LIMIT_FAIL_CLOSED", "true")
	t.Setenv("ARENA_AGENT_ORDER_LIMIT_PER_MINUTE", "7")

	cfg := Load()
	if len(cfg.AllowedOrigins) != 2 {
		t.Fatalf("origins = %#v, want 2", cfg.AllowedOrigins)
	}
	if !cfg.LegacyTeamTokenAuth {
		t.Fatal("expected legacy team auth true")
	}
	if !cfg.RateLimits.FailClosed {
		t.Fatal("expected fail closed true")
	}
	if cfg.RateLimits.AgentOrderPerMinute != 7 {
		t.Fatalf("order limit = %d, want 7", cfg.RateLimits.AgentOrderPerMinute)
	}
}
