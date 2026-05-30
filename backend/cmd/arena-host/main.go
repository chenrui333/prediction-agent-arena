package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
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
		logger.Error("failed to initialize hosted app", "error", err)
		os.Exit(1)
	}
	defer arena.Close()

	server := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: arena.API.Router(),
	}
	errCh := make(chan error, 2)

	go func() {
		logger.Info("arena api listening", "addr", cfg.HTTPAddr)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- fmt.Errorf("http server failed: %w", err)
		}
	}()

	go func() {
		w := &worker.SnapshotWorker{Store: arena.Store, Events: arena.Events, Interval: cfg.SnapshotInterval, Logger: logger}
		logger.Info("arena worker started", "interval", cfg.SnapshotInterval.String())
		if err := w.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			errCh <- fmt.Errorf("snapshot worker failed: %w", err)
		}
	}()

	failed := false
	select {
	case <-ctx.Done():
	case err := <-errCh:
		failed = true
		logger.Error("hosted app failed", "error", err)
		stop()
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown failed", "error", err)
		failed = true
	}
	if failed {
		os.Exit(1)
	}
}
