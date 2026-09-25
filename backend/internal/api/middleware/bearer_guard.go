package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/go-chi/chi/v5"
)

type roomContextKey string

const RoomContextKey roomContextKey = "room"

// roomAgentIDContextKey holds the agent id authenticated by a per-agent room token
// (solvr_rt_...).
type roomAgentIDContextKey struct{}

// RoomFromContext retrieves the resolved room from the request context.
// Returns nil if no room is present (i.e., BearerGuard middleware was not applied).
func RoomFromContext(ctx context.Context) *models.Room {
	room, _ := ctx.Value(RoomContextKey).(*models.Room)
	return room
}

// RoomAgentIDFromContext returns the authoritatively-authenticated agent id for the
// request, set by BearerGuard. Empty string when BearerGuard did not run.
func RoomAgentIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(roomAgentIDContextKey{}).(string)
	return id
}

// BearerGuard creates middleware that authenticates /r/{slug}/* requests with a
// per-agent room token (solvr_rt_...), issued by POST /v1/rooms/{slug}/handshake to an
// admitted member. It extracts the token from the Authorization header (Bearer <token>)
// or from a ?token= query parameter (for SSE connections where browsers cannot set
// headers), resolves it by SHA-256 hash to the room AND the authenticated agent id, and
// injects both so message authorship is authoritative (mission #3). The shared room
// token (solvr_rm_...) is retired (000098): anything else is 401.
func BearerGuard(roomRepo *db.RoomRepository, agentTokenRepo *db.RoomAgentTokenRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var plaintext string
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				plaintext = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				plaintext = r.URL.Query().Get("token")
			}

			if plaintext == "" {
				bearerGuardUnauthorized(w, "missing bearer token")
				return
			}
			if agentTokenRepo == nil || !token.IsAgentRoomToken(plaintext) {
				bearerGuardUnauthorized(w, "invalid room token; handshake for a per-agent room token")
				return
			}

			tokenHash := token.HashToken(plaintext)
			identity, err := agentTokenRepo.ResolveByHash(r.Context(), tokenHash)
			if err != nil {
				bearerGuardUnauthorized(w, "invalid or expired room token")
				return
			}
			room, err := roomRepo.GetByID(r.Context(), identity.RoomID)
			if err != nil {
				bearerGuardUnauthorized(w, "invalid room token")
				return
			}
			// A room token authorizes only its own room: /r/{other-slug} is refused
			// instead of silently acting on the token's room (slugs are immutable).
			if slug := chi.URLParam(r, "slug"); slug != "" && slug != room.Slug {
				roomGuardError(w, http.StatusForbidden, "FORBIDDEN", errRoomTokenScope.Error())
				return
			}
			ctx := context.WithValue(r.Context(), RoomContextKey, room)
			ctx = context.WithValue(ctx, roomAgentIDContextKey{}, identity.AgentID)
			ctx = WithRoomAccessRecheck(ctx, tokenRecheck(room, tokenHash, roomRepo, agentTokenRepo))
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// bearerGuardUnauthorized writes a 401 JSON error.
func bearerGuardUnauthorized(w http.ResponseWriter, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{"code": "UNAUTHORIZED", "message": message},
	})
}
