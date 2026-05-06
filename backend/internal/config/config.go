package config

import (
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTPAddr                     string
	DBPath                       string
	RedisAddr                    string
	RedisPassword                string
	AdminToken                   string
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
}

func Load() Config {
	return Config{
		HTTPAddr:                     env("ARENA_HTTP_ADDR", ":8080"),
		DBPath:                       env("ARENA_DB_PATH", "./data/arena.db"),
		RedisAddr:                    env("ARENA_REDIS_ADDR", "localhost:6379"),
		RedisPassword:                env("ARENA_REDIS_PASSWORD", ""),
		AdminToken:                   env("ARENA_ADMIN_TOKEN", "dev-admin-token"),
		LogDir:                       env("ARENA_LOG_DIR", "./logs"),
		ExportDir:                    env("ARENA_EXPORT_DIR", "./exports"),
		FrontendOrigin:               env("ARENA_FRONTEND_ORIGIN", "http://localhost:3000"),
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
	}
}

func env(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
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
