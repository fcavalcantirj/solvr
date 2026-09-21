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
}
