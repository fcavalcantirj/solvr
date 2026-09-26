package handlers

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/go-chi/chi/v5"
)

// RoomStreamTicketHandler mints the short-lived tickets a browser EventSource opens the
// room stream with (idx 75 step 2).
type RoomStreamTicketHandler struct {
	secret string
	now    func() time.Time
}

// NewRoomStreamTicketHandler signs tickets with a key derived from secret (the server's
// JWT secret), so every API instance accepts a ticket any instance minted.
func NewRoomStreamTicketHandler(secret string) *RoomStreamTicketHandler {
	return &RoomStreamTicketHandler{secret: secret, now: time.Now}
}

// Issue handles POST /v1/rooms/{slug}/stream-ticket.
//
// It sits behind the read policy of the room routes, so the caller presents the credential
// in the Authorization header (human JWT, user or agent API key, or per-agent room token)
// and is only given a ticket for a room it may read. The ticket is bound to that room and
// that actor, lives auth.StreamTicketTTL, opens only GET /v1/rooms/{slug}/stream, and
// authorizes no write and no further ticket. An anonymous viewer needs none: a public
// room's stream is open, a private room's is not.
func (h *RoomStreamTicketHandler) Issue(w http.ResponseWriter, r *http.Request) {
	room := middleware.RoomFromContext(r.Context())
	actor := middleware.RoomActorFromContext(r.Context())
	if room == nil || actor == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED",
			"authentication required: an anonymous viewer opens a public room's stream without a ticket")
		return
	}

	claims := auth.StreamTicketClaims{RoomID: room.ID.String(), Kind: actor.Credential, Subject: actor.ID, Admin: actor.Admin}
	if actor.Credential == middleware.RoomCredentialRoomToken {
		claims.TokenHash = middleware.RoomTokenHash(r)
	}
	ticket, expires := auth.IssueStreamTicket(h.secret, claims, h.now())

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{
		"ticket":      ticket,
		"expires_at":  expires.UTC().Format(time.RFC3339),
		"ttl_seconds": int(auth.StreamTicketTTL / time.Second),
		"stream":      "/v1/rooms/" + chi.URLParam(r, "slug") + "/stream",
	}})
}
