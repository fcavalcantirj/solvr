package api

import (
	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountHomepageRoutes registers the public homepage endpoints.
//
//	GET /v1/homepage/example  -- the planner/executor collaboration the homepage
//	                             shows as proof. Public, no auth: the whole point
//	                             is that a visitor sees real agents working before
//	                             they have an account.
//	GET /v1/homepage/overview -- the whole live overview the index renders: room
//	                             statistics, the public activity stream, the
//	                             editorial room previews, API usage, search
//	                             statistics and the all-time community totals.
//	GET /v1/homepage/activity -- the paginated Load more behind the stream.
//	GET /v1/homepage/rooms    -- the public room statistics alone, for the
//	                             shared 24h / 7d / 30d selector. Re-reading four
//	                             numbers and a series beats re-reading the index.
//	GET /v1/homepage/search   -- the search statistics alone, behind the same
//	                             24h / 7d / 30d selector.
//
// Kept out of router.go (which is already over the file-size limit) and out of
// mountRoomRoutes (which owns /v1/rooms and /r/{slug}).
func mountHomepageRoutes(r chi.Router, pool *db.Pool) {
	if pool == nil {
		return
	}

	exampleHandler := handlers.NewCollaborationExampleHandler(
		db.NewRoomRepository(pool),
		db.NewMessageRepository(pool),
		db.NewAgentPresenceRepository(pool),
	)
	r.Get("/v1/homepage/example", exampleHandler.GetExample)

	overviewHandler := handlers.NewHomepageOverviewHandler(
		db.NewHomepageRepository(pool),
		db.NewRoomRepository(pool),
		db.NewStatsRepository(pool),
		db.NewSearchAnalyticsRepository(pool),
		handlers.PreviewSlugsFromEnv(),
	)
	r.Get("/v1/homepage/overview", overviewHandler.GetOverview)
	r.Get("/v1/homepage/activity", overviewHandler.GetActivity)
	r.Get("/v1/homepage/rooms", overviewHandler.GetRooms)
	r.Get("/v1/homepage/search", overviewHandler.GetSearch)
}
