package db

import (
	"context"
	"errors"

	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAgentRoomTokenNotFound is returned when a per-agent room token lookup misses.
var ErrAgentRoomTokenNotFound = errors.New("agent room token not found")

// ErrAgentRoomTokenLimit is returned by Issue when the agent already holds
// MaxLiveRoomAgentTokens live tokens for the room. Rotate is the explicit way out.
var ErrAgentRoomTokenLimit = errors.New("agent already holds the maximum number of live room tokens")

const (
	// MaxLiveRoomAgentTokens is how many sessions (live tokens) one agent may hold in one room.
	MaxLiveRoomAgentTokens = 10
	// MaxRotatedRoomAgentTokens is how many replaced tokens are remembered per (room, agent)
	// so their holders can be told to handshake again.
	MaxRotatedRoomAgentTokens = 20
)

// AgentRoomTokenIdentity is the resolved (room, agent) pair for a per-agent token.
type AgentRoomTokenIdentity struct {
	RoomID  uuid.UUID
	AgentID string
}

// RoomAgentTokenRepository manages per-agent room credentials (mission #3).
type RoomAgentTokenRepository struct {
	pool *Pool
}

// NewRoomAgentTokenRepository creates a new RoomAgentTokenRepository.
func NewRoomAgentTokenRepository(pool *Pool) *RoomAgentTokenRepository {
	return &RoomAgentTokenRepository{pool: pool}
}

// liveToken is the SQL predicate for a token a caller may still use: neither replaced by a
// rotation nor past its expiry.
const liveToken = `rotated_at IS NULL AND (expires_at IS NULL OR expires_at > NOW())`

// Issue adds a session: a new per-agent token for (room, agent), returned in plaintext once.
// The agent's earlier tokens keep working, so a second session that handshakes does not
// invalidate the first. It fails with ErrAgentRoomTokenLimit when the agent already holds
// MaxLiveRoomAgentTokens live tokens; Rotate is the explicit way out. ttl <= 0 issues a
// non-expiring token.
func (r *RoomAgentTokenRepository) Issue(ctx context.Context, roomID uuid.UUID, agentID string, ttlSeconds int) (string, error) {
	plaintext, _, err := r.issue(ctx, roomID, agentID, ttlSeconds, false)
	return plaintext, err
}

// Rotate issues a new token and replaces every live token the agent holds for the room in
// one transaction: the replaced tokens stop resolving and are remembered (WasRotated) so
// their holders can be told to handshake again. It returns the new plaintext token and how
// many live tokens it replaced. Rotation is never refused for the live-token limit.
func (r *RoomAgentTokenRepository) Rotate(ctx context.Context, roomID uuid.UUID, agentID string, ttlSeconds int) (string, int, error) {
	return r.issue(ctx, roomID, agentID, ttlSeconds, true)
}

func (r *RoomAgentTokenRepository) issue(ctx context.Context, roomID uuid.UUID, agentID string, ttlSeconds int, rotate bool) (string, int, error) {
	plaintext, hashHex, err := token.GenerateAgentRoomToken()
	if err != nil {
		return "", 0, err
	}
	replaced := 0
	err = r.pool.WithTx(ctx, func(tx Tx) error {
		// One issuer at a time per (room, agent): the limit check and the rotation must not
		// interleave with another handshake of the same agent.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, roomID.String()+":"+agentID); err != nil {
			return err
		}
		if rotate {
			tag, err := tx.Exec(ctx, `UPDATE room_agent_tokens SET rotated_at = NOW()
				WHERE room_id = $1 AND agent_id = $2 AND `+liveToken, roomID, agentID)
			if err != nil {
				return err
			}
			replaced = int(tag.RowsAffected())
			// Remember replaced tokens only for a while and only up to a bound.
			if _, err := tx.Exec(ctx, `DELETE FROM room_agent_tokens
				WHERE room_id = $1 AND agent_id = $2 AND rotated_at IS NOT NULL
				  AND (rotated_at < NOW() - interval '30 days'
				       OR token_hash NOT IN (SELECT token_hash FROM room_agent_tokens
				                              WHERE room_id = $1 AND agent_id = $2 AND rotated_at IS NOT NULL
				                              ORDER BY rotated_at DESC, token_hash LIMIT $3))`,
				roomID, agentID, MaxRotatedRoomAgentTokens); err != nil {
				return err
			}
		} else {
			var live int
			if err := tx.QueryRow(ctx, `SELECT COUNT(*) FROM room_agent_tokens
				WHERE room_id = $1 AND agent_id = $2 AND `+liveToken, roomID, agentID).Scan(&live); err != nil {
				return err
			}
			if live >= MaxLiveRoomAgentTokens {
				return ErrAgentRoomTokenLimit
			}
		}
		// Expiry is computed in SQL so it does not depend on this host's clock.
		_, err := tx.Exec(ctx, `INSERT INTO room_agent_tokens (room_id, agent_id, token_hash, expires_at)
			VALUES ($1, $2, $3, CASE WHEN $4::int > 0 THEN NOW() + make_interval(secs => $4::int) ELSE NULL END)`,
			roomID, agentID, hashHex, ttlSeconds)
		return err
	})
	if err != nil {
		if !errors.Is(err, ErrAgentRoomTokenLimit) {
			LogQueryError(ctx, "Issue", "room_agent_tokens", err)
		}
		return "", 0, err
	}
	return plaintext, replaced, nil
}

// ResolveByHash returns the (room, agent) a live per-agent token identifies. Expired and
// rotated tokens are treated as not found. Updates last_used_at on success (best-effort).
func (r *RoomAgentTokenRepository) ResolveByHash(ctx context.Context, hash string) (*AgentRoomTokenIdentity, error) {
	var id AgentRoomTokenIdentity
	err := r.pool.QueryRow(ctx, `
		SELECT room_id, agent_id
		FROM room_agent_tokens
		WHERE token_hash = $1 AND `+liveToken, hash).Scan(&id.RoomID, &id.AgentID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrAgentRoomTokenNotFound
		}
		LogQueryError(ctx, "ResolveByHash", "room_agent_tokens", err)
		return nil, err
	}
	_, _ = r.pool.Exec(ctx, `UPDATE room_agent_tokens SET last_used_at = NOW() WHERE token_hash = $1`, hash)
	return &id, nil
}

// IsLive reports whether hash is a live (unexpired, not rotated) per-agent token for
// roomID. Unlike ResolveByHash it only reads: open streams re-check their token with it.
func (r *RoomAgentTokenRepository) IsLive(ctx context.Context, hash string, roomID uuid.UUID) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM room_agent_tokens
		              WHERE token_hash = $1 AND room_id = $2 AND `+liveToken+`)
	`, hash, roomID).Scan(&ok)
	if err != nil {
		LogQueryError(ctx, "IsLive", "room_agent_tokens", err)
	}
	return ok, err
}

// WasRotated reports whether hash is a token an explicit rotation replaced (and that has
// not since been revoked away). It is how a guard tells a replaced token, whose holder can
// recover by handshaking again, from a token that never existed or was revoked.
func (r *RoomAgentTokenRepository) WasRotated(ctx context.Context, hash string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM room_agent_tokens
		WHERE token_hash = $1 AND rotated_at IS NOT NULL)`, hash).Scan(&ok)
	if err != nil {
		LogQueryError(ctx, "WasRotated", "room_agent_tokens", err)
	}
	return ok, err
}

// Revoke deletes every per-agent token an agent holds for a room, replaced ones included
// (a revoked agent is not "rotated"). It is not an error if none exists.
func (r *RoomAgentTokenRepository) Revoke(ctx context.Context, roomID uuid.UUID, agentID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM room_agent_tokens WHERE room_id = $1 AND agent_id = $2`, roomID, agentID)
	if err != nil {
		LogQueryError(ctx, "Revoke", "room_agent_tokens", err)
	}
	return err
}
