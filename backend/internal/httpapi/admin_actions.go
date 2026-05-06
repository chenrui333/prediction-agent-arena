package httpapi

import (
	"context"
	"encoding/json"
)

func (s *Server) recordAdminAction(ctx context.Context, roundSlug, action string, roundID, teamID *int64, metadata map[string]interface{}) {
	if metadata == nil {
		metadata = map[string]interface{}{}
	}
	metadata["action"] = action
	if roundID != nil {
		metadata["round_id"] = *roundID
	}
	if teamID != nil {
		metadata["team_id"] = *teamID
	}
	raw, err := json.Marshal(metadata)
	if err != nil {
		raw = []byte(`{}`)
	}
	if _, err := s.Store.CreateAdminAction(ctx, action, "admin", roundID, teamID, string(raw)); err != nil {
		s.logWarn("admin action audit failed", "action", action, "error", err)
	}
	if roundSlug == "" {
		roundSlug = "admin"
	}
	_ = s.Events.Append(ctx, roundSlug, "admin", "admin_action", metadata)
}
