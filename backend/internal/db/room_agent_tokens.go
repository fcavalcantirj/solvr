package db

import (
	"context"
	"errors"
	"time"

	"github.com/fcavalcantirj/solvr/internal/token"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// ErrAgentRoomTokenNotFound is returned when a per-agent room token lookup misses.
var ErrAgentRoomTokenNotFound = errors.New("agent room token not found")

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

// Issue creates (or replaces) the per-agent token for (room, agent) and returns the
// plaintext token, shown once. ttl <= 0 issues a non-expiring token.
func (r *RoomAgentTokenRepository) Issue(ctx context.Context, roomID uuid.UUID, agentID string, ttlSeconds int) (string, error) {
	plaintext, hashHex, err := token.GenerateAgentRoomToken()
	if err != nil {
		return "", err
	}
	var expiresAt *time.Time
	if ttlSeconds > 0 {
		// Compute expiry in SQL to avoid clock skew and the disallowed time.Now in some contexts.
		var exp time.Time
		if err := r.pool.QueryRow(ctx, `SELECT NOW() + make_interval(secs => $1)`, ttlSeconds).Scan(&exp); err != nil {
			return "", err
		}
		expiresAt = &exp
	}
	_, err = r.pool.Exec(ctx, `
		INSERT INTO room_agent_tokens (room_id, agent_id, token_hash, expires_at)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (room_id, agent_id)
		DO UPDATE SET token_hash = EXCLUDED.token_hash, expires_at = EXCLUDED.expires_at,
		              created_at = NOW(), last_used_at = NULL
	`, roomID, agentID, hashHex, expiresAt)
	if err != nil {
		LogQueryError(ctx, "Issue", "room_agent_tokens", err)
		return "", err
	}
	return plaintext, nil
}

// ResolveByHash returns the (room, agent) a live per-agent token identifies. Expired
// tokens are treated as not found. Updates last_used_at on success (best-effort).
func (r *RoomAgentTokenRepository) ResolveByHash(ctx context.Context, hash string) (*AgentRoomTokenIdentity, error) {
	var id AgentRoomTokenIdentity
	err := r.pool.QueryRow(ctx, `
		SELECT room_id, agent_id
		FROM room_agent_tokens
		WHERE token_hash = $1 AND (expires_at IS NULL OR expires_at > NOW())
	`, hash).Scan(&id.RoomID, &id.AgentID)
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

// Revoke deletes an agent's per-agent token for a room. It is not an error if none exists.
func (r *RoomAgentTokenRepository) Revoke(ctx context.Context, roomID uuid.UUID, agentID string) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM room_agent_tokens WHERE room_id = $1 AND agent_id = $2`, roomID, agentID)
	if err != nil {
		LogQueryError(ctx, "Revoke", "room_agent_tokens", err)
	}
	return err
}
