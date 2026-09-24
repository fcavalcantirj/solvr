package middleware

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// RoomAccessGuard enforces the closed-room read/write ACL (mission #1) on the public
// /v1/rooms/{slug}/* routes, and injects the resolved room into the request context so
// downstream handlers skip a second lookup.
//
// Access rules:
//   - Public room (is_private = false): always allowed, even anonymously.
//   - Closed room (is_private = true): allowed only for a member —
//     1. a request carrying a live per-agent room token for THIS room (Authorization:
//     Bearer solvr_rt_... or ?token=...), OR
//     2. an authenticated agent on the room's member allowlist (or family scope), OR
//     3. a human with an active membership, or an admin (JWT / user API key).
//     Everyone else gets 403.
//
// Apply OptionalAuthMiddleware BEFORE this guard so the caller's agent/human identity
// is available in context for the membership checks.
func RoomAccessGuard(roomRepo *db.RoomRepository, memberRepo *db.RoomMemberRepository, agentTokenRepo *db.RoomAgentTokenRepository) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			slug := chi.URLParam(r, "slug")
			if slug == "" {
				roomGuardError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
				return
			}

			room, err := roomRepo.GetBySlug(r.Context(), slug)
			if err != nil {
				if errors.Is(err, db.ErrRoomNotFound) {
					roomGuardError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
					return
				}
				slog.Error("room access guard: failed to load room", "error", err, "slug", slug)
				roomGuardError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load room")
				return
			}

			if !room.IsPrivate {
				// Public room: anyone may read. Inject room and continue.
				next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), RoomContextKey, room)))
				return
			}

			allowed, err := roomMemberAccessAllowed(r, room, memberRepo, agentTokenRepo)
			if err != nil {
				slog.Error("room access guard: membership check failed", "error", err, "slug", slug)
				roomGuardError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to check room access")
				return
			}
			if !allowed {
				roomGuardError(w, http.StatusForbidden, "FORBIDDEN", "this room is closed to non-members")
				return
			}

			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), RoomContextKey, room)))
		})
	}
}

// roomMemberAccessAllowed reports whether the caller may access a closed room. It uses
// the same actor resolution and decision as RoomPolicyGuard (roomActorAllowed), except
// that a room token which cannot act on this room is ignored rather than refused, so the
// caller's account identity (if any) still decides.
func roomMemberAccessAllowed(r *http.Request, room *models.Room, memberRepo *db.RoomMemberRepository, agentTokenRepo *db.RoomAgentTokenRepository) (bool, error) {
	actor, err := resolveRoomActor(r, room, agentTokenRepo)
	if err != nil {
		actor = accountRoomActor(r)
	}
	return roomActorAllowed(r.Context(), room, actor, RoomRead, memberRepo)
}

// roomBearerToken extracts a candidate room token from the Authorization header or the
// ?token= query parameter (SSE clients cannot set headers).
func roomBearerToken(r *http.Request) string {
	if authHeader := r.Header.Get("Authorization"); strings.HasPrefix(authHeader, "Bearer ") {
		return strings.TrimPrefix(authHeader, "Bearer ")
	}
	return r.URL.Query().Get("token")
}

// roomGuardError writes a JSON error response mirroring the room handlers' shape.
func roomGuardError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"error": map[string]string{"code": code, "message": message},
	})
}
