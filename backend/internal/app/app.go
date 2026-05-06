package app

import (
	"context"
	"database/sql"
	"log/slog"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/cache"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/config"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/db"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/events"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/httpapi"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/risk"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue/fake"
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
	api := &httpapi.Server{
		Store:          st,
		Venue:          fake.NewStoreBacked(st),
		Cache:          redisClient,
		Events:         eventWriter,
		Policy:         policy,
		AdminToken:     cfg.AdminToken,
		Logger:         logger,
		LeaderboardTTL: cfg.LeaderboardTTL,
		ExportDir:      cfg.ExportDir,
		CORSOrigin:     cfg.FrontendOrigin,
	}
	return &App{Config: cfg, DB: conn, Store: st, Cache: redisClient, Events: eventWriter, API: api}, nil
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
