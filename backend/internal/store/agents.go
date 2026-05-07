package store

import (
	"context"
	"database/sql"
	"fmt"
)

func (s *Store) CreateAgent(ctx context.Context, input AgentInput, tokenHash string) (Agent, error) {
	if err := validateSlug(input.Slug); err != nil {
		return Agent{}, err
	}
	if input.TeamID <= 0 {
		return Agent{}, fmt.Errorf("%w: team_id is required", ErrValidation)
	}
	if input.Name == "" {
		input.Name = input.Slug
	}
	if input.Status == "" {
		input.Status = "active"
	}
	if input.Kind == "" {
		input.Kind = "agent"
	}
	if input.MetadataJSON == "" {
		input.MetadataJSON = "{}"
	}
	now := Now()
	res, err := s.db.ExecContext(ctx, `
		INSERT INTO agents(team_id, slug, name, api_token_hash, status, kind, repo_url, commit_sha, docker_image, metadata_json, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, input.TeamID, input.Slug, input.Name, tokenHash, input.Status, input.Kind, input.RepoURL, input.CommitSHA, input.DockerImage, input.MetadataJSON, now, now)
	if err != nil {
		return Agent{}, err
	}
	id, _ := res.LastInsertId()
	return s.GetAgent(ctx, id)
}

func (s *Store) GetAgent(ctx context.Context, id int64) (Agent, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT a.id, a.team_id, t.slug, a.slug, a.name, a.api_token_hash, a.status, a.kind, a.repo_url, a.commit_sha, a.docker_image, a.metadata_json, a.created_at, a.updated_at
		FROM agents a
		JOIN teams t ON t.id = a.team_id
		WHERE a.id = ?
	`, id)
	agent, err := scanAgent(row)
	return agent, normalizeErr(err)
}

func (s *Store) FindAgentByTokenHash(ctx context.Context, tokenHash string) (Agent, Team, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT
			a.id, a.team_id, t.slug, a.slug, a.name, a.api_token_hash, a.status, a.kind, a.repo_url, a.commit_sha, a.docker_image, a.metadata_json, a.created_at, a.updated_at,
			t.id, t.slug, t.name, t.api_token_hash, t.is_active, t.created_at, t.updated_at
		FROM agents a
		JOIN teams t ON t.id = a.team_id
		WHERE a.api_token_hash = ?
	`, tokenHash)
	var agent Agent
	var team Team
	var active int64
	err := row.Scan(
		&agent.ID, &agent.TeamID, &agent.TeamSlug, &agent.Slug, &agent.Name, &agent.APITokenHash, &agent.Status, &agent.Kind, &agent.RepoURL, &agent.CommitSHA, &agent.DockerImage, &agent.MetadataJSON, &agent.CreatedAt, &agent.UpdatedAt,
		&team.ID, &team.Slug, &team.Name, &team.APITokenHash, &active, &team.CreatedAt, &team.UpdatedAt,
	)
	if err != nil {
		return Agent{}, Team{}, normalizeErr(err)
	}
	team.IsActive = scanBool(active)
	return agent, team, nil
}

func (s *Store) ListTeamAgents(ctx context.Context, teamID int64) ([]Agent, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT a.id, a.team_id, t.slug, a.slug, a.name, a.api_token_hash, a.status, a.kind, a.repo_url, a.commit_sha, a.docker_image, a.metadata_json, a.created_at, a.updated_at
		FROM agents a
		JOIN teams t ON t.id = a.team_id
		WHERE a.team_id = ?
		ORDER BY a.id
	`, teamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	agents := []Agent{}
	for rows.Next() {
		agent, err := scanAgent(rows)
		if err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func (s *Store) SetAgentStatus(ctx context.Context, id int64, status string) (Agent, error) {
	if status != "active" && status != "paused" && status != "revoked" {
		return Agent{}, fmt.Errorf("%w: invalid agent status", ErrValidation)
	}
	_, err := s.db.ExecContext(ctx, "UPDATE agents SET status = ?, updated_at = ? WHERE id = ?", status, Now(), id)
	if err != nil {
		return Agent{}, err
	}
	return s.GetAgent(ctx, id)
}

func (s *Store) UpdateAgentTokenHash(ctx context.Context, id int64, tokenHash string) (Agent, error) {
	_, err := s.db.ExecContext(ctx, "UPDATE agents SET api_token_hash = ?, updated_at = ? WHERE id = ?", tokenHash, Now(), id)
	if err != nil {
		return Agent{}, err
	}
	return s.GetAgent(ctx, id)
}

type agentScanner interface {
	Scan(dest ...interface{}) error
}

func scanAgent(row agentScanner) (Agent, error) {
	var agent Agent
	var repoURL, commitSHA, dockerImage sql.NullString
	err := row.Scan(&agent.ID, &agent.TeamID, &agent.TeamSlug, &agent.Slug, &agent.Name, &agent.APITokenHash, &agent.Status, &agent.Kind, &repoURL, &commitSHA, &dockerImage, &agent.MetadataJSON, &agent.CreatedAt, &agent.UpdatedAt)
	agent.RepoURL = nullString(repoURL)
	agent.CommitSHA = nullString(commitSHA)
	agent.DockerImage = nullString(dockerImage)
	return agent, err
}
