package app

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"strings"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/cache"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/config"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/db"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/events"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/httpapi"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/risk"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue/fake"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue/polymarketpaper"
)

type App struct {
	Config config.Config
	DB     *sql.DB
	Store  *store.Store
	Cache  *cache.Client
	Events *events.Writer
	API    *httpapi.Server
}

func New(ctx context.Context, cfg config.Config, logger *slog.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	if strings.EqualFold(cfg.Env, "local") && cfg.AdminToken == "dev-admin-token" && logger != nil {
		logger.Warn("using default admin token in local mode; change ARENA_ADMIN_TOKEN before binding beyond localhost")
	}
	conn, err := db.Open(ctx, cfg.DBPath)
	if err != nil {
		return nil, err
	}
	if err := db.Migrate(ctx, conn); err != nil {
		_ = conn.Close()
		return nil, err
	}
	policy := risk.DefaultPolicy()
	policy.InitialBalanceCents = cfg.InitialBalance
	policy.MaxOrderValueCents = cfg.MaxOrderValue
	policy.MaxPositionPerMarketCents = cfg.MaxPositionMarket
	policy.MaxTotalExposureCents = cfg.MaxTotalExposure
	policy.MaxOrdersPerMinute = cfg.MaxOrdersPerMinute
	policy.MaxOpenOrders = cfg.MaxOpenOrders

	st := store.New(conn)
	redisClient := cache.New(cfg.RedisAddr, cfg.RedisPassword, logger)
	eventWriter := events.NewWriter(cfg.LogDir)
	selectedVenue, err := newVenue(cfg, st)
	if err != nil {
		_ = redisClient.Close()
		_ = conn.Close()
		return nil, err
	}
	api := &httpapi.Server{
		Store:          st,
		Venue:          selectedVenue,
		Cache:          redisClient,
		Events:         eventWriter,
		Policy:         policy,
		AdminToken:     cfg.AdminToken,
		Logger:         logger,
		LeaderboardTTL: cfg.LeaderboardTTL,
		ExportDir:      cfg.ExportDir,
		CORSOrigins:    cfg.AllowedOrigins,
		LegacyTeamAuth: cfg.LegacyTeamTokenAuth,
		AuditSalt:      cfg.AuditSalt,
		RateLimits:     cfg.RateLimits,
	}
	return &App{Config: cfg, DB: conn, Store: st, Cache: redisClient, Events: eventWriter, API: api}, nil
}

func newVenue(cfg config.Config, st *store.Store) (venue.Venue, error) {
	switch strings.TrimSpace(strings.ToLower(cfg.Venue)) {
	case "", "fake":
		return fake.NewStoreBacked(st), nil
	case "polymarket_paper":
		return polymarketpaper.New(polymarketpaper.Config{
			Bin:           cfg.PolymarketPaperBin,
			AccountPrefix: cfg.PolymarketPaperAccountPrefix,
			Timeout:       cfg.PolymarketPaperTimeout,
			DataDir:       cfg.PolymarketPaperDataDir,
		})
	default:
		return nil, fmt.Errorf("unsupported ARENA_VENUE %q; expected fake or polymarket_paper", cfg.Venue)
	}
}

func (a *App) Close() error {
	if a.Cache != nil {
		_ = a.Cache.Close()
	}
	if a.DB != nil {
		return a.DB.Close()
	}
	return nil
}
