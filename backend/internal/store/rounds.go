package store

import (
	"context"
	"database/sql"
)

type RoundInput struct {
	Slug                string `json:"slug"`
	Name                string `json:"name"`
	Mode                string `json:"mode"`
	Status              string `json:"status"`
	InitialBalanceCents int64  `json:"initial_balance_cents"`
	StartsAt            string `json:"starts_at"`
	EndsAt              string `json:"ends_at"`
}

func (s *Store) CreateRound(ctx context.Context, input RoundInput) (Round, error) {
	if err := validateSlug(input.Slug); err != nil {
		return Round{}, err
	}
	if input.Name == "" {
		input.Name = input.Slug
	}
	if input.Mode == "" {
		input.Mode = "practice"
	}
	if input.Status == "" {
		input.Status = "draft"
	}
	if input.InitialBalanceCents <= 0 {
		input.InitialBalanceCents = 1000000
	}
	now := Now()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO rounds(slug, name, mode, status, initial_balance_cents, starts_at, ends_at, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.Slug, input.Name, input.Mode, input.Status, input.InitialBalanceCents, nullIfEmpty(input.StartsAt), nullIfEmpty(input.EndsAt), now, now)
	if err != nil {
		return Round{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetRound(ctx, id)
}

func (s *Store) ListRounds(ctx context.Context) ([]Round, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, slug, name, mode, status, initial_balance_cents, starts_at, ends_at, created_at, updated_at
		FROM rounds
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rounds := []Round{}
	for rows.Next() {
		round, err := scanRound(rows)
		if err != nil {
			return nil, err
		}
		rounds = append(rounds, round)
	}
	return rounds, rows.Err()
}

func (s *Store) GetRound(ctx context.Context, id int64) (Round, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, mode, status, initial_balance_cents, starts_at, ends_at, created_at, updated_at
		FROM rounds
		WHERE id = ?
	`, id)
	round, err := scanRound(row)
	return round, normalizeErr(err)
}

func (s *Store) GetRoundBySlug(ctx context.Context, slug string) (Round, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, mode, status, initial_balance_cents, starts_at, ends_at, created_at, updated_at
		FROM rounds
		WHERE slug = ?
	`, slug)
	round, err := scanRound(row)
	return round, normalizeErr(err)
}

func (s *Store) GetActiveRound(ctx context.Context) (Round, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, mode, status, initial_balance_cents, starts_at, ends_at, created_at, updated_at
		FROM rounds
		WHERE status = 'active'
		ORDER BY id DESC
		LIMIT 1
	`)
	round, err := scanRound(row)
	return round, normalizeErr(err)
}

func (s *Store) GetLatestRound(ctx context.Context) (Round, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, slug, name, mode, status, initial_balance_cents, starts_at, ends_at, created_at, updated_at
		FROM rounds
		ORDER BY id DESC
		LIMIT 1
	`)
	round, err := scanRound(row)
	return round, normalizeErr(err)
}

func (s *Store) SetRoundStatus(ctx context.Context, id int64, status string) (Round, error) {
	if status == "active" {
		if _, err := s.db.ExecContext(ctx, "UPDATE rounds SET status = 'paused', updated_at = ? WHERE status = 'active' AND id <> ?", Now(), id); err != nil {
			return Round{}, err
		}
	}
	_, err := s.db.ExecContext(ctx, "UPDATE rounds SET status = ?, updated_at = ? WHERE id = ?", status, Now(), id)
	if err != nil {
		return Round{}, err
	}
	return s.GetRound(ctx, id)
}

type roundScanner interface {
	Scan(dest ...interface{}) error
}

func scanRound(row roundScanner) (Round, error) {
	var round Round
	var startsAt, endsAt sql.NullString
	err := row.Scan(&round.ID, &round.Slug, &round.Name, &round.Mode, &round.Status, &round.InitialBalanceCents, &startsAt, &endsAt, &round.CreatedAt, &round.UpdatedAt)
	round.StartsAt = nullString(startsAt)
	round.EndsAt = nullString(endsAt)
	return round, err
}

func nullIfEmpty(value string) sql.NullString {
	if value == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: value, Valid: true}
}
