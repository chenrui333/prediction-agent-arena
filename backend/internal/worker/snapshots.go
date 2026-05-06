package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/events"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
)

type SnapshotWorker struct {
	Store    *store.Store
	Events   *events.Writer
	Interval time.Duration
	Logger   *slog.Logger
}

func (w *SnapshotWorker) Run(ctx context.Context) error {
	interval := w.Interval
	if interval <= 0 {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if err := w.tick(ctx); err != nil && w.Logger != nil {
			w.Logger.Warn("snapshot worker tick failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (w *SnapshotWorker) tick(ctx context.Context) error {
	round, err := w.Store.GetActiveRound(ctx)
	if err != nil {
		return nil
	}
	ticks, err := w.Store.AdvanceRoundSimulatedMarkets(ctx, round.ID)
	if err != nil {
		return err
	}
	if w.Events != nil {
		for _, tick := range ticks {
			_ = w.Events.Append(ctx, round.Slug, "market", "market_price_tick", tick)
		}
	}
	fills, err := w.Store.FillOpenOrders(ctx, round.ID)
	if err != nil {
		return err
	}
	if w.Events != nil {
		for _, result := range fills {
			_ = w.Events.Append(ctx, result.RoundSlug, result.TeamSlug, "fill", result.Fill)
		}
	}
	teams, err := w.Store.ListTeams(ctx)
	if err != nil {
		return err
	}
	for _, team := range teams {
		portfolio, err := w.Store.CreatePortfolioSnapshot(ctx, round.ID, team.ID)
		if err != nil {
			return err
		}
		score, err := w.Store.RefreshScore(ctx, round.ID, team.ID)
		if err != nil {
			return err
		}
		if w.Events != nil {
			_ = w.Events.Append(ctx, round.Slug, team.Slug, "portfolio_snapshot", portfolio)
			_ = w.Events.Append(ctx, round.Slug, team.Slug, "score_snapshot", score)
		}
	}
	if err := w.Store.RecordWorkerHeartbeat(ctx, "snapshot_worker", map[string]interface{}{
		"round_id":         round.ID,
		"round_slug":       round.Slug,
		"price_tick_count": len(ticks),
		"fill_count":       len(fills),
	}); err != nil {
		return err
	}
	return nil
}
