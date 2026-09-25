package db

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// AgentPresenceRepository handles database operations for agent presence records.
type AgentPresenceRepository struct {
	pool *Pool
}

// NewAgentPresenceRepository creates a new AgentPresenceRepository.
func NewAgentPresenceRepository(pool *Pool) *AgentPresenceRepository {
	return &AgentPresenceRepository{pool: pool}
}

// ErrPresenceNotMember is returned when the agent has no ACTIVE membership in the room:
// presence is bound to room_members (migration 000099).
var ErrPresenceNotMember = errors.New("agent is not an active member of the room")

// ErrPresenceNameTaken is returned when another member is live in the room under the
// requested agent_name.
var ErrPresenceNameTaken = errors.New("agent_name is in use by another member of the room")

// presenceColumns is the column list every presence reader scans with scanPresence.
const presenceColumns = `id, room_id, agent_id, agent_name, card_json, joined_at, last_seen, ttl_seconds`

func scanPresence(row pgx.Row, rec *models.AgentPresenceRecord) error {
	return row.Scan(&rec.ID, &rec.RoomID, &rec.AgentID, &rec.AgentName, &rec.CardJSON,
		&rec.JoinedAt, &rec.LastSeen, &rec.TTLSeconds)
}

// Upsert records the member's presence in the room: one row per (room, agent). A repeat
// join refreshes card_json, agent_name, last_seen and ttl_seconds. An EXPIRED row of
// another member holding the same agent_name is cleared first; a live one yields
// ErrPresenceNameTaken.
func (r *AgentPresenceRepository) Upsert(ctx context.Context, params models.UpsertAgentPresenceParams) (*models.AgentPresenceRecord, error) {
	// Default card_json to empty JSON object if nil (DB column is NOT NULL)
	cardJSON := params.CardJSON
	if cardJSON == nil {
		cardJSON = json.RawMessage(`{}`)
	}

	if _, err := r.pool.Exec(ctx, `
		DELETE FROM agent_presence
		 WHERE room_id = $1 AND agent_name = $2 AND agent_id <> $3
		   AND last_seen < NOW() - (ttl_seconds || ' seconds')::interval`,
		params.RoomID, params.AgentName, params.AgentID); err != nil {
		LogQueryError(ctx, "Upsert.ClearExpiredName", "agent_presence", err)
		return nil, err
	}

	query := `
		INSERT INTO agent_presence (room_id, agent_id, agent_name, card_json, ttl_seconds)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (room_id, agent_id)
		DO UPDATE SET agent_name = EXCLUDED.agent_name, card_json = EXCLUDED.card_json,
		              last_seen = NOW(), ttl_seconds = EXCLUDED.ttl_seconds
		RETURNING ` + presenceColumns

	var record models.AgentPresenceRecord
	err := scanPresence(r.pool.QueryRow(ctx, query,
		params.RoomID, params.AgentID, params.AgentName, cardJSON, params.TTLSeconds), &record)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) {
			switch pgErr.ConstraintName {
			case "room_member_active_required", "agent_presence_membership_fkey":
				return nil, ErrPresenceNotMember
			case "agent_presence_room_id_agent_name_key":
				return nil, ErrPresenceNameTaken
			}
		}
		LogQueryError(ctx, "Upsert", "agent_presence", err)
		return nil, err
	}

	return &record, nil
}

// CurrentName returns the agent_name the member is present under in the room, or "" when
// it is not present.
func (r *AgentPresenceRepository) CurrentName(ctx context.Context, roomID uuid.UUID, agentID string) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx,
		`SELECT agent_name FROM agent_presence WHERE room_id = $1 AND agent_id = $2`, roomID, agentID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		LogQueryError(ctx, "CurrentName", "agent_presence", err)
		return "", err
	}
	return name, nil
}

// Remove deletes the member's presence in the room and returns the agent_name it was
// present under ("" when it was not present). Only the authenticated agent's own row is
// touched, whatever agent_name a caller sends.
func (r *AgentPresenceRepository) Remove(ctx context.Context, roomID uuid.UUID, agentID string) (string, error) {
	return r.returnName(ctx, "Remove",
		`DELETE FROM agent_presence WHERE room_id = $1 AND agent_id = $2 RETURNING agent_name`, roomID, agentID)
}

// ListByRoom returns live agents in a room (those within their TTL window).
func (r *AgentPresenceRepository) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]models.AgentPresenceRecord, error) {
	query := `
		SELECT ` + presenceColumns + `
		FROM agent_presence
		WHERE room_id = $1 AND last_seen > NOW() - (ttl_seconds || ' seconds')::interval
		ORDER BY joined_at
	`
	return r.list(ctx, "ListByRoom", query, roomID)
}

// LiveCard returns the card_json of the agent present in the room under agentName while its
// presence has not expired; found is false otherwise. Every API instance answers from this
// shared row, so an agent that joined through one instance is visible through all of them.
func (r *AgentPresenceRepository) LiveCard(ctx context.Context, roomID uuid.UUID, agentName string) (json.RawMessage, bool, error) {
	var card json.RawMessage
	err := r.pool.QueryRow(ctx, `
		SELECT card_json FROM agent_presence
		WHERE room_id = $1 AND agent_name = $2
		  AND last_seen > NOW() - (ttl_seconds || ' seconds')::interval`, roomID, agentName).Scan(&card)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		LogQueryError(ctx, "LiveCard", "agent_presence", err)
		return nil, false, err
	}
	return card, true, nil
}

// UpdateHeartbeat renews the member's presence in the room and returns the agent_name it
// is present under ("" when it is not present; nothing is written then).
func (r *AgentPresenceRepository) UpdateHeartbeat(ctx context.Context, roomID uuid.UUID, agentID string) (string, error) {
	return r.returnName(ctx, "UpdateHeartbeat",
		`UPDATE agent_presence SET last_seen = NOW() WHERE room_id = $1 AND agent_id = $2 RETURNING agent_name`,
		roomID, agentID)
}

func (r *AgentPresenceRepository) returnName(ctx context.Context, op, query string, roomID uuid.UUID, agentID string) (string, error) {
	var name string
	err := r.pool.QueryRow(ctx, query, roomID, agentID).Scan(&name)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", nil
	}
	if err != nil {
		LogQueryError(ctx, op, "agent_presence", err)
		return "", err
	}
	return name, nil
}

func (r *AgentPresenceRepository) list(ctx context.Context, op, query string, args ...any) ([]models.AgentPresenceRecord, error) {
	rows, err := r.pool.Query(ctx, query, args...)
	if err != nil {
		LogQueryError(ctx, op, "agent_presence", err)
		return nil, err
	}
	defer rows.Close()

	records := []models.AgentPresenceRecord{}
	for rows.Next() {
		var rec models.AgentPresenceRecord
		if err := scanPresence(rows, &rec); err != nil {
			LogQueryError(ctx, op+".Scan", "agent_presence", err)
			return nil, fmt.Errorf("scan presence: %w", err)
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
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
		SELECT ap.id, ap.room_id, ap.agent_id, ap.agent_name, ap.card_json, ap.joined_at, ap.last_seen, ap.ttl_seconds
		FROM agent_presence ap
		JOIN rooms r ON r.id = ap.room_id
		WHERE r.is_private = FALSE
		AND ap.last_seen > NOW() - (ap.ttl_seconds || ' seconds')::interval
		ORDER BY ap.last_seen DESC
	`
	return r.list(ctx, "ListAllPublic", query)
}
