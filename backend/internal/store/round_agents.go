package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) LockRoundAgent(ctx context.Context, input RoundAgentInput) (RoundAgent, error) {
	if input.RoundID <= 0 {
		return RoundAgent{}, fmt.Errorf("%w: round_id is required", ErrValidation)
	}
	if input.AgentID <= 0 {
		return RoundAgent{}, fmt.Errorf("%w: agent_id is required", ErrValidation)
	}
	if input.MetadataJSON == "" {
		input.MetadataJSON = "{}"
	}
	if input.LockedBy == "" {
		input.LockedBy = "admin"
	}
	agent, err := s.GetAgent(ctx, input.AgentID)
	if err != nil {
		return RoundAgent{}, err
	}
	now := Now()
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO round_agents(round_id, team_id, agent_id, commit_sha, docker_image, metadata_json, locked_by, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(round_id, team_id) DO UPDATE SET
			agent_id = excluded.agent_id,
			commit_sha = excluded.commit_sha,
			docker_image = excluded.docker_image,
			metadata_json = excluded.metadata_json,
			locked_by = excluded.locked_by,
			updated_at = excluded.updated_at
	`, input.RoundID, agent.TeamID, input.AgentID, input.CommitSHA, input.DockerImage, input.MetadataJSON, input.LockedBy, now, now)
	if err != nil {
		return RoundAgent{}, err
	}
	return s.GetRoundAgentForTeam(ctx, input.RoundID, agent.TeamID)
}

func (s *Store) CheckRoundAgentLocks(ctx context.Context, roundID int64) (RoundAgentLockPreflight, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT
			t.id,
			t.slug,
			t.is_active,
			COALESCE(ra.agent_id, 0),
			COALESCE(a.slug, ''),
			COALESCE(a.status, ''),
			COALESCE(a.team_id, 0)
		FROM round_teams rt
		JOIN teams t ON t.id = rt.team_id
		LEFT JOIN round_agents ra ON ra.round_id = ? AND ra.team_id = t.id
		LEFT JOIN agents a ON a.id = ra.agent_id
		WHERE rt.round_id = ? AND rt.status = 'active'
		ORDER BY t.slug
	`, roundID, roundID)
	if err != nil {
		return RoundAgentLockPreflight{}, err
	}
	defer rows.Close()
	result := RoundAgentLockPreflight{
		MissingTeams: []string{},
		InvalidTeams: []RoundAgentLockIssue{},
	}
	for rows.Next() {
		var teamID, teamIsActive, agentID, agentTeamID int64
		var teamSlug, agentSlug, agentStatus string
		if err := rows.Scan(&teamID, &teamSlug, &teamIsActive, &agentID, &agentSlug, &agentStatus, &agentTeamID); err != nil {
			return RoundAgentLockPreflight{}, err
		}
		result.ActiveEnrolledTeamCount++
		if !scanBool(teamIsActive) {
			result.InvalidTeams = append(result.InvalidTeams, RoundAgentLockIssue{TeamID: teamID, TeamSlug: teamSlug, AgentID: agentID, AgentSlug: agentSlug, Reason: "enrolled team is paused globally"})
			continue
		}
		if agentID == 0 {
			result.MissingTeams = append(result.MissingTeams, teamSlug)
			continue
		}
		if agentTeamID != teamID {
			result.InvalidTeams = append(result.InvalidTeams, RoundAgentLockIssue{TeamID: teamID, TeamSlug: teamSlug, AgentID: agentID, AgentSlug: agentSlug, Reason: "locked agent belongs to a different team"})
			continue
		}
		if agentStatus != "active" {
			result.InvalidTeams = append(result.InvalidTeams, RoundAgentLockIssue{TeamID: teamID, TeamSlug: teamSlug, AgentID: agentID, AgentSlug: agentSlug, Reason: "locked agent is not active"})
		}
	}
	return result, rows.Err()
}

func (s *Store) GetRoundAgent(ctx context.Context, id int64) (RoundAgent, error) {
	row := s.db.QueryRowContext(ctx, roundAgentSelect()+`
		WHERE ra.id = ?
	`, id)
	item, err := scanRoundAgent(row)
	return item, normalizeErr(err)
}

func (s *Store) GetRoundAgentForTeam(ctx context.Context, roundID, teamID int64) (RoundAgent, error) {
	row := s.db.QueryRowContext(ctx, roundAgentSelect()+`
		WHERE ra.round_id = ? AND ra.team_id = ?
	`, roundID, teamID)
	item, err := scanRoundAgent(row)
	return item, normalizeErr(err)
}

func (s *Store) ListRoundAgents(ctx context.Context, roundID int64) ([]RoundAgent, error) {
	rows, err := s.db.QueryContext(ctx, roundAgentSelect()+`
		WHERE ra.round_id = ?
		ORDER BY t.slug
	`, roundID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []RoundAgent{}
	for rows.Next() {
		item, err := scanRoundAgent(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) RoundAgentLocked(ctx context.Context, roundID, agentID int64) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `
		SELECT id
		FROM round_agents
		WHERE round_id = ? AND agent_id = ?
	`, roundID, agentID).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

func roundAgentSelect() string {
	return `
		SELECT
			ra.id,
			ra.round_id,
			r.slug,
			ra.team_id,
			t.slug,
			ra.agent_id,
			a.slug,
			ra.commit_sha,
			ra.docker_image,
			ra.metadata_json,
			ra.locked_by,
			ra.created_at,
			ra.updated_at
		FROM round_agents ra
		JOIN rounds r ON r.id = ra.round_id
		JOIN teams t ON t.id = ra.team_id
		JOIN agents a ON a.id = ra.agent_id
	`
}

type roundAgentScanner interface {
	Scan(dest ...interface{}) error
}

func scanRoundAgent(row roundAgentScanner) (RoundAgent, error) {
	var item RoundAgent
	err := row.Scan(&item.ID, &item.RoundID, &item.RoundSlug, &item.TeamID, &item.TeamSlug, &item.AgentID, &item.AgentSlug, &item.CommitSHA, &item.DockerImage, &item.MetadataJSON, &item.LockedBy, &item.CreatedAt, &item.UpdatedAt)
	return item, err
}
