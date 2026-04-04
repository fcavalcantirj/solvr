package api

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/httprate"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
)

// mountRoomRoutes registers all room-related endpoints.
// Two route namespaces per D-15:
//
//	/v1/rooms/*  -- REST CRUD (Solvr JWT/agent key auth)
//	/r/{slug}/*  -- A2A protocol (room bearer token auth)
//
// The authMiddleware parameter is the unified auth middleware used for
// write operations on /v1/rooms/* (same as other protected endpoints).
func mountRoomRoutes(
	r chi.Router,
	pool *db.Pool,
	hubMgr *hub.HubManager,
	registry *hub.PresenceRegistry,
	authMiddleware func(http.Handler) http.Handler,
) {
	roomRepo := db.NewRoomRepository(pool)
	msgRepo := db.NewMessageRepository(pool)
	presenceRepo := db.NewAgentPresenceRepository(pool)

	roomHandler := handlers.NewRoomHandler(roomRepo, msgRepo, presenceRepo)
	msgHandler := handlers.NewRoomMessagesHandler(msgRepo, roomRepo, presenceRepo, hubMgr)
	presenceHandler := handlers.NewRoomPresenceHandler(presenceRepo, roomRepo, hubMgr, registry)
	sseHandler := handlers.NewRoomSSEHandler(hubMgr, msgRepo)

	// -- REST routes: /v1/rooms/* (D-18, D-19: public list/detail, auth for write) --
	r.Route("/v1/rooms", func(r chi.Router) {
		// Public endpoints (no auth required per D-18, D-19)
		r.Get("/", roomHandler.ListRooms)
		r.Get("/{slug}", roomHandler.GetRoom)
		r.Get("/{slug}/messages", msgHandler.ListMessages)
		r.Get("/{slug}/agents", presenceHandler.ListPresence)

		// Authenticated endpoints (Solvr JWT or agent API key per D-16)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Post("/", roomHandler.CreateRoom)
			r.Patch("/{slug}", roomHandler.UpdateRoom)
			r.Delete("/{slug}", roomHandler.DeleteRoom)
			r.Post("/{slug}/rotate-token", roomHandler.RotateToken)
		})
	})

	// -- A2A protocol routes: /r/{slug}/* (D-17: bearer token auth) --
	r.Route("/r/{slug}", func(r chi.Router) {
		r.Use(apimiddleware.SSENoBuffering) // Must be before BearerGuard so header is set even on 401
		r.Use(apimiddleware.BearerGuard(roomRepo))

		// D-32: Per-room rate limiting on message posting (60 req/min per IP).
		// Applied only to POST /message, not to read endpoints or SSE stream.
		r.With(httprate.LimitByIP(60, time.Minute)).Post("/message", msgHandler.PostMessage)

		r.Get("/messages", msgHandler.ListMessages)

		r.Post("/join", presenceHandler.JoinRoom)
		r.Post("/heartbeat", presenceHandler.Heartbeat)
		r.Post("/leave", presenceHandler.LeaveRoom)
		r.Get("/agents", presenceHandler.ListPresence)
		r.Get("/agents/{agent_name}", presenceHandler.GetAgentCard)

		r.Get("/stream", sseHandler.Stream)
	})
}
