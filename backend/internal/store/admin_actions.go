package store

import (
	"context"
	"database/sql"
)

func (s *Store) CreateAdminAction(ctx context.Context, action, actor string, roundID, teamID *int64, metadataJSON string) (AdminAction, error) {
	if actor == "" {
		actor = "admin"
	}
	if metadataJSON == "" {
		metadataJSON = "{}"
	}
	now := Now()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO admin_actions(round_id, team_id, action, actor, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, ptrToNullInt64(roundID), ptrToNullInt64(teamID), action, actor, metadataJSON, now)
	if err != nil {
		return AdminAction{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetAdminAction(ctx, id)
}

func (s *Store) GetAdminAction(ctx context.Context, id int64) (AdminAction, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, action, actor, metadata_json, created_at
		FROM admin_actions
		WHERE id = ?
	`, id)
	action, err := scanAdminAction(row)
	return action, normalizeErr(err)
}

type adminActionScanner interface {
	Scan(dest ...interface{}) error
}

func scanAdminAction(row adminActionScanner) (AdminAction, error) {
	var action AdminAction
	var roundID, teamID sql.NullInt64
	err := row.Scan(&action.ID, &roundID, &teamID, &action.Action, &action.Actor, &action.MetadataJSON, &action.CreatedAt)
	action.RoundID = nullInt64Ptr(roundID)
	action.TeamID = nullInt64Ptr(teamID)
	return action, err
}
