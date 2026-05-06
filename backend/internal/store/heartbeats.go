package store

import (
	"context"
	"database/sql"
)

func (s *Store) CreateHeartbeat(ctx context.Context, roundID, teamID int64, agentID *int64, status, metadataJSON string) (AgentHeartbeat, error) {
	if status == "" {
		status = "online"
	}
	if metadataJSON == "" {
		metadataJSON = "{}"
	}
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO agent_heartbeats(round_id, team_id, agent_id, status, metadata_json, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, roundID, teamID, ptrToNullInt64(agentID), status, metadataJSON, Now())
	if err != nil {
		return AgentHeartbeat{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetHeartbeat(ctx, id)
}

func (s *Store) GetHeartbeat(ctx context.Context, id int64) (AgentHeartbeat, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, round_id, team_id, agent_id, status, metadata_json, created_at
		FROM agent_heartbeats
		WHERE id = ?
	`, id)
	var hb AgentHeartbeat
	var agentID sql.NullInt64
	err := row.Scan(&hb.ID, &hb.RoundID, &hb.TeamID, &agentID, &hb.Status, &hb.MetadataJSON, &hb.CreatedAt)
	hb.AgentID = nullInt64Ptr(agentID)
	return hb, normalizeErr(err)
}
