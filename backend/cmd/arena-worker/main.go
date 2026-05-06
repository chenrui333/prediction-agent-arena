package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/app"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/config"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))
	cfg := config.Load()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	arena, err := app.New(ctx, cfg, logger)
	if err != nil {
		logger.Error("failed to initialize worker", "error", err)
		os.Exit(1)
	}
	defer arena.Close()

	w := &worker.SnapshotWorker{Store: arena.Store, Events: arena.Events, Interval: cfg.SnapshotInterval, Logger: logger}
	logger.Info("arena worker started", "interval", cfg.SnapshotInterval.String())
	if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
		logger.Error("arena worker failed", "error", err)
		os.Exit(1)
	}
}
