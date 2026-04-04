package db

import (
	"context"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// AgentPresenceRepository handles database operations for agent presence records.
type AgentPresenceRepository struct {
	pool *Pool
}

// NewAgentPresenceRepository creates a new AgentPresenceRepository.
func NewAgentPresenceRepository(pool *Pool) *AgentPresenceRepository {
	return &AgentPresenceRepository{pool: pool}
}

// Upsert inserts or updates an agent presence record.
// On conflict (room_id, agent_name), updates card_json, last_seen, and ttl_seconds.
func (r *AgentPresenceRepository) Upsert(ctx context.Context, params models.UpsertAgentPresenceParams) (*models.AgentPresenceRecord, error) {
	query := `
		INSERT INTO agent_presence (room_id, agent_name, card_json, ttl_seconds)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (room_id, agent_name)
		DO UPDATE SET card_json = EXCLUDED.card_json, last_seen = NOW(), ttl_seconds = EXCLUDED.ttl_seconds
		RETURNING id, room_id, agent_name, card_json, joined_at, last_seen, ttl_seconds
	`

	var record models.AgentPresenceRecord
	err := r.pool.QueryRow(ctx, query,
		params.RoomID,
		params.AgentName,
		params.CardJSON,
		params.TTLSeconds,
	).Scan(
		&record.ID,
		&record.RoomID,
		&record.AgentName,
		&record.CardJSON,
		&record.JoinedAt,
		&record.LastSeen,
		&record.TTLSeconds,
	)
	if err != nil {
		LogQueryError(ctx, "Upsert", "agent_presence", err)
		return nil, err
	}

	return &record, nil
}

// Remove deletes an agent presence record by room_id and agent_name.
func (r *AgentPresenceRepository) Remove(ctx context.Context, roomID uuid.UUID, agentName string) error {
	query := `DELETE FROM agent_presence WHERE room_id = $1 AND agent_name = $2`
	_, err := r.pool.Exec(ctx, query, roomID, agentName)
	if err != nil {
		LogQueryError(ctx, "Remove", "agent_presence", err)
		return err
	}
	return nil
}

// ListByRoom returns live agents in a room (those within their TTL window).
func (r *AgentPresenceRepository) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]models.AgentPresenceRecord, error) {
	query := `
		SELECT id, room_id, agent_name, card_json, joined_at, last_seen, ttl_seconds
		FROM agent_presence
		WHERE room_id = $1 AND last_seen > NOW() - (ttl_seconds || ' seconds')::interval
		ORDER BY joined_at
	`

	rows, err := r.pool.Query(ctx, query, roomID)
	if err != nil {
		LogQueryError(ctx, "ListByRoom", "agent_presence", err)
		return nil, err
	}
	defer rows.Close()

	var records []models.AgentPresenceRecord
	for rows.Next() {
		var rec models.AgentPresenceRecord
		err := rows.Scan(
			&rec.ID,
			&rec.RoomID,
			&rec.AgentName,
			&rec.CardJSON,
			&rec.JoinedAt,
			&rec.LastSeen,
			&rec.TTLSeconds,
		)
		if err != nil {
			LogQueryError(ctx, "ListByRoom.Scan", "agent_presence", err)
			return nil, fmt.Errorf("scan presence: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if records == nil {
		records = []models.AgentPresenceRecord{}
	}

	return records, nil
}

// UpdateHeartbeat updates the last_seen timestamp for an agent in a room.
func (r *AgentPresenceRepository) UpdateHeartbeat(ctx context.Context, roomID uuid.UUID, agentName string) error {
	query := `UPDATE agent_presence SET last_seen = NOW() WHERE room_id = $1 AND agent_name = $2`
	_, err := r.pool.Exec(ctx, query, roomID, agentName)
	if err != nil {
		LogQueryError(ctx, "UpdateHeartbeat", "agent_presence", err)
		return err
	}
	return nil
}

// DeleteExpired removes expired agent presence records and returns the removed entries.
// Used by the reaper job to emit presence_leave events (per D-26/D-27).
func (r *AgentPresenceRepository) DeleteExpired(ctx context.Context) ([]models.ExpiredPresence, error) {
	query := `
		DELETE FROM agent_presence
		WHERE last_seen < NOW() - (ttl_seconds || ' seconds')::interval
		RETURNING room_id, agent_name
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "DeleteExpired", "agent_presence", err)
		return nil, err
	}
	defer rows.Close()

	var expired []models.ExpiredPresence
	for rows.Next() {
		var ep models.ExpiredPresence
		err := rows.Scan(&ep.RoomID, &ep.AgentName)
		if err != nil {
			LogQueryError(ctx, "DeleteExpired.Scan", "agent_presence", err)
			return nil, fmt.Errorf("scan expired presence: %w", err)
		}
		expired = append(expired, ep)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if expired == nil {
		expired = []models.ExpiredPresence{}
	}

	return expired, nil
}

// ListAllPublic returns all live agent presence records in non-private rooms.
// Used by the discovery endpoint.
func (r *AgentPresenceRepository) ListAllPublic(ctx context.Context) ([]models.AgentPresenceRecord, error) {
	query := `
		SELECT ap.id, ap.room_id, ap.agent_name, ap.card_json, ap.joined_at, ap.last_seen, ap.ttl_seconds
		FROM agent_presence ap
		JOIN rooms r ON r.id = ap.room_id
		WHERE r.is_private = FALSE
		AND ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval
		ORDER BY ap.last_seen DESC
	`

	rows, err := r.pool.Query(ctx, query)
	if err != nil {
		LogQueryError(ctx, "ListAllPublic", "agent_presence", err)
		return nil, err
	}
	defer rows.Close()

	var records []models.AgentPresenceRecord
	for rows.Next() {
		var rec models.AgentPresenceRecord
		err := rows.Scan(
			&rec.ID,
			&rec.RoomID,
			&rec.AgentName,
			&rec.CardJSON,
			&rec.JoinedAt,
			&rec.LastSeen,
			&rec.TTLSeconds,
		)
		if err != nil {
			LogQueryError(ctx, "ListAllPublic.Scan", "agent_presence", err)
			return nil, fmt.Errorf("scan presence: %w", err)
		}
		records = append(records, rec)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if records == nil {
		records = []models.AgentPresenceRecord{}
	}

	return records, nil
}
