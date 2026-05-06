package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CompactResult struct {
	RoundID                   int64 `json:"round_id"`
	PortfolioSnapshotsDeleted int64 `json:"portfolio_snapshots_deleted"`
	ScoreSnapshotsDeleted     int64 `json:"score_snapshots_deleted"`
}

func (s *Store) RecordWorkerHeartbeat(ctx context.Context, service string, metadata map[string]interface{}) error {
	if service == "" {
		service = "worker"
	}
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		return err
	}
	now := Now()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO worker_heartbeats(service, last_seen_at, metadata_json, updated_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(service) DO UPDATE SET
			last_seen_at = excluded.last_seen_at,
			metadata_json = excluded.metadata_json,
			updated_at = excluded.updated_at
	`, service, now, string(raw), now)
	return err
}

func (s *Store) Health(ctx context.Context, redisOK bool) (ArenaHealth, error) {
	health := ArenaHealth{Status: "ok", RedisOK: redisOK}
	if err := s.db.PingContext(ctx); err != nil {
		health.Status = "error"
		health.DBOK = false
		return health, err
	}
	health.DBOK = true
	if !redisOK {
		health.Status = "degraded"
	}
	if round, err := s.GetActiveRound(ctx); err == nil {
		health.ActiveRoundID = round.ID
		health.ActiveRoundSlug = round.Slug
	}
	health.LatestMarketTickAt = s.latestTimestamp(ctx, "SELECT COALESCE(MAX(created_at), '') FROM market_price_ticks")
	health.LatestPortfolioSnapshotAt = s.latestTimestamp(ctx, "SELECT COALESCE(MAX(created_at), '') FROM portfolio_snapshots")
	health.LatestWorkerHeartbeatAt = s.latestTimestamp(ctx, "SELECT COALESCE(MAX(last_seen_at), '') FROM worker_heartbeats")
	return health, nil
}

func (s *Store) latestTimestamp(ctx context.Context, query string) string {
	var value sql.NullString
	if err := s.db.QueryRowContext(ctx, query).Scan(&value); err != nil || !value.Valid {
		return ""
	}
	return value.String
}

func (s *Store) CompactSnapshots(ctx context.Context, roundID int64, keepEvery time.Duration) (CompactResult, error) {
	if keepEvery <= 0 {
		keepEvery = 5 * time.Minute
	}
	var result CompactResult
	result.RoundID = roundID
	portfolioDeleted, err := s.compactSnapshotTable(ctx, "portfolio_snapshots", roundID, keepEvery)
	if err != nil {
		return result, err
	}
	scoreDeleted, err := s.compactSnapshotTable(ctx, "score_snapshots", roundID, keepEvery)
	if err != nil {
		return result, err
	}
	result.PortfolioSnapshotsDeleted = portfolioDeleted
	result.ScoreSnapshotsDeleted = scoreDeleted
	return result, nil
}

func (s *Store) compactSnapshotTable(ctx context.Context, table string, roundID int64, keepEvery time.Duration) (int64, error) {
	if table != "portfolio_snapshots" && table != "score_snapshots" {
		return 0, fmt.Errorf("%w: unsupported snapshot table", ErrValidation)
	}
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, team_id, created_at
		FROM `+table+`
		WHERE round_id = ?
		ORDER BY team_id, created_at, id
	`, roundID)
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	type snapshotRef struct {
		id        int64
		teamID    int64
		createdAt string
	}
	refs := []snapshotRef{}
	latestByTeam := map[int64]int64{}
	for rows.Next() {
		var ref snapshotRef
		if err := rows.Scan(&ref.id, &ref.teamID, &ref.createdAt); err != nil {
			return 0, err
		}
		refs = append(refs, ref)
		latestByTeam[ref.teamID] = ref.id
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}
	seenBuckets := map[string]bool{}
	deleteIDs := []int64{}
	for _, ref := range refs {
		if latestByTeam[ref.teamID] == ref.id {
			continue
		}
		createdAt, err := time.Parse(time.RFC3339Nano, ref.createdAt)
		if err != nil {
			continue
		}
		bucket := createdAt.UnixNano() / keepEvery.Nanoseconds()
		key := strconv.FormatInt(ref.teamID, 10) + ":" + strconv.FormatInt(bucket, 10)
		if seenBuckets[key] {
			deleteIDs = append(deleteIDs, ref.id)
			continue
		}
		seenBuckets[key] = true
	}
	if len(deleteIDs) == 0 {
		return 0, nil
	}
	placeholders := make([]string, 0, len(deleteIDs))
	args := make([]interface{}, 0, len(deleteIDs))
	for _, id := range deleteIDs {
		placeholders = append(placeholders, "?")
		args = append(args, id)
	}
	res, err := s.db.ExecContext(ctx, "DELETE FROM "+table+" WHERE id IN ("+strings.Join(placeholders, ",")+")", args...)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}
