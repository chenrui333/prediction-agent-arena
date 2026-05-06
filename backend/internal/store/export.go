package store

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

type ExportResult struct {
	RoundSlug      string   `json:"round_slug"`
	ExportDir      string   `json:"export_dir"`
	Artifacts      []string `json:"artifacts"`
	LeaderboardCSV string   `json:"leaderboard_csv"`
	ScoresJSONL    string   `json:"scores_jsonl"`
}

func (s *Store) ExportRound(ctx context.Context, roundID int64, exportRoot string) (ExportResult, error) {
	round, err := s.GetRound(ctx, roundID)
	if err != nil {
		return ExportResult{}, err
	}
	rows, err := s.ListLeaderboard(ctx, roundID)
	if err != nil {
		return ExportResult{}, err
	}
	dir := filepath.Join(exportRoot, round.Slug)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return ExportResult{}, err
	}
	leaderboardPath := filepath.Join(dir, "leaderboard.csv")
	if err := writeLeaderboardCSV(leaderboardPath, rows); err != nil {
		return ExportResult{}, err
	}
	scoresPath := filepath.Join(dir, "scores.jsonl")
	if err := writeScoresJSONL(scoresPath, rows); err != nil {
		return ExportResult{}, err
	}
	return ExportResult{
		RoundSlug:      round.Slug,
		ExportDir:      dir,
		Artifacts:      []string{leaderboardPath, scoresPath},
		LeaderboardCSV: leaderboardPath,
		ScoresJSONL:    scoresPath,
	}, nil
}

func writeLeaderboardCSV(path string, rows []LeaderboardRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	writer := csv.NewWriter(file)
	defer writer.Flush()
	if err := writer.Write([]string{"rank", "team_slug", "team_name", "composite_score", "equity_cents", "return_bps", "max_drawdown_bps", "brier_score_bps", "trade_count", "gross_exposure_cents", "last_heartbeat", "status"}); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writer.Write([]string{
			strconv.FormatInt(row.Rank, 10),
			row.TeamSlug,
			row.TeamName,
			strconv.FormatInt(row.CompositeScore, 10),
			strconv.FormatInt(row.EquityCents, 10),
			strconv.FormatInt(row.ReturnBPS, 10),
			strconv.FormatInt(row.MaxDrawdownBPS, 10),
			strconv.FormatInt(row.BrierScoreBPS, 10),
			strconv.FormatInt(row.TradeCount, 10),
			strconv.FormatInt(row.GrossExposureCents, 10),
			row.LastHeartbeat,
			row.Status,
		}); err != nil {
			return err
		}
	}
	return writer.Error()
}

func writeScoresJSONL(path string, rows []LeaderboardRow) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	encoder := json.NewEncoder(file)
	for _, row := range rows {
		if err := encoder.Encode(row); err != nil {
			return err
		}
	}
	return nil
}
