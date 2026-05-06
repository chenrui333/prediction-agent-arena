package config

import (
	"strings"
	"testing"
)

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
		RateLimits:     RateLimits{Enabled: true, FailClosed: true},
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("expected wildcard origin error")
	}
}

func TestValidateExposedRequiresRateLimits(t *testing.T) {
	tests := []struct {
		name       string
		rateLimits RateLimits
		want       string
	}{
		{
			name:       "disabled",
			rateLimits: RateLimits{Enabled: false, FailClosed: true},
			want:       "ARENA_RATE_LIMIT_ENABLED=true",
		},
		{
			name:       "fail open",
			rateLimits: RateLimits{Enabled: true, FailClosed: false},
			want:       "ARENA_RATE_LIMIT_FAIL_CLOSED=true",
		},
		{
			name:       "valid",
			rateLimits: RateLimits{Enabled: true, FailClosed: true},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{
				Env:            "exposed",
				AdminToken:     "0123456789abcdef0123456789abcdef",
				AuditSalt:      "strong-audit-salt",
				AllowedOrigins: []string{"http://localhost:3000"},
				RateLimits:     tt.rateLimits,
			}
			err := cfg.Validate()
			if tt.want == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
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
