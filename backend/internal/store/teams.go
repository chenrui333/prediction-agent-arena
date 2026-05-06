package store

import (
	"context"
)

func (s *Store) CreateTeam(ctx context.Context, slug, name, tokenHash string) (Team, error) {
	if err := validateSlug(slug); err != nil {
		return Team{}, err
	}
	now := Now()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO teams(slug, name, api_token_hash, is_active, created_at, updated_at)
		VALUES (?, ?, ?, 1, ?, ?)
	`, slug, name, tokenHash, now, now)
	if err != nil {
		return Team{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetTeam(ctx, id)
}

func (s *Store) ListTeams(ctx context.Context) ([]Team, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, api_token_hash, is_active, created_at, updated_at
		FROM teams
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	teams := []Team{}
	for rows.Next() {
		team, err := scanTeam(rows)
		if err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, rows.Err()
}

func (s *Store) GetTeam(ctx context.Context, id int64) (Team, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, api_token_hash, is_active, created_at, updated_at
		FROM teams
		WHERE id = ?
	`, id)
	team, err := scanTeam(row)
	return team, normalizeErr(err)
}

func (s *Store) GetTeamBySlug(ctx context.Context, slug string) (Team, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, api_token_hash, is_active, created_at, updated_at
		FROM teams
		WHERE slug = ?
	`, slug)
	team, err := scanTeam(row)
	return team, normalizeErr(err)
}

func (s *Store) FindTeamByTokenHash(ctx context.Context, tokenHash string) (Team, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, api_token_hash, is_active, created_at, updated_at
		FROM teams
		WHERE api_token_hash = ?
	`, tokenHash)
	team, err := scanTeam(row)
	return team, normalizeErr(err)
}

func (s *Store) SetTeamActive(ctx context.Context, id int64, active bool) (Team, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE teams SET is_active = ?, updated_at = ? WHERE id = ?", boolInt(active), Now(), id)
	if err != nil {
		return Team{}, err
	}
	return s.GetTeam(ctx, id)
}

func (s *Store) UpdateTeamTokenHash(ctx context.Context, id int64, tokenHash string) (Team, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE teams SET api_token_hash = ?, updated_at = ? WHERE id = ?", tokenHash, Now(), id)
	if err != nil {
		return Team{}, err
	}
	return s.GetTeam(ctx, id)
}

func (s *Store) ResetTeamRound(ctx context.Context, roundID, teamID int64) error {
	return s.WithTx(ctx, func(tx *Tx) error {
		tables := []string{"settlements", "agent_heartbeats", "score_snapshots", "risk_events", "positions", "fills", "orders", "decisions", "portfolio_snapshots"}
		for _, table := range tables {
			if _, err := tx.tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE round_id = ? AND team_id = ?", roundID, teamID); err != nil {
				return err
			}
		}
		return nil
	})
}

func (s *Store) ResetTeam(ctx context.Context, teamID int64) error {
	return s.WithTx(ctx, func(tx *Tx) error {
		tables := []string{"settlements", "agent_heartbeats", "score_snapshots", "risk_events", "positions", "fills", "orders", "decisions", "portfolio_snapshots"}
		for _, table := range tables {
			if _, err := tx.tx.ExecContext(ctx, "DELETE FROM "+table+" WHERE team_id = ?", teamID); err != nil {
				return err
			}
		}
		return nil
	})
}

type teamScanner interface {
	Scan(dest ...interface{}) error
}

func scanTeam(row teamScanner) (Team, error) {
	var team Team
	var active int64
	err := row.Scan(&team.ID, &team.Slug, &team.Name, &team.APITokenHash, &active, &team.CreatedAt, &team.UpdatedAt)
	team.IsActive = scanBool(active)
	return team, err
}
