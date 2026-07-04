package db

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// RoomEventRepository handles typed room events (mission #4).
type RoomEventRepository struct {
	pool *Pool
}

// NewRoomEventRepository creates a new RoomEventRepository.
func NewRoomEventRepository(pool *Pool) *RoomEventRepository {
	return &RoomEventRepository{pool: pool}
}

// Create appends a typed event to a room.
func (r *RoomEventRepository) Create(ctx context.Context, p models.CreateRoomEventParams) (*models.RoomEvent, error) {
	payload := p.Payload
	if payload == nil {
		payload = json.RawMessage(`{}`)
	}
	query := `
		INSERT INTO room_events (room_id, event_type, issue, actor, payload)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, room_id, event_type, issue, actor, payload, created_at
	`
	var e models.RoomEvent
	err := r.pool.QueryRow(ctx, query, p.RoomID, p.EventType, p.Issue, p.Actor, payload).Scan(
		&e.ID, &e.RoomID, &e.EventType, &e.Issue, &e.Actor, &e.Payload, &e.CreatedAt,
	)
	if err != nil {
		LogQueryError(ctx, "Create", "room_events", err)
		return nil, err
	}
	return &e, nil
}

// Query returns events for a room, optionally filtered by event type and/or issue,
// newest first. Empty filter values match everything on that dimension.
func (r *RoomEventRepository) Query(ctx context.Context, p models.QueryRoomEventsParams) ([]models.RoomEvent, error) {
	limit := p.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}

	// Build the WHERE clause dynamically; the ($x = '' OR col = $x) idiom keeps the
	// query planner able to use the (room_id, event_type) / (room_id, issue) indexes
	// when a filter is provided and is a no-op when it is empty.
	query := `
		SELECT id, room_id, event_type, issue, actor, payload, created_at
		FROM room_events
		WHERE room_id = $1
		  AND ($2 = '' OR event_type = $2)
		  AND ($3 = '' OR issue = $3)
		ORDER BY id DESC
		LIMIT $4
	`
	rows, err := r.pool.Query(ctx, query, p.RoomID, p.EventType, p.Issue, limit)
	if err != nil {
		LogQueryError(ctx, "Query", "room_events", err)
		return nil, err
	}
	defer rows.Close()

	events := []models.RoomEvent{}
	for rows.Next() {
		var e models.RoomEvent
		if err := rows.Scan(&e.ID, &e.RoomID, &e.EventType, &e.Issue, &e.Actor, &e.Payload, &e.CreatedAt); err != nil {
			LogQueryError(ctx, "Query.Scan", "room_events", err)
			return nil, fmt.Errorf("scan room event: %w", err)
		}
		events = append(events, e)
	}
	return events, rows.Err()
}

// LatestByIssue returns the most recent event for a given issue in a room, or nil if
// none. Useful for "who currently holds APP-185" style lookups.
func (r *RoomEventRepository) LatestByIssue(ctx context.Context, roomID uuid.UUID, issue string) (*models.RoomEvent, error) {
	events, err := r.Query(ctx, models.QueryRoomEventsParams{RoomID: roomID, Issue: issue, Limit: 1})
	if err != nil {
		return nil, err
	}
	if len(events) == 0 {
		return nil, nil
	}
	return &events[0], nil
}
