package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type RateLimits struct {
	Enabled                 bool
	FailClosed              bool
	AgentOrderPerMinute     int
	AgentDecisionPerMinute  int
	AgentHeartbeatPerMinute int
	AgentReadPerMinute      int
	PublicReadPerMinute     int
	AdminPerMinute          int
	AuthFailurePerMinute    int
}

type Config struct {
	Env                          string
	HTTPAddr                     string
	DBPath                       string
	RedisAddr                    string
	RedisPassword                string
	AdminToken                   string
	AllowedOrigins               []string
	PublicTeamActivity           string
	LegacyTeamTokenAuth          bool
	AuditSalt                    string
	TrustProxyHeaders            bool
	TrustedProxyCIDRs            []string
	LogDir                       string
	ExportDir                    string
	FrontendOrigin               string
	Venue                        string
	PolymarketPaperBin           string
	PolymarketPaperAccountPrefix string
	PolymarketPaperTimeout       time.Duration
	PolymarketPaperDataDir       string
	ShutdownTimeout              time.Duration
	SnapshotInterval             time.Duration
	LeaderboardTTL               time.Duration
	InitialBalance               int64
	MaxOrderValue                int64
	MaxPositionMarket            int64
	MaxTotalExposure             int64
	MaxOrdersPerMinute           int
	MaxOpenOrders                int
	RateLimits                   RateLimits
}

func Load() Config {
	frontendOrigin := env("ARENA_FRONTEND_ORIGIN", "http://localhost:3000")
	return Config{
		Env:                          env("ARENA_ENV", "local"),
		HTTPAddr:                     env("ARENA_HTTP_ADDR", ":8080"),
		DBPath:                       env("ARENA_DB_PATH", "./data/arena.db"),
		RedisAddr:                    env("ARENA_REDIS_ADDR", "localhost:6379"),
		RedisPassword:                env("ARENA_REDIS_PASSWORD", ""),
		AdminToken:                   env("ARENA_ADMIN_TOKEN", "dev-admin-token"),
		AllowedOrigins:               envList("ARENA_ALLOWED_ORIGINS", []string{frontendOrigin, "http://127.0.0.1:3000"}),
		PublicTeamActivity:           normalizePublicTeamActivity(env("ARENA_PUBLIC_TEAM_ACTIVITY", "summary")),
		LegacyTeamTokenAuth:          envBool("ARENA_LEGACY_TEAM_TOKEN_AUTH", false),
		AuditSalt:                    env("ARENA_AUDIT_SALT", "local-dev-audit-salt"),
		TrustProxyHeaders:            envBool("ARENA_TRUST_PROXY_HEADERS", false),
		TrustedProxyCIDRs:            envList("ARENA_TRUSTED_PROXY_CIDRS", []string{"127.0.0.1/32", "::1/128"}),
		LogDir:                       env("ARENA_LOG_DIR", "./logs"),
		ExportDir:                    env("ARENA_EXPORT_DIR", "./exports"),
		FrontendOrigin:               frontendOrigin,
		Venue:                        env("ARENA_VENUE", "fake"),
		PolymarketPaperBin:           env("POLYMARKET_PAPER_BIN", "pm-trader"),
		PolymarketPaperAccountPrefix: env("POLYMARKET_PAPER_ACCOUNT_PREFIX", "arena"),
		PolymarketPaperTimeout:       envSecondsDuration("POLYMARKET_PAPER_TIMEOUT_SECONDS", 10*time.Second),
		PolymarketPaperDataDir:       env("POLYMARKET_PAPER_DATA_DIR", "./data/pm-trader"),
		ShutdownTimeout:              envDuration("ARENA_SHUTDOWN_TIMEOUT", 10*time.Second),
		SnapshotInterval:             envDuration("ARENA_SNAPSHOT_INTERVAL", time.Minute),
		LeaderboardTTL:               envDuration("ARENA_LEADERBOARD_TTL", 5*time.Second),
		InitialBalance:               envInt64("ARENA_INITIAL_BALANCE_CENTS", 1000000),
		MaxOrderValue:                envInt64("ARENA_MAX_ORDER_VALUE_CENTS", 50000),
		MaxPositionMarket:            envInt64("ARENA_MAX_POSITION_PER_MARKET_CENTS", 100000),
		MaxTotalExposure:             envInt64("ARENA_MAX_TOTAL_EXPOSURE_CENTS", 400000),
		MaxOrdersPerMinute:           envInt("ARENA_MAX_ORDERS_PER_MINUTE", 10),
		MaxOpenOrders:                envInt("ARENA_MAX_OPEN_ORDERS", 20),
		RateLimits: RateLimits{
			Enabled:                 envBool("ARENA_RATE_LIMIT_ENABLED", true),
			FailClosed:              envBool("ARENA_RATE_LIMIT_FAIL_CLOSED", false),
			AgentOrderPerMinute:     envInt("ARENA_AGENT_ORDER_LIMIT_PER_MINUTE", 10),
			AgentDecisionPerMinute:  envInt("ARENA_AGENT_DECISION_LIMIT_PER_MINUTE", 30),
			AgentHeartbeatPerMinute: envInt("ARENA_AGENT_HEARTBEAT_LIMIT_PER_MINUTE", 12),
			AgentReadPerMinute:      envInt("ARENA_AGENT_READ_LIMIT_PER_MINUTE", 120),
			PublicReadPerMinute:     envInt("ARENA_PUBLIC_READ_LIMIT_PER_MINUTE", 120),
			AdminPerMinute:          envInt("ARENA_ADMIN_LIMIT_PER_MINUTE", 120),
			AuthFailurePerMinute:    envInt("ARENA_AUTH_FAILURE_LIMIT_PER_MINUTE", 20),
		},
	}
}

func (c Config) Validate() error {
	envName := strings.ToLower(strings.TrimSpace(c.Env))
	if envName == "" {
		envName = "local"
	}
	if envName != "local" && envName != "exposed" {
		return fmt.Errorf("ARENA_ENV must be local or exposed, got %q", c.Env)
	}
	if envName != "exposed" {
		return nil
	}
	if c.AdminToken == "" || c.AdminToken == "dev-admin-token" || len(c.AdminToken) < 32 {
		return errors.New("ARENA_ENV=exposed requires ARENA_ADMIN_TOKEN to be non-default and at least 32 characters")
	}
	if len(c.AuditSalt) < 16 {
		return errors.New("ARENA_ENV=exposed requires ARENA_AUDIT_SALT to be at least 16 characters")
	}
	if normalizePublicTeamActivity(c.PublicTeamActivity) == "" {
		return errors.New("ARENA_PUBLIC_TEAM_ACTIVITY must be summary, redacted, or full")
	}
	if len(c.AllowedOrigins) == 0 {
		return errors.New("ARENA_ENV=exposed requires ARENA_ALLOWED_ORIGINS")
	}
	for _, origin := range c.AllowedOrigins {
		if origin == "*" {
			return errors.New("ARENA_ENV=exposed does not allow wildcard CORS origins")
		}
	}
	if !c.RateLimits.Enabled {
		return errors.New("ARENA_ENV=exposed requires ARENA_RATE_LIMIT_ENABLED=true")
	}
	if !c.RateLimits.FailClosed {
		return errors.New("ARENA_ENV=exposed requires ARENA_RATE_LIMIT_FAIL_CLOSED=true")
	}
	if c.LegacyTeamTokenAuth {
		return errors.New("ARENA_ENV=exposed requires ARENA_LEGACY_TEAM_TOKEN_AUTH=false")
	}
	return nil
}

func normalizePublicTeamActivity(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "", "summary":
		return "summary"
	case "redacted":
		return "redacted"
	case "full":
		return "full"
	default:
		return ""
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func envBool(key string, fallback bool) bool {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envList(key string, fallback []string) []string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parts := strings.Split(value, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	if len(out) == 0 {
		return fallback
	}
	return out
}

func envInt64(key string, fallback int64) int64 {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil {
		return fallback
	}
	return parsed
}

func envInt(key string, fallback int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envSecondsDuration(key string, fallback time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return fallback
	}
	return time.Duration(parsed) * time.Second
}
