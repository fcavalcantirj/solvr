package middleware

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/go-chi/chi/v5"
)

// Stream ticket error codes (idx 75 step 2). Both are recoverable: the client asks
// POST /v1/rooms/{slug}/stream-ticket for a fresh ticket and reopens the stream.
const (
	CodeStreamTicketInvalid = "STREAM_TICKET_INVALID"
	CodeStreamTicketExpired = "STREAM_TICKET_EXPIRED"
)

type streamTicketContextKey struct{}

// StreamTicketFromContext returns the verified ticket the stream route carried, or nil.
// Only SSEStreamTicket sets it, and only on the read stream: no write route ever has one.
func StreamTicketFromContext(ctx context.Context) *auth.StreamTicketClaims {
	claims, _ := ctx.Value(streamTicketContextKey{}).(*auth.StreamTicketClaims)
	return claims
}

// RoomTokenHash is the stored hash of the room token this request presents, or the hash
// of the empty string when it presents none. A stream opened with a ticket minted from a
// room token presents the hash carried in the ticket instead.
func RoomTokenHash(r *http.Request) string {
	if claims := StreamTicketFromContext(r.Context()); claims != nil && claims.TokenHash != "" {
		return claims.TokenHash
	}
	return token.HashToken(roomBearerToken(r))
}

// SSEStreamTicket authenticates the room stream for a browser EventSource, which cannot
// send an Authorization header. It reads `?ticket=` (a short-lived, room-bound, read-only
// ticket from POST /v1/rooms/{slug}/stream-ticket) when the request carries no
// Authorization header, verifies it, and leaves the claims in the context for the room
// policy guard, which turns them into the actor and re-checks that actor while the stream is
// open. A ticket is honoured only here: every other route is header-only.
//
// A long-lived bearer in the stream URL (`?access_token=`, `?token=`) is refused with a
// 400 that names the replacement and never echoes the value: a URL is copied, logged and
// screenshotted, so it must not carry a credential that outlives the page.
func SSEStreamTicket(secret string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			query := r.URL.Query()
			slug := chi.URLParam(r, "slug")
			if query.Has("access_token") || query.Has("token") {
				roomGuardError(w, http.StatusBadRequest, "VALIDATION_ERROR",
					"credentials in the stream URL are retired: POST /v1/rooms/"+slug+"/stream-ticket with your credential in the Authorization header, then open the stream with ?ticket=; a client that can set headers sends Authorization: Bearer on the stream itself")
				return
			}
			if ticket := query.Get("ticket"); ticket != "" && r.Header.Get("Authorization") == "" {
				claims, err := auth.ParseStreamTicket(secret, ticket, time.Now())
				switch {
				case errors.Is(err, auth.ErrStreamTicketExpired):
					roomGuardError(w, http.StatusUnauthorized, CodeStreamTicketExpired, freshTicketMessage("the stream ticket expired", slug))
					return
				case err != nil:
					roomGuardError(w, http.StatusUnauthorized, CodeStreamTicketInvalid, freshTicketMessage("the stream ticket is not valid", slug))
					return
				}
				r = r.WithContext(context.WithValue(r.Context(), streamTicketContextKey{}, claims))
			}
			next.ServeHTTP(w, r)
		})
	}
}

func freshTicketMessage(reason, slug string) string {
	return reason + ": POST /v1/rooms/" + slug + "/stream-ticket for a new one"
}
