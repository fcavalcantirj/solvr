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

// RoomFromContext retrieves the resolved room from the request context.
// Returns nil if no room is present (i.e., BearerGuard middleware was not applied).
func RoomFromContext(ctx context.Context) *models.Room {
	room, _ := ctx.Value(RoomContextKey).(*models.Room)
	return room
}

// BearerGuard creates middleware that authenticates requests using a room bearer token.
// It extracts the token from the Authorization header (Bearer <token>) or from a
// ?token= query parameter (for SSE connections where browsers cannot set headers).
// The plaintext token is SHA-256 hashed and looked up via RoomRepository.GetByTokenHash.
// On success, the resolved *models.Room is injected into the request context.
func BearerGuard(roomRepo *db.RoomRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			var plaintext string

			// Try Authorization header first
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				plaintext = strings.TrimPrefix(authHeader, "Bearer ")
			} else {
				// Fall back to ?token= query param for SSE connections
				plaintext = r.URL.Query().Get("token")
			}

			if plaintext == "" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"code":    "UNAUTHORIZED",
						"message": "missing bearer token",
					},
				})
				return
			}

			hash := token.HashToken(plaintext)
			room, err := roomRepo.GetByTokenHash(r.Context(), hash)
			if err != nil {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"code":    "UNAUTHORIZED",
						"message": "invalid room token",
					},
				})
				return
			}

			ctx := context.WithValue(r.Context(), RoomContextKey, room)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
