package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) EnrollRoundTeam(ctx context.Context, input RoundTeamInput) (RoundTeam, error) {
	if input.RoundID <= 0 {
		return RoundTeam{}, fmt.Errorf("%w: round_id is required", ErrValidation)
	}
	if input.TeamID <= 0 {
		return RoundTeam{}, fmt.Errorf("%w: team_id is required", ErrValidation)
	}
	status := input.Status
	if status == "" {
		status = "active"
	}
	if err := validateRoundTeamStatus(status); err != nil {
		return RoundTeam{}, err
	}
	now := Now()
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO round_teams(round_id, team_id, status, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(round_id, team_id) DO UPDATE SET
			status = excluded.status,
			updated_at = excluded.updated_at
	`, input.RoundID, input.TeamID, status, now, now)
	if err != nil {
		return RoundTeam{}, err
	}
	return s.GetRoundTeam(ctx, input.RoundID, input.TeamID)
}

func (s *Store) SetRoundTeamStatus(ctx context.Context, roundID, teamID int64, status string) (RoundTeam, error) {
	if err := validateRoundTeamStatus(status); err != nil {
		return RoundTeam{}, err
	}
	result, err := s.db.ExecContext(ctx, `
		UPDATE round_teams
		SET status = ?, updated_at = ?
		WHERE round_id = ? AND team_id = ?
	`, status, Now(), roundID, teamID)
	if err != nil {
		return RoundTeam{}, err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return RoundTeam{}, ErrNotFound
	}
	return s.GetRoundTeam(ctx, roundID, teamID)
}

func (s *Store) GetRoundTeam(ctx context.Context, roundID, teamID int64) (RoundTeam, error) {
	row := s.db.QueryRowContext(ctx, roundTeamSelect()+`
		WHERE rt.round_id = ? AND rt.team_id = ?
	`, roundID, teamID)
	item, err := scanRoundTeam(row)
	return item, normalizeErr(err)
}

func (s *Store) ListRoundTeams(ctx context.Context, roundID int64) ([]RoundTeam, error) {
	rows, err := s.db.QueryContext(ctx, roundTeamSelect()+`
		WHERE rt.round_id = ?
		ORDER BY t.slug
	`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RoundTeam{}
	for rows.Next() {
		item, err := scanRoundTeam(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) CountRoundTeams(ctx context.Context, roundID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM round_teams WHERE round_id = ?", roundID).Scan(&count)
	return count, err
}

func (s *Store) CountActiveRoundTeams(ctx context.Context, roundID int64) (int64, error) {
	var count int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM round_teams rt
		JOIN teams t ON t.id = rt.team_id
		WHERE rt.round_id = ? AND rt.status = 'active' AND t.is_active = 1
	`, roundID).Scan(&count)
	return count, err
}

func validateRoundTeamStatus(status string) error {
	switch status {
	case "active", "paused", "withdrawn":
		return nil
	default:
		return fmt.Errorf("%w: round team status must be active, paused, or withdrawn", ErrValidation)
	}
}

func roundTeamSelect() string {
	return `
		SELECT
			rt.round_id,
			r.slug,
			rt.team_id,
			t.slug,
			t.name,
			t.is_active,
			rt.status,
			rt.created_at,
			rt.updated_at
		FROM round_teams rt
		JOIN rounds r ON r.id = rt.round_id
		JOIN teams t ON t.id = rt.team_id
	`
}

type roundTeamScanner interface {
	Scan(dest ...interface{}) error
}

func scanRoundTeam(row roundTeamScanner) (RoundTeam, error) {
	var item RoundTeam
	var teamIsActive int64
	err := row.Scan(&item.RoundID, &item.RoundSlug, &item.TeamID, &item.TeamSlug, &item.TeamName, &teamIsActive, &item.Status, &item.CreatedAt, &item.UpdatedAt)
	item.TeamIsActive = scanBool(teamIsActive)
	if err == sql.ErrNoRows {
		return item, ErrNotFound
	}
	return item, err
}
