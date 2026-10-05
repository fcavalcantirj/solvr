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
// The funnel joins the channels a connection attempt crosses: the BROWSER
// (a panel opened, a prompt copied, a room viewed), the site's WEB SERVER (the
// skill link of the copied sentence fetched) and the SERVER (a room created, a
// participant joined, the first two-way exchange). Browser and web-server steps
// are reported through the ingest endpoint; server steps are recorded here from
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

// FunnelSource is the public room or post a step is attributed to (idx 88). The zero
// value means unattributed. The API resolves it from a public identifier; a client's
// raw text never reaches the row.
type FunnelSource struct {
	Kind string // models.FunnelSourceKindRoom | models.FunnelSourceKindPost
	ID   uuid.UUID
}

// columns returns the (source_kind, source_id) values to store: both NULL when unset.
func (s FunnelSource) columns() (any, any) {
	if s.Kind == "" || s.ID == uuid.Nil {
		return nil, nil
	}
	return s.Kind, s.ID
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
	Source             FunnelSource
}

// RecordBrowserEvent stores one browser-reported funnel step. The caller has
// already validated that EventName is a browser event and classified the actor.
func (r *FunnelEventRepository) RecordBrowserEvent(ctx context.Context, ev BrowserFunnelEvent) error {
	srcKind, srcID := ev.Source.columns()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref,
			preset, role, entry_surface, instruction_version, source_kind, source_id
		) VALUES ($1, $2, 'browser', $3, $4, $5, $6, $7, $8, $9, $10)
	`,
		nullFunnel(ev.FlowID), ev.EventName, ev.ActorType, nullFunnel(ev.ActorRef),
		nullFunnel(ev.Preset), nullFunnel(ev.Role), nullFunnel(ev.EntrySurface),
		nullFunnel(ev.InstructionVersion), srcKind, srcID,
	)
	if err != nil {
		LogQueryError(ctx, "RecordBrowserEvent", "funnel_events", err)
		return fmt.Errorf("record funnel browser event: %w", err)
	}
	return nil
}

// RecordSkillFetched stores the web server's skill_fetched step: the skill link of a
// copied sentence (skill.md?f=<flow code>) was fetched. The caller has validated the flow
// code and decided the entry surface (models.FunnelSurfaceAgentFetch or
// models.FunnelSurfaceBrowserVisit); nothing else a client sent reaches the row. Every
// fetch is a row: the reports count distinct flows, not fetches.
func (r *FunnelEventRepository) RecordSkillFetched(ctx context.Context, flowID, actorType, actorRef, entrySurface string) error {
	if !models.ValidFunnelActorType(actorType) {
		actorType = models.FunnelActorAnonymous
	}
	_, err := r.pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref, entry_surface
		) VALUES ($1, 'skill_fetched', 'web_server', $2, $3, $4)
	`, nullFunnel(flowID), actorType, nullFunnel(actorRef), nullFunnel(entrySurface))
	if err != nil {
		LogQueryError(ctx, "RecordSkillFetched", "funnel_events", err)
		return fmt.Errorf("record funnel skill_fetched: %w", err)
	}
	return nil
}

// FlowKnown reports whether a flow code is KNOWN: at least one funnel step other than
// room_created already carries it (the visit that copied the sentence, or the fetch of
// its skill link). POST /v1/rooms asks this before it keeps a flow code, so a room is
// only ever attributed to a flow that exists; a room cannot vouch for the code it
// brought. One lookup by flow_id, served by idx_funnel_flow.
func (r *FunnelEventRepository) FlowKnown(ctx context.Context, flowID string) (bool, error) {
	if flowID == "" {
		return false, nil
	}
	var known bool
	err := r.pool.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM funnel_events WHERE flow_id = $1 AND event_name <> 'room_created'
		)`, flowID).Scan(&known)
	if err != nil {
		LogQueryError(ctx, "FlowKnown", "funnel_events", err)
		return false, fmt.Errorf("look up funnel flow: %w", err)
	}
	return known, nil
}

// RecordRoomCreated records the server room_created step. It carries the flow_id
// the create-room call brought, which is how a browser step and this room's later
// server steps are stitched into one attempt. Deduped once per room.
func (r *FunnelEventRepository) RecordRoomCreated(ctx context.Context, roomID uuid.UUID, actorType, actorRef, flowID string) error {
	return r.RecordRoomCreatedFrom(ctx, roomID, actorType, actorRef, flowID, FunnelSource{})
}

// RecordRoomCreatedFrom is RecordRoomCreated for a room seeded from a public source
// (a room or a post). The room's later server steps inherit the source from this row.
func (r *FunnelEventRepository) RecordRoomCreatedFrom(ctx context.Context, roomID uuid.UUID, actorType, actorRef, flowID string, src FunnelSource) error {
	if !models.ValidFunnelActorType(actorType) {
		actorType = models.FunnelActorAnonymous
	}
	srcKind, srcID := src.columns()
	_, err := r.pool.Exec(ctx, `
		INSERT INTO funnel_events (
			flow_id, event_name, source_channel, actor_type, actor_ref, room_id, source_kind, source_id
		) VALUES ($1, 'room_created', 'server', $2, $3, $4, $5, $6)
		ON CONFLICT (room_id) WHERE event_name = 'room_created' AND room_id IS NOT NULL
		DO NOTHING
	`, nullFunnel(flowID), actorType, nullFunnel(actorRef), roomID, srcKind, srcID)
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
			flow_id, event_name, source_channel, actor_type, actor_ref, room_id, ordinal,
			source_kind, source_id
		)
		SELECT
			(SELECT flow_id FROM funnel_events
			  WHERE room_id = $1 AND event_name = 'room_created' LIMIT 1),
			'participant_joined', 'server', $2, $3, $1,
			(SELECT COUNT(*) FROM funnel_events
			  WHERE room_id = $1 AND event_name = 'participant_joined') + 1,
			created.source_kind, created.source_id
		FROM (SELECT 1) AS one
		LEFT JOIN funnel_events created
		  ON created.room_id = $1 AND created.event_name = 'room_created'
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
			flow_id, event_name, source_channel, actor_type, room_id, source_kind, source_id
		)
		SELECT
			(SELECT flow_id FROM funnel_events
			  WHERE room_id = $1 AND event_name = 'room_created' LIMIT 1),
			'first_two_way_exchange', 'server', 'agent', $1,
			created.source_kind, created.source_id
		FROM (SELECT 1) AS one
		LEFT JOIN funnel_events created
		  ON created.room_id = $1 AND created.event_name = 'room_created'
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
		       COALESCE(instruction_version, ''), COALESCE(source_kind, ''),
		       COALESCE(source_id::text, ''), occurred_at
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
			&e.InstructionVersion, &e.SourceKind, &e.SourceID, &e.OccurredAt); err != nil {
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
