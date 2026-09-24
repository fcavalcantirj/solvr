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

// ErrRoomEntryNotFound is returned when an entry lookup finds no matching,
// non-deleted entry in the given room.
var ErrRoomEntryNotFound = errors.New("room entry not found")

// ErrCrossRoomReference is returned when a reply references an entry that is not in
// the same room (task: "enforce same-room reply and addressing references").
var ErrCrossRoomReference = errors.New("reference target is not in the same room")

// ErrInvalidEntryReference is returned when a write's reply_to_entry_id,
// supersedes_entry_id or addressed_member_ids does not reference an entry or
// participant of the same room (enforced by the room_entries_allocate trigger).
var ErrInvalidEntryReference = errors.New("entry reference is not part of this room")

// sameRoomReferenceConstraint is the constraint name the timeline trigger raises for a
// foreign reply, supersede or addressing reference.
const sameRoomReferenceConstraint = "room_entries_same_room_reference"

// asInvalidEntryReference maps the trigger's same-room violation to
// ErrInvalidEntryReference and returns any other error unchanged.
func asInvalidEntryReference(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == sameRoomReferenceConstraint {
		return fmt.Errorf("%w: %s", ErrInvalidEntryReference, pgErr.Message)
	}
	return err
}

// roomEntryColumns is the shared read column list. client_entry_id is a write/lookup-
// only idempotency key and is intentionally never surfaced in read responses.
const roomEntryColumns = `id, room_id, sequence, kind, author_type, author_id, actor_label, ` +
	`body, content_type, reply_to_entry_id, addressed_member_ids, supersedes_entry_id, pinned_at, ` +
	`event_type, issue, extension, created_at, deleted_at`

func scanRoomEntry(s rowScanner) (models.RoomEntry, error) {
	var e models.RoomEntry
	err := s.Scan(
		&e.ID, &e.RoomID, &e.Sequence, &e.Kind, &e.AuthorType, &e.AuthorID, &e.ActorLabel,
		&e.Body, &e.ContentType, &e.ReplyToEntryID, &e.AddressedMemberIDs, &e.SupersedesEntryID, &e.PinnedAt,
		&e.EventType, &e.Issue, &e.Extension, &e.CreatedAt, &e.DeletedAt,
	)
	return e, err
}

// RoomEntryRepository is the storage layer for the unified room timeline.
type RoomEntryRepository struct {
	pool *Pool
}

// NewRoomEntryRepository creates a new RoomEntryRepository.
func NewRoomEntryRepository(pool *Pool) *RoomEntryRepository {
	return &RoomEntryRepository{pool: pool}
}

// Create inserts one entry and reports whether a new row was created.
//
// The per-room sequence (and the entry id) is allocated by the room_entries_allocate
// trigger (migration 000094) within the inserting transaction while it holds the room
// row lock, so concurrent commits preserve a stable order in which id order matches
// sequence order, rather than relying on a nontransactional global sequence. A message must carry a body and an event must
// carry an event_type (enforced by the DB CHECK). A reply must reference an entry in
// the same room. When ClientEntryID is set for an authenticated author, a retry with
// the same (room_id, author_id, client_entry_id) returns the existing entry with
// created=false instead of duplicating it.
func (r *RoomEntryRepository) Create(ctx context.Context, params models.CreateRoomEntryParams) (*models.RoomEntry, bool, error) {
	dedupable := params.ClientEntryID != nil && *params.ClientEntryID != "" && params.AuthorID != nil

	var out *models.RoomEntry
	created := false

	err := r.pool.WithTx(ctx, func(tx Tx) error {
		if dedupable {
			existing, gerr := getRoomEntryByClientEntry(ctx, tx, params.RoomID, *params.AuthorID, *params.ClientEntryID)
			if gerr == nil {
				out = existing
				return nil
			}
			if !errors.Is(gerr, pgx.ErrNoRows) {
				return gerr
			}
		}

		if params.ReplyToEntryID != nil {
			var same bool
			if serr := tx.QueryRow(ctx,
				`SELECT EXISTS(SELECT 1 FROM room_entries WHERE id = $1 AND room_id = $2)`,
				*params.ReplyToEntryID, params.RoomID,
			).Scan(&same); serr != nil {
				return serr
			}
			if !same {
				return ErrCrossRoomReference
			}
		}

		extension := params.Extension
		if extension == nil {
			extension = json.RawMessage(`{}`)
		}
		contentType := params.ContentType
		if contentType == "" {
			contentType = "text"
		}

		query := `
			INSERT INTO room_entries
				(room_id, kind, author_type, author_id, actor_label, body, content_type,
				 reply_to_entry_id, addressed_member_ids, supersedes_entry_id, event_type, issue, extension, client_entry_id)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
			RETURNING ` + roomEntryColumns

		entry, ierr := scanRoomEntry(tx.QueryRow(ctx, query,
			params.RoomID, params.Kind, params.AuthorType, params.AuthorID, params.ActorLabel,
			params.Body, contentType, params.ReplyToEntryID, params.AddressedMemberIDs,
			params.SupersedesEntryID, params.EventType, params.Issue, extension, params.ClientEntryID,
		))
		if ierr != nil {
			var pgErr *pgconn.PgError
			if dedupable && errors.As(ierr, &pgErr) && pgErr.Code == "23505" {
				if existing, gerr := getRoomEntryByClientEntry(ctx, tx, params.RoomID, *params.AuthorID, *params.ClientEntryID); gerr == nil {
					out = existing
					return nil
				}
			}
			return asInvalidEntryReference(ierr)
		}
		out = &entry
		created = true
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrCrossRoomReference) && !errors.Is(err, ErrInvalidEntryReference) {
			LogQueryError(ctx, "Create", "room_entries", err)
		}
		return nil, false, err
	}
	return out, created, nil
}

func getRoomEntryByClientEntry(ctx context.Context, tx Tx, roomID uuid.UUID, authorID, clientEntryID string) (*models.RoomEntry, error) {
	entry, err := scanRoomEntry(tx.QueryRow(ctx, `SELECT `+roomEntryColumns+`
		FROM room_entries
		WHERE room_id = $1 AND author_id = $2 AND client_entry_id = $3 AND deleted_at IS NULL
		ORDER BY id ASC LIMIT 1`, roomID, authorID, clientEntryID))
	if err != nil {
		return nil, err
	}
	return &entry, nil
}

// GetByID returns a single non-deleted entry by id, scoped to the room so a deep link
// cannot fetch another room's content. Returns ErrRoomEntryNotFound when absent.
func (r *RoomEntryRepository) GetByID(ctx context.Context, roomID uuid.UUID, id int64) (*models.RoomEntry, error) {
	entry, err := scanRoomEntry(r.pool.QueryRow(ctx, `SELECT `+roomEntryColumns+`
		FROM room_entries WHERE room_id = $1 AND id = $2 AND deleted_at IS NULL`, roomID, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, ErrRoomEntryNotFound
		}
		LogQueryError(ctx, "GetByID", "room_entries", err)
		return nil, err
	}
	return &entry, nil
}

// ListRecent returns the most recent non-deleted MESSAGE entries in oldest->newest
// order (the transcript). Events are excluded; they are read through QueryEvents.
func (r *RoomEntryRepository) ListRecent(ctx context.Context, roomID uuid.UUID, limit int) ([]models.RoomEntry, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `SELECT `+roomEntryColumns+` FROM (
			SELECT `+roomEntryColumns+`
			FROM room_entries
			WHERE room_id = $1 AND kind = 'message' AND deleted_at IS NULL
			ORDER BY sequence DESC LIMIT $2
		) t ORDER BY sequence ASC`, roomID, limit)
	if err != nil {
		LogQueryError(ctx, "ListRecent", "room_entries", err)
		return nil, err
	}
	return collectRoomEntries(ctx, rows, "ListRecent")
}

// ListAll returns every non-deleted entry (messages and events) in timeline order.
// Used for full-timeline replay and structured-event-inclusive reads.
func (r *RoomEntryRepository) ListAll(ctx context.Context, roomID uuid.UUID) ([]models.RoomEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+roomEntryColumns+`
		FROM room_entries
		WHERE room_id = $1 AND deleted_at IS NULL
		ORDER BY sequence ASC`, roomID)
	if err != nil {
		LogQueryError(ctx, "ListAll", "room_entries", err)
		return nil, err
	}
	return collectRoomEntries(ctx, rows, "ListAll")
}

// ListPage returns up to limit non-deleted entries (messages and events) with a
// sequence greater than afterSequence, in timeline order. kind filters to "message" or
// "event" when non-empty. Callers request limit+1 to learn whether more entries exist.
func (r *RoomEntryRepository) ListPage(ctx context.Context, roomID uuid.UUID, afterSequence int, kind string, limit int) ([]models.RoomEntry, error) {
	rows, err := r.pool.Query(ctx, `SELECT `+roomEntryColumns+`
		FROM room_entries
		WHERE room_id = $1 AND sequence > $2 AND deleted_at IS NULL
		  AND ($3 = '' OR kind = $3)
		ORDER BY sequence ASC LIMIT $4`, roomID, afterSequence, kind, limit)
	if err != nil {
		LogQueryError(ctx, "ListPage", "room_entries", err)
		return nil, err
	}
	return collectRoomEntries(ctx, rows, "ListPage")
}

// ListEventsAfter returns up to limit non-deleted EVENT entries with an id greater than
// afterID, in timeline order. Messages and events share one entry id space, so an SSE
// reconnect cursor (the last delivered entry id) resumes both kinds from the same point.
func (r *RoomEntryRepository) ListEventsAfter(ctx context.Context, roomID uuid.UUID, afterID int64, limit int) ([]models.RoomEntry, error) {
	if limit <= 0 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `SELECT `+roomEntryColumns+`
		FROM room_entries
		WHERE room_id = $1 AND kind = 'event' AND id > $2 AND deleted_at IS NULL
		ORDER BY id ASC LIMIT $3`, roomID, afterID, limit)
	if err != nil {
		LogQueryError(ctx, "ListEventsAfter", "room_entries", err)
		return nil, err
	}
	return collectRoomEntries(ctx, rows, "ListEventsAfter")
}

// CountMessages counts non-deleted MESSAGE entries in a room; events are excluded so a
// room's message count matches the legacy messages-table semantics.
func (r *RoomEntryRepository) CountMessages(ctx context.Context, roomID uuid.UUID) (int, error) {
	var n int
	if err := r.pool.QueryRow(ctx,
		`SELECT COUNT(*) FROM room_entries WHERE room_id = $1 AND kind = 'message' AND deleted_at IS NULL`,
		roomID,
	).Scan(&n); err != nil {
		LogQueryError(ctx, "CountMessages", "room_entries", err)
		return 0, err
	}
	return n, nil
}

// QueryEvents returns event entries for a room, newest first, retaining the original
// room_events type and issue filters (empty means "any").
func (r *RoomEntryRepository) QueryEvents(ctx context.Context, p models.QueryRoomEntryEventsParams) ([]models.RoomEntry, error) {
	limit := p.Limit
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.pool.Query(ctx, `SELECT `+roomEntryColumns+`
		FROM room_entries
		WHERE room_id = $1 AND kind = 'event' AND deleted_at IS NULL
		  AND ($2 = '' OR event_type = $2)
		  AND ($3 = '' OR issue = $3)
		ORDER BY id DESC LIMIT $4`, p.RoomID, p.EventType, p.Issue, limit)
	if err != nil {
		LogQueryError(ctx, "QueryEvents", "room_entries", err)
		return nil, err
	}
	return collectRoomEntries(ctx, rows, "QueryEvents")
}

// ResolveLegacy maps an old messages.id / room_events.id to its new entry id so deep
// links and SSE cursors resolve during the transition. Returns ErrRoomEntryNotFound
// when the legacy id was never migrated.
func (r *RoomEntryRepository) ResolveLegacy(ctx context.Context, legacyKind string, legacyID int64) (int64, error) {
	var entryID int64
	err := r.pool.QueryRow(ctx,
		`SELECT entry_id FROM room_entry_legacy_map WHERE legacy_kind = $1 AND legacy_id = $2`,
		legacyKind, legacyID,
	).Scan(&entryID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrRoomEntryNotFound
		}
		LogQueryError(ctx, "ResolveLegacy", "room_entry_legacy_map", err)
		return 0, err
	}
	return entryID, nil
}

func collectRoomEntries(ctx context.Context, rows pgx.Rows, op string) ([]models.RoomEntry, error) {
	defer rows.Close()
	entries := []models.RoomEntry{}
	for rows.Next() {
		e, err := scanRoomEntry(rows)
		if err != nil {
			LogQueryError(ctx, op+".Scan", "room_entries", err)
			return nil, fmt.Errorf("scan room entry: %w", err)
		}
		entries = append(entries, e)
	}
	return entries, rows.Err()
}

// BackfillFromLegacy migrates rows from the frozen legacy_messages and
// legacy_room_events archives into room_entries via room_entries_backfill_legacy()
// (migration 000094, the same routine the cutover ran). Message entries keep their
// legacy id so old links and SSE cursors resolve by identity; events are mapped in
// room_entry_legacy_map. It is idempotent: re-running inserts nothing for already-mapped
// rows. Returns the number of new entries inserted.
func (r *RoomEntryRepository) BackfillFromLegacy(ctx context.Context) (int, error) {
	var inserted int
	if err := r.pool.QueryRow(ctx, `SELECT room_entries_backfill_legacy()`).Scan(&inserted); err != nil {
		LogQueryError(ctx, "BackfillFromLegacy", "room_entries", err)
		return 0, err
	}
	return inserted, nil
}
