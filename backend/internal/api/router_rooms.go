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
	optionalAuthMiddleware func(http.Handler) http.Handler,
) {
	roomRepo := db.NewRoomRepository(pool)
	msgRepo := db.NewMessageRepository(pool)
	presenceRepo := db.NewAgentPresenceRepository(pool)
	memberRepo := db.NewRoomMemberRepository(pool)
	claimRepo := db.NewRoomClaimRepository(pool)
	eventRepo := db.NewRoomEventRepository(pool)
	agentTokenRepo := db.NewRoomAgentTokenRepository(pool)

	roomHandler := handlers.NewRoomHandler(roomRepo, msgRepo, presenceRepo, memberRepo, agentTokenRepo)
	msgHandler := handlers.NewRoomMessagesHandler(msgRepo, roomRepo, presenceRepo, hubMgr)
	presenceHandler := handlers.NewRoomPresenceHandler(presenceRepo, roomRepo, hubMgr, registry)
	sseHandler := handlers.NewRoomSSEHandler(hubMgr, msgRepo, roomRepo)
	claimsHandler := handlers.NewRoomClaimsHandler(claimRepo)
	eventsHandler := handlers.NewRoomEventsHandler(eventRepo, hubMgr)

	// readGuard resolves the room, enforces the closed-room ACL (mission #1), and
	// injects the room into context so the handlers below skip a second lookup.
	// optionalAuth runs first so the guard sees the caller's agent/human identity.
	readGuard := func(next http.Handler) http.Handler {
		return optionalAuthMiddleware(apimiddleware.RoomAccessGuard(roomRepo, memberRepo, agentTokenRepo)(next))
	}

	// -- REST routes: /v1/rooms/* (D-18, D-19: public list/detail, auth for write) --
	r.Route("/v1/rooms", func(r chi.Router) {
		// List is unconditionally public; it already excludes closed rooms.
		r.Get("/", roomHandler.ListRooms)

		// Per-room reads pass through the access guard: open for public rooms,
		// members-only (403 otherwise) for closed rooms.
		r.With(readGuard).Get("/{slug}", roomHandler.GetRoom)
		r.With(readGuard).Get("/{slug}/messages", msgHandler.ListMessages)
		r.With(readGuard).Get("/{slug}/agents", presenceHandler.ListPresence)

		// Public SSE stream for browser clients (no bearer token required, D-33 / T-16-04).
		// The access guard still gates closed rooms.
		r.With(apimiddleware.SSENoBuffering, readGuard).Get("/{slug}/stream", sseHandler.PublicStream)

		// Authenticated endpoints (Solvr JWT or agent API key per D-16)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.Post("/", roomHandler.CreateRoom)
			r.Patch("/{slug}", roomHandler.UpdateRoom)
			r.Delete("/{slug}", roomHandler.DeleteRoom)
			r.Post("/{slug}/rotate-token", roomHandler.RotateToken)
			// Human comment endpoint (JWT-authenticated, rate limited per T-16-02).
			r.With(httprate.LimitByIP(10, time.Minute)).Post("/{slug}/messages", msgHandler.PostHumanMessage)

			// Per-agent handshake + member allowlist management (mission #3).
			r.Post("/{slug}/handshake", roomHandler.Handshake)
			r.Get("/{slug}/members", roomHandler.ListMembers)
			r.Post("/{slug}/members", roomHandler.AddMember)
			r.Delete("/{slug}/members/{agent_id}", roomHandler.RemoveMember)
		})
	})

	// -- A2A protocol routes: /r/{slug}/* (D-17: bearer token auth) --
	r.Route("/r/{slug}", func(r chi.Router) {
		r.Use(apimiddleware.SSENoBuffering) // Must be before BearerGuard so header is set even on 401
		r.Use(apimiddleware.BearerGuard(roomRepo, agentTokenRepo))

		// D-32: Per-room rate limiting on message posting (60 req/min per IP).
		// Applied only to POST /message, not to read endpoints or SSE stream.
		r.With(httprate.LimitByIP(60, time.Minute)).Post("/message", msgHandler.PostMessage)

		r.Get("/messages", msgHandler.ListMessages)

		r.Post("/join", presenceHandler.JoinRoom)
		r.Post("/heartbeat", presenceHandler.Heartbeat)
		r.Post("/leave", presenceHandler.LeaveRoom)
		r.Get("/agents", presenceHandler.ListPresence)
		r.Get("/agents/{agent_name}", presenceHandler.GetAgentCard)

		// Atomic claim/lease primitive (mission #2). Distributed lock per (room, key).
		r.Post("/claim", claimsHandler.Claim)
		r.Post("/claim/renew", claimsHandler.RenewClaim)
		r.Post("/claim/release", claimsHandler.ReleaseClaim)
		r.Get("/claims", claimsHandler.ListClaims)

		// Typed coordination events (mission #4). Structured, queryable, streamed.
		r.With(httprate.LimitByIP(60, time.Minute)).Post("/events", eventsHandler.PostEvent)
		r.Get("/events", eventsHandler.ListEvents)

		r.Get("/stream", sseHandler.Stream)
	})
}
