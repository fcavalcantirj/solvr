package db

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Recording the connection funnel.
//
// The funnel joins two channels a connection attempt crosses: the BROWSER
// (a panel opened, a prompt copied, a room viewed) and the SERVER (a room
// created, a participant joined, the first two-way exchange). Browser steps are
// self-reported through the ingest endpoint; server steps are recorded here from
// confirmed server actions, so a client can neither fake a room creation nor
// suppress the measurement by blocking browser analytics.
//
// Three properties this file is responsible for:
//
//   - MILESTONES DEDUPE. A room is created once, its first two-way exchange
//     happens once, and an actor joins a room once. Every server write uses a
//     partial unique index and ON CONFLICT DO NOTHING so a retry, a reconnect or
//     a replayed message cannot inflate a funnel count.
//   - NO SECRETS. A row keeps a non-secret flow_id, a pseudonymous actor_ref
//     (a hash, never a raw id), an opaque room_id for internal linkage, and the
//     step's shape. Never a credential, a message body, the task text or a raw
//     private room title.
//   - BEST EFFORT. A funnel row that could not be written is a statistic that is
//     briefly short, never a room action that failed. Callers log and move on.

// FunnelEventRepository writes and reads connection-funnel steps.
type FunnelEventRepository struct {
	pool *Pool
}

// NewFunnelEventRepository creates the funnel event store.
func NewFunnelEventRepository(pool *Pool) *FunnelEventRepository {
	return &FunnelEventRepository{pool: pool}
}

// PseudonymizeActor turns a raw account or agent id into a stable pseudonymous
// reference. The same id always hashes to the same actor_ref, so distinct actors
// can be counted, but the row never carries the id itself. An empty id yields an
// empty ref (an anonymous step names nobody).
func PseudonymizeActor(id string) string {
	if id == "" {
		return ""
	}
	sum := sha256.Sum256([]byte("funnel:" + id))
	return hex.EncodeToString(sum[:16]) // 32 hex chars, well under the column bound
}

// BrowserFunnelEvent is one browser-reported step, as accepted by the ingest
// endpoint after the API classifies the actor.
type BrowserFunnelEvent struct {
	FlowID             string
	EventName          string
	ActorType          string
	ActorRef           string
	Preset             string
	Role               string
	EntrySurface       string
	InstructionVersion string
}

// RecordBrowserEvent stores one browser-reported funnel step. The caller has
// already validated that EventName is a browser event and classified the actor.
func (r *FunnelEventRepository) RecordBrowserEvent(ctx context.Context, ev BrowserFunnelEvent) error {
	_, err := r.pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref,
			preset, role, entry_surface, instruction_version
		) VALUES ($1, $2, 'browser', $3, $4, $5, $6, $7, $8)
	`,
		nullFunnel(ev.FlowID), ev.EventName, ev.ActorType, nullFunnel(ev.ActorRef),
		nullFunnel(ev.Preset), nullFunnel(ev.Role), nullFunnel(ev.EntrySurface),
		nullFunnel(ev.InstructionVersion),
	)
	if err != nil {
		LogQueryError(ctx, "RecordBrowserEvent", "funnel_events", err)
		return fmt.Errorf("record funnel browser event: %w", err)
	}
	return nil
}

// RecordRoomCreated records the server room_created step. It carries the flow_id
// the create-room call brought, which is how a browser step and this room's later
// server steps are stitched into one attempt. Deduped once per room.
func (r *FunnelEventRepository) RecordRoomCreated(ctx context.Context, roomID uuid.UUID, actorType, actorRef, flowID string) error {
	if !models.ValidFunnelActorType(actorType) {
		actorType = models.FunnelActorAnonymous
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref, room_id
		) VALUES ($1, 'room_created', 'server', $2, $3, $4)
		ON CONFLICT (room_id) WHERE event_name = 'room_created' AND room_id IS NOT NULL
		DO NOTHING
	`, nullFunnel(flowID), actorType, nullFunnel(actorRef), roomID)
	if err != nil {
		LogQueryError(ctx, "RecordRoomCreated", "funnel_events", err)
		return fmt.Errorf("record funnel room_created: %w", err)
	}
	return nil
}

// RecordParticipantJoined records the server participant_joined step with the
// joining actor's 1-based ordinal for the room. Deduped per (room, actor), so a
// reconnecting agent keeps its original ordinal and is not counted twice. The
// flow_id is filled from the room's room_created step so downstream steps of one
// attempt share it. Returns whether a new row was written and its ordinal.
func (r *FunnelEventRepository) RecordParticipantJoined(ctx context.Context, roomID uuid.UUID, actorType, actorRef string) (ordinal int, recorded bool, err error) {
	if actorRef == "" {
		// Without a stable actor reference an ordinal cannot be attributed and a
		// rejoin cannot be deduped, so the step is skipped rather than mis-counted.
		return 0, false, nil
	}
	if !models.ValidFunnelActorType(actorType) {
		actorType = models.FunnelActorAgent
	}
	row := r.pool.QueryRow(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref, room_id, ordinal
		)
		SELECT
			(SELECT flow_id FROM funnel_events
			  WHERE room_id = $1 AND event_name = 'room_created' LIMIT 1),
			'participant_joined', 'server', $2, $3, $1,
			(SELECT COUNT(*) FROM funnel_events
			  WHERE room_id = $1 AND event_name = 'participant_joined') + 1
		ON CONFLICT (room_id, actor_ref)
			WHERE event_name = 'participant_joined' AND room_id IS NOT NULL AND actor_ref IS NOT NULL
		DO NOTHING
		RETURNING ordinal
	`, roomID, actorType, actorRef)

	var ord int
	if scanErr := row.Scan(&ord); scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return 0, false, nil // already joined: deduped, not an error
		}
		LogQueryError(ctx, "RecordParticipantJoined", "funnel_events", scanErr)
		return 0, false, fmt.Errorf("record funnel participant_joined: %w", scanErr)
	}
	return ord, true, nil
}

// RecordFirstTwoWayExchange records the room's first two-way exchange milestone,
// but ONLY once at least two distinct agent identities have each posted a
// non-deleted message in the room. Called best-effort after each agent message:
// it no-ops until the condition holds, then records exactly once per room. The
// flow_id is filled from the room's room_created step. Returns whether it wrote.
func (r *FunnelEventRepository) RecordFirstTwoWayExchange(ctx context.Context, roomID uuid.UUID) (recorded bool, err error) {
	tag, err := r.pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, room_id
		)
		SELECT
			(SELECT flow_id FROM funnel_events
			  WHERE room_id = $1 AND event_name = 'room_created' LIMIT 1),
			'first_two_way_exchange', 'server', 'agent', $1
		WHERE (
			SELECT COUNT(DISTINCT COALESCE(author_id, agent_name)) FROM messages
			 WHERE room_id = $1 AND author_type = 'agent' AND deleted_at IS NULL
		) >= 2
		ON CONFLICT (room_id) WHERE event_name = 'first_two_way_exchange' AND room_id IS NOT NULL
		DO NOTHING
	`, roomID)
	if err != nil {
		LogQueryError(ctx, "RecordFirstTwoWayExchange", "funnel_events", err)
		return false, fmt.Errorf("record funnel first_two_way_exchange: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ListByFlow returns every step recorded for one flow, oldest first. It is used
// by tests and internal analysis; it is never served to a public surface.
func (r *FunnelEventRepository) ListByFlow(ctx context.Context, flowID string) ([]models.FunnelEvent, error) {
	return r.list(ctx, `WHERE flow_id = $1`, flowID)
}

// ListByRoom returns every step recorded for one room, oldest first.
func (r *FunnelEventRepository) ListByRoom(ctx context.Context, roomID uuid.UUID) ([]models.FunnelEvent, error) {
	return r.list(ctx, `WHERE room_id = $1`, roomID)
}

func (r *FunnelEventRepository) list(ctx context.Context, where string, arg any) ([]models.FunnelEvent, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT id, COALESCE(flow_id, ''), event_name, source_channel, actor_type,
		       COALESCE(actor_ref, ''), COALESCE(room_id::text, ''), COALESCE(preset, ''),
		       COALESCE(role, ''), COALESCE(ordinal, 0), COALESCE(entry_surface, ''),
		       COALESCE(instruction_version, ''), occurred_at
		  FROM funnel_events `+where+` ORDER BY id ASC`, arg)
	if err != nil {
		LogQueryError(ctx, "ListFunnelEvents", "funnel_events", err)
		return nil, fmt.Errorf("list funnel events: %w", err)
	}
	defer rows.Close()

	var out []models.FunnelEvent
	for rows.Next() {
		var e models.FunnelEvent
		if err := rows.Scan(&e.ID, &e.FlowID, &e.EventName, &e.SourceChannel, &e.ActorType,
			&e.ActorRef, &e.RoomID, &e.Preset, &e.Role, &e.Ordinal, &e.EntrySurface,
			&e.InstructionVersion, &e.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan funnel event: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// nullFunnel keeps an absent optional string NULL so the length CHECK constraints
// and partial indexes treat "not provided" distinctly from an empty value.
func nullFunnel(s string) any {
	if s == "" {
		return nil
	}
	return s
}
