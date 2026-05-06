package httpapi

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/chenrui333/prediction-agent-arena/backend/internal/cache"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/events"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/risk"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/store"
	"github.com/chenrui333/prediction-agent-arena/backend/internal/venue"
)

type Server struct {
	Store          *store.Store
	Venue          venue.Venue
	Cache          *cache.Client
	Events         *events.Writer
	Policy         risk.Policy
	AdminToken     string
	Logger         *slog.Logger
	LeaderboardTTL time.Duration
	ExportDir      string
	CORSOrigin     string
}

func (s *Server) Router() http.Handler {
	r := chi.NewRouter()
	r.Use(s.recoverer)
	r.Use(s.cors)

	r.Get("/health", s.health)
	r.Route("/api/v1", func(r chi.Router) {
		r.Get("/markets", s.listMarkets)
		r.Get("/markets/{market_id}", s.getMarket)
		r.Get("/leaderboard", s.leaderboard)
		r.Get("/teams/{team_slug}", s.getTeamActivity)

		r.Group(func(r chi.Router) {
			r.Use(s.studentAuth)
			r.Get("/portfolio", s.getPortfolio)
			r.Post("/heartbeat", s.postHeartbeat)
			r.Post("/decisions", s.postDecision)
			r.Post("/orders", s.postOrder)
			r.Post("/orders/{order_id}/cancel", s.cancelOrder)
			r.Get("/fills", s.listFills)
		})

		r.Route("/admin", func(r chi.Router) {
			r.Use(s.adminAuth)
			r.Get("/health", s.adminHealth)
			r.Get("/summary", s.adminSummary)
			r.Post("/snapshots/compact", s.compactSnapshots)
			r.Post("/teams", s.createTeam)
			r.Get("/teams", s.listTeams)
			r.Post("/teams/{team_id}/pause", s.pauseTeam)
			r.Post("/teams/{team_id}/resume", s.resumeTeam)
			r.Post("/teams/{team_id}/rotate-token", s.rotateTeamToken)
			r.Post("/teams/{team_id}/reset", s.resetTeam)
			r.Post("/rounds", s.createRound)
			r.Get("/rounds", s.listRounds)
			r.Post("/rounds/{round_id}/activate", s.activateRound)
			r.Post("/rounds/{round_id}/pause", s.pauseRound)
			r.Post("/rounds/{round_id}/complete", s.completeRound)
			r.Post("/rounds/{round_id}/reset", s.resetRound)
			r.Post("/rounds/{round_id}/settle", s.settleRound)
			r.Post("/rounds/{round_id}/teams/{team_id}/reset", s.resetTeamRound)
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
