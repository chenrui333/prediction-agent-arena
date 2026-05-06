package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/cache"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/config"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/events"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/risk"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

type Server struct {
	Store              *store.Store
	Venue              venue.Venue
	Cache              *cache.Client
	Events             *events.Writer
	Policy             risk.Policy
	AdminToken         string
	Logger             *slog.Logger
	LeaderboardTTL     time.Duration
	ExportDir          string
	CORSOrigins        []string
	PublicTeamActivity string
	LegacyTeamAuth     bool
	AuditSalt          string
	TrustProxyHeaders  bool
	TrustedProxyCIDRs  []string
	RateLimits         config.RateLimits
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(s.recoverer)
	r.Use(s.cors)

	r.Get("/health", s.health)
	r.Route("/api/v1", func(r chi.Router) {
		r.With(s.rateLimitPublicRead).Get("/markets", s.listMarkets)
		r.With(s.rateLimitPublicRead).Get("/markets/{market_id}", s.getMarket)
		r.With(s.rateLimitPublicRead).Get("/leaderboard", s.leaderboard)
		r.With(s.rateLimitPublicRead).Get("/teams/{team_slug}", s.getTeamActivity)

		r.Group(func(r chi.Router) {
			r.Use(s.studentAuth)
			r.Use(s.auditRequests)
			r.With(s.rateLimitStudentRead).Get("/me", s.me)
			r.With(s.rateLimitStudentRead).Get("/portfolio", s.getPortfolio)
			r.With(s.requireRoundAgentIfLocked, s.rateLimitHeartbeat).Post("/heartbeat", s.postHeartbeat)
			r.With(s.requireActiveAgentMutation, s.requireRoundAgentIfLocked, s.rateLimitDecision).Post("/decisions", s.postDecision)
			r.With(s.requireActiveAgentMutation, s.requireRoundAgentIfLocked, s.rateLimitOrder).Post("/orders", s.postOrder)
			r.With(s.requireActiveAgentMutation, s.requireRoundAgentIfLocked, s.rateLimitOrder).Post("/orders/{order_id}/cancel", s.cancelOrder)
			r.With(s.rateLimitStudentRead).Get("/fills", s.listFills)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(s.adminAuth)
			r.Use(s.auditRequests)
			r.Use(s.rateLimitAdmin)
			r.Get("/health", s.adminHealth)
			r.Get("/summary", s.adminSummary)
			r.Post("/snapshots/compact", s.compactSnapshots)
			r.Post("/audit/compact", s.compactAudit)
			r.Post("/teams", s.createTeam)
			r.Get("/teams", s.listTeams)
			r.Post("/teams/{team_id}/agents", s.createAgent)
			r.Get("/teams/{team_id}/agents", s.listTeamAgents)
			r.Post("/teams/{team_id}/pause", s.pauseTeam)
			r.Post("/teams/{team_id}/resume", s.resumeTeam)
			r.Post("/teams/{team_id}/rotate-token", s.rotateTeamToken)
			r.Post("/teams/{team_id}/reset", s.resetTeam)
			r.Post("/agents/{agent_id}/pause", s.pauseAgent)
			r.Post("/agents/{agent_id}/resume", s.resumeAgent)
			r.Post("/agents/{agent_id}/revoke", s.revokeAgent)
			r.Post("/agents/{agent_id}/rotate-token", s.rotateAgentToken)
			r.Post("/rounds", s.createRound)
			r.Get("/rounds", s.listRounds)
			r.Post("/rounds/{round_id}/activate", s.activateRound)
			r.Post("/rounds/{round_id}/pause", s.pauseRound)
			r.Post("/rounds/{round_id}/complete", s.completeRound)
			r.Post("/rounds/{round_id}/require-locked-agents", s.requireLockedAgentsRound)
			r.Post("/rounds/{round_id}/allow-unlocked-agents", s.allowUnlockedAgentsRound)
			r.Post("/rounds/{round_id}/reset", s.resetRound)
			r.Post("/rounds/{round_id}/settle", s.settleRound)
			r.Post("/rounds/{round_id}/teams/{team_id}/reset", s.resetTeamRound)
			r.Get("/rounds/{round_id}/teams/{team_id}/activity", s.getAdminTeamActivity)
			r.Post("/rounds/{round_id}/agents/{agent_id}/lock", s.lockRoundAgent)
			r.Get("/rounds/{round_id}/agents", s.listRoundAgents)
			r.Post("/rounds/{round_id}/freeze-leaderboard", s.freezeLeaderboard)
			r.Post("/markets", s.upsertMarket)
			r.Get("/markets", s.adminListMarkets)
			r.Post("/markets/{market_id}/resolve", s.resolveMarket)
			r.Post("/rounds/{round_id}/markets/{market_id}", s.allowMarket)
			r.Get("/export/{round_id}", s.exportRound)
		})
	})
	return r
}
