package middleware

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
)

type roomContextKey string

const RoomContextKey roomContextKey = "room"

// roomAgentIDContextKey holds the authenticated agent id when a per-agent room token
// (solvr_rt_...) was used. Empty for the shared room token (solvr_rm_...).
type roomAgentIDContextKey struct{}

// RoomFromContext retrieves the resolved room from the request context.
// Returns nil if no room is present (i.e., BearerGuard middleware was not applied).
func RoomFromContext(ctx context.Context) *models.Room {
	room, _ := ctx.Value(RoomContextKey).(*models.Room)
	return room
}

// RoomAgentIDFromContext returns the authoritatively-authenticated agent id for the
// request, set only when a per-agent room token was used. Empty string otherwise.
func RoomAgentIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(roomAgentIDContextKey{}).(string)
	return id
}

// BearerGuard creates middleware that authenticates requests using a room bearer token.
// It extracts the token from the Authorization header (Bearer <token>) or from a
// ?token= query parameter (for SSE connections where browsers cannot set headers).
//
// It accepts two kinds of token, both resolved by SHA-256 hash:
//   - the shared room token (solvr_rm_...): resolves the room only (backward compat);
//   - a per-agent room token (solvr_rt_...): resolves the room AND the authenticated
//     agent id, which is injected so message authorship is authoritative (mission #3).
//
// agentTokenRepo may be nil, in which case only shared tokens are accepted.
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

			hash := token.HashToken(plaintext)

			// Per-agent room token (solvr_rt_...): resolves room + authoritative agent id.
			if agentTokenRepo != nil && token.IsAgentRoomToken(plaintext) {
				identity, err := agentTokenRepo.ResolveByHash(r.Context(), hash)
				if err != nil {
					bearerGuardUnauthorized(w, "invalid or expired room token")
					return
				}
				room, err := roomRepo.GetByID(r.Context(), identity.RoomID)
				if err != nil {
					bearerGuardUnauthorized(w, "invalid room token")
					return
				}
				ctx := context.WithValue(r.Context(), RoomContextKey, room)
				ctx = context.WithValue(ctx, roomAgentIDContextKey{}, identity.AgentID)
				next.ServeHTTP(w, r.WithContext(ctx))
				return
			}

			// Shared room token (solvr_rm_...).
			room, err := roomRepo.GetByTokenHash(r.Context(), hash)
			if err != nil {
				bearerGuardUnauthorized(w, "invalid room token")
				return
			}
			ctx := context.WithValue(r.Context(), RoomContextKey, room)
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
