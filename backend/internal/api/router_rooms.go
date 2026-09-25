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
//	/r/{slug}/*  -- A2A protocol (per-agent room token auth, from the handshake)
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

	roomHandler := handlers.NewRoomHandler(roomRepo, msgRepo, presenceRepo, memberRepo, agentTokenRepo, eventRepo)
	msgHandler := handlers.NewRoomMessagesHandler(msgRepo, roomRepo, presenceRepo, eventRepo, hubMgr)
	presenceHandler := handlers.NewRoomPresenceHandler(presenceRepo, roomRepo, hubMgr, registry)

	// Wire the connection-funnel recorder so room_created, participant_joined and
	// first_two_way_exchange are recorded from confirmed server actions. Shared by
	// the three handlers so every server step lands in one funnel_events table.
	funnelRepo := db.NewFunnelEventRepository(pool)
	roomHandler.SetFunnelRecorder(funnelRepo)
	roomHandler.SetOverviewChangeNotifier(pool.OverviewChanged)
	presenceHandler.SetFunnelRecorder(funnelRepo)
	msgHandler.SetFunnelRecorder(funnelRepo)
	entryRepo := db.NewRoomEntryRepository(pool)
	if hubMgr != nil {
		// Live delivery reads committed entries back from the timeline (see StartRoomRelay).
		hubMgr.EnableRelay(handlers.NewTimelineFrameSource(entryRepo))
	}
	sseHandler := handlers.NewRoomSSEHandler(hubMgr, msgRepo, entryRepo, roomRepo)
	claimsHandler := handlers.NewRoomClaimsHandler(claimRepo)
	eventsHandler := handlers.NewRoomEventsHandler(entryRepo, hubMgr)
	roomConnectHandler := handlers.NewRoomConnectHandler(roomRepo, msgRepo)
	roomSavePostHandler := handlers.NewRoomSavePostHandler(db.NewPostRepository(pool), roomRepo, memberRepo)

	// Canonical timeline routes accept human, agent-account and room-scoped credentials
	// through ONE authorization policy (RoomPolicyGuard). Writes share the adapters'
	// rate-limit buckets: an agent write (message or event) counts against the same
	// 60/min bucket as POST /r/{slug}/message and POST /r/{slug}/events, a human write against the same 10/min bucket as
	// POST /v1/rooms/{slug}/messages. Each limiter is built once and mounted on both.
	entriesHandler := handlers.NewRoomEntriesHandler(entryRepo, msgHandler, eventsHandler)
	agentWriteLimit := httprate.LimitByIP(60, time.Minute)
	humanWriteLimit := httprate.LimitByIP(10, time.Minute)
	entryWriteLimit := func(next http.Handler) http.Handler {
		agentNext, humanNext := agentWriteLimit(next), humanWriteLimit(next)
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if actor := apimiddleware.RoomActorFromContext(r.Context()); actor != nil && actor.Type == apimiddleware.RoomActorHuman {
				humanNext.ServeHTTP(w, r)
				return
			}
			agentNext.ServeHTTP(w, r)
		})
	}
	entriesPolicy := func(access apimiddleware.RoomAccess) func(http.Handler) http.Handler {
		guard := apimiddleware.RoomPolicyGuard(roomRepo, memberRepo, agentTokenRepo, access)
		return func(next http.Handler) http.Handler { return optionalAuthMiddleware(guard(next)) }
	}

	// -- REST routes: /v1/rooms/* (D-18, D-19: public list/detail, auth for write) --
	r.Route("/v1/rooms", func(r chi.Router) {
		// List is unconditionally public; it already excludes closed rooms.
		r.Get("/", roomHandler.ListRooms)

		// Per-room reads are decided by the same policy as GET /{slug}/entries: open for
		// public rooms, members-only (403 otherwise) for closed rooms, and a presented room
		// token must be valid (401) and for THIS room (403).
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}", roomHandler.GetRoom)
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/agents", presenceHandler.ListPresence)
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/connect", roomConnectHandler.GetRoomConnect)

		// Message reads are adapters over the canonical timeline: the message entries of
		// GET /{slug}/entries, decided by the same policy (a presented room token must be
		// valid and for THIS room).
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/messages", msgHandler.ListMessages)
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/messages/{id}", msgHandler.GetMessage)

		// Canonical timeline: messages and events in one ordered, cursor-paged list.
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/entries", entriesHandler.ListEntries)
		r.With(entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/entries/{entry_id}", entriesHandler.GetEntry)
		r.With(entriesPolicy(apimiddleware.RoomWrite), entryWriteLimit).Post("/{slug}/entries", entriesHandler.PostEntry)

		// Published outcome posts saved from this room (room links to the published outcome).
		// OptionalAuth lets the handler gate a private room's outcomes to its participants.
		r.With(optionalAuthMiddleware).Get("/{slug}/posts", roomSavePostHandler.ListOutcomePosts)

		// Canonical SSE stream (D-33 / T-16-04): anonymous for public rooms, and human,
		// agent-account or room-scoped credentials through the same policy as the entries
		// routes (a presented room token must be valid and for THIS room). SSEAccessTokenToHeader
		// promotes ?access_token= (the only way a browser EventSource can send a credential)
		// into the Authorization header (BART-156). GET /r/{slug}/stream is its adapter.
		r.With(apimiddleware.SSENoBuffering, apimiddleware.SSEAccessTokenToHeader, entriesPolicy(apimiddleware.RoomRead)).Get("/{slug}/stream", sseHandler.PublicStream)

		// Authenticated endpoints (Solvr JWT or agent API key per D-16)
		r.Group(func(r chi.Router) {
			r.Use(authMiddleware)
			r.With(apimiddleware.Idempotency(db.NewIdempotencyRepository(pool), "room.create")).Post("/", roomHandler.CreateRoom)
			r.Patch("/{slug}", roomHandler.UpdateRoom)
			r.Delete("/{slug}", roomHandler.DeleteRoom)
			// Finish / reopen a collaboration (owner-only). Archiving keeps the
			// transcript readable but refuses new messages and joins until reopened.
			r.Post("/{slug}/archive", roomHandler.ArchiveRoom)
			r.Post("/{slug}/reopen", roomHandler.ReopenRoom)
			// Human comment endpoint (JWT-authenticated, rate limited per T-16-02): adapter
			// into the canonical message submission, decided by the same write policy as
			// POST /{slug}/entries — a PRIVATE room only for its owner/family/members, a
			// public room open to any authenticated human (BART-156 family scope).
			r.With(apimiddleware.RoomPolicyGuard(roomRepo, memberRepo, agentTokenRepo, apimiddleware.RoomWrite), humanWriteLimit).Post("/{slug}/messages", msgHandler.PostHumanMessage)

			// Per-agent handshake + member allowlist management (mission #3).
			r.Post("/{slug}/handshake", roomHandler.Handshake)
			r.Get("/{slug}/members", roomHandler.ListMembers)
			r.Post("/{slug}/members", roomHandler.AddMember)
			r.Delete("/{slug}/members/{agent_id}", roomHandler.RemoveMember)

			// Turn a room outcome into a reusable canonical draft Post (author reviews and
			// publishes it via the normal Post flow; a private-room outcome is published only
			// through the owner-approval endpoint below).
			r.Post("/{slug}/save-as-post", roomSavePostHandler.SaveAsPost)
			r.Post("/{slug}/posts/{postID}/publish", roomSavePostHandler.ApprovePublication)
		})
	})

	// -- A2A protocol routes: /r/{slug}/* (per-agent solvr_rt_ bearer token auth) --
	r.Route("/r/{slug}", func(r chi.Router) {
		r.Use(apimiddleware.SSENoBuffering) // Must be before BearerGuard so header is set even on 401
		r.Use(apimiddleware.BearerGuard(roomRepo, agentTokenRepo))

		// D-32: Per-room rate limiting on message posting (60 req/min per IP).
		// Applied only to POST /message, not to read endpoints or SSE stream.
		// Transport adapter into the same submission path as POST /v1/rooms/{slug}/entries.
		r.With(agentWriteLimit).Post("/message", msgHandler.PostMessage)

		r.Get("/messages", msgHandler.ListMessages)
		r.Get("/messages/{id}", msgHandler.GetMessage)

		// Review loop: pin a directive/result and list a room's pinned entries so
		// participants surface them without scanning the whole transcript.
		r.Get("/pins", msgHandler.ListPinnedMessages)
		r.Post("/messages/{id}/pin", msgHandler.PinMessage)
		r.Delete("/messages/{id}/pin", msgHandler.UnpinMessage)

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
		// Transport adapter into the same event submission path and agent write bucket as
		// POST /v1/rooms/{slug}/entries with kind=event.
		r.With(agentWriteLimit).Post("/events", eventsHandler.PostEvent)
		r.Get("/events", eventsHandler.ListEvents)

		// Transport adapter over the canonical GET /v1/rooms/{slug}/stream (same hub, frames and
		// reconnect replay of messages and events).
		r.Get("/stream", sseHandler.Stream)
	})
}
