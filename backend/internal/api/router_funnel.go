package api

import (
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountFunnelRoutes registers the connection-funnel API.
//
//	GET  /v1/analytics/funnel/contract  the one documented event contract
//	POST /v1/analytics/funnel           the page reports a browser step
//
// The contract is public documentation. Ingest is public too — a logged-out
// visitor's steps must be counted — but runs behind optional auth so a signed-in
// person's or an agent's steps are attributed to a pseudonymous reference rather
// than to nobody. Server-side funnel steps (room_created, participant_joined,
// first_two_way_exchange) are recorded from the room handlers, not here, so the
// funnel stays measurable even when this endpoint is never called.
//
// Kept out of router.go, which is already over the file-size limit.
func mountFunnelRoutes(r chi.Router, pool *db.Pool) {
	if pool == nil {
		return
	}

	h := handlers.NewFunnelHandler(db.NewFunnelEventRepository(pool))
	// A step may be attributed to the public room or post it came from (idx 88).
	h.SetSourceResolvers(db.NewRoomRepository(pool), db.NewPostRepository(pool))
	r.Get("/v1/analytics/funnel/contract", h.GetContract)

	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-jwt-secret-32-chars-long!!"
	}
	apiKeyValidator := auth.NewAPIKeyValidator(db.NewAgentRepository(pool)).WithOwnerCheck(db.NewUserRepository(pool))
	userAPIKeyValidator := auth.NewUserAPIKeyValidator(db.NewUserAPIKeyRepository(pool))
	optionalAuth := auth.OptionalAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, db.NewUserRepository(pool))

	r.With(optionalAuth).Post("/v1/analytics/funnel", h.IngestBrowserEvent)
}
