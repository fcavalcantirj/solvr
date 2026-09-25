package middleware

import (
	"context"
	"errors"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/token"
)

// RoomAccessRecheck re-runs a room guard's authorization decision for a request that is
// still open (a stream). It returns false when the caller no longer has the access the
// guard granted: the room is gone, the room token was revoked or replaced, or the room
// policy now refuses the actor. A non-nil error means the decision could not be read.
type RoomAccessRecheck func(ctx context.Context) (bool, error)

type roomAccessRecheckKey struct{}

// RoomAccessRecheckFromContext returns the recheck the room guard left for this request,
// or nil when no guard ran.
func RoomAccessRecheckFromContext(ctx context.Context) RoomAccessRecheck {
	fn, _ := ctx.Value(roomAccessRecheckKey{}).(RoomAccessRecheck)
	return fn
}

// WithRoomAccessRecheck stores fn as the request's recheck (the room guards set it).
func WithRoomAccessRecheck(ctx context.Context, fn RoomAccessRecheck) context.Context {
	return context.WithValue(ctx, roomAccessRecheckKey{}, fn)
}

// currentRoom re-reads the room; (nil, nil) when it was deleted.
func currentRoom(ctx context.Context, roomRepo *db.RoomRepository, room *models.Room) (*models.Room, error) {
	fresh, err := roomRepo.GetByID(ctx, room.ID)
	if errors.Is(err, db.ErrRoomNotFound) {
		return nil, nil
	}
	return fresh, err
}

// policyRecheck re-applies roomActorAllowed with the current room and, for a room-token
// actor, the token's current liveness.
func policyRecheck(r *http.Request, room *models.Room, actor *RoomActor, access RoomAccess,
	roomRepo *db.RoomRepository, memberRepo *db.RoomMemberRepository, agentTokenRepo *db.RoomAgentTokenRepository) RoomAccessRecheck {
	var tokenHash string
	if actor != nil && actor.Credential == RoomCredentialRoomToken {
		tokenHash = token.HashToken(roomBearerToken(r))
	}
	return func(ctx context.Context) (bool, error) {
		fresh, err := currentRoom(ctx, roomRepo, room)
		if err != nil || fresh == nil {
			return false, err
		}
		if tokenHash != "" {
			if live, err := agentTokenRepo.IsLive(ctx, tokenHash, fresh.ID); err != nil || !live {
				return false, err
			}
		}
		return roomActorAllowed(ctx, fresh, actor, access, memberRepo)
	}
}

// tokenRecheck is the recheck of a room-token-only route (BearerGuard): the room still
// exists and the token is still live for it.
func tokenRecheck(room *models.Room, tokenHash string, roomRepo *db.RoomRepository, agentTokenRepo *db.RoomAgentTokenRepository) RoomAccessRecheck {
	return func(ctx context.Context) (bool, error) {
		fresh, err := currentRoom(ctx, roomRepo, room)
		if err != nil || fresh == nil {
			return false, err
		}
		return agentTokenRepo.IsLive(ctx, tokenHash, fresh.ID)
	}
}
