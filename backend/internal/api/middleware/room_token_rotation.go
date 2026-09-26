package middleware

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
)

// CodeCredentialRotated is the error code of a per-agent room token that an explicit
// rotation replaced (POST /v1/rooms/{slug}/handshake with {"rotate": true}). The holder is
// still a member: it recovers by handshaking again with its agent API key. A token that never
// existed, expired or was revoked stays UNAUTHORIZED.
const CodeCredentialRotated = "CREDENTIAL_ROTATED"

// ErrRoomCredentialRotated classifies a presented room token that a rotation replaced. A
// RoomAccessRecheck also returns it together with false, so an open stream can say why it
// ended.
var ErrRoomCredentialRotated = errors.New("room token was replaced by a rotation")

// rotatedTokenMessage tells the holder of a replaced token how to recover.
func rotatedTokenMessage(slug string) string {
	if slug == "" {
		slug = "{slug}"
	}
	return "this room token was replaced by a rotation (a session of this agent handshook with rotate true); " +
		"POST /v1/rooms/" + slug + "/handshake with your agent API key to get a new one"
}

// tokenMiss classifies why a per-agent token did not resolve: replaced by a rotation, or
// simply not valid. A failed lookup of the reason is reported as the generic miss.
func tokenMiss(ctx context.Context, agentTokenRepo *db.RoomAgentTokenRepository, hash string) error {
	if rotated, err := agentTokenRepo.WasRotated(ctx, hash); err == nil && rotated {
		return ErrRoomCredentialRotated
	}
	return errRoomTokenInvalid
}

// tokenLive is the token half of a stream recheck: (true, nil) while the token is live,
// (false, ErrRoomCredentialRotated) when a rotation replaced it, (false, nil) otherwise.
func tokenLive(ctx context.Context, agentTokenRepo *db.RoomAgentTokenRepository, hash string, roomID uuid.UUID) (bool, error) {
	live, err := agentTokenRepo.IsLive(ctx, hash, roomID)
	if err != nil || live {
		return live, err
	}
	if rotated, err := agentTokenRepo.WasRotated(ctx, hash); err == nil && rotated {
		return false, ErrRoomCredentialRotated
	}
	return false, nil
}
