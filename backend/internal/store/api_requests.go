package store

import "context"

func (s *Store) CreateAPIRequest(ctx context.Context, input APIRequestInput) error {
	if input.Method == "" {
		input.Method = "UNKNOWN"
	}
	if input.Path == "" {
		input.Path = "/"
	}
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO api_requests(team_id, agent_id, method, path, status, rate_limited, ip_hash, user_agent_hash, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, ptrToNullInt64(input.TeamID), ptrToNullInt64(input.AgentID), input.Method, input.Path, input.Status, boolInt(input.RateLimited), input.IPHash, input.UserAgentHash, Now())
	return err
}

func (s *Store) DeleteAPIRequestsBefore(ctx context.Context, cutoff string) (int64, error) {
	res, err := s.db.ExecContext(ctx, `
		DELETE FROM api_requests
		WHERE created_at < ?
	`, cutoff)
	if err != nil {
		return 0, err
	}
	count, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}
	return count, nil
}
