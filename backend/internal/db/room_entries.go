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
// The per-room sequence is allocated within the same transaction that inserts the
// row: the room row is locked FOR UPDATE, then sequence = MAX+1 is read and written,
// so concurrent commits preserve a stable order rather than relying on a
// nontransactional global sequence. A message must carry a body and an event must
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

		// Serialize sequence allocation for this room.
		if _, lerr := tx.Exec(ctx, `SELECT id FROM rooms WHERE id = $1 FOR UPDATE`, params.RoomID); lerr != nil {
			return lerr
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
				(room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
				 reply_to_entry_id, addressed_member_ids, supersedes_entry_id, event_type, issue, extension, client_entry_id)
			VALUES ($1,
				(SELECT COALESCE(MAX(sequence), 0) + 1 FROM room_entries WHERE room_id = $1),
				$2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)
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
			return ierr
		}
		out = &entry
		created = true
		return nil
	})
	if err != nil {
		if !errors.Is(err, ErrCrossRoomReference) {
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

// backfillInsertSQL merges legacy messages and room_events into room_entries and builds
// the legacy-ID map. Idempotent and incremental: only legacy rows not already mapped are
// inserted, sequenced after the room's current MAX(sequence), merged by created_at with a
// stable tie breaker (messages before events, then legacy id). Kept in sync with the
// backfill block in migration 000093.
const backfillInsertSQL = `
WITH unified AS (
    SELECT m.room_id, 'message'::text AS kind,
           m.author_type, m.author_id, m.agent_name AS actor_label,
           m.content AS body, m.content_type,
           NULL::text AS event_type, ''::text AS issue,
           m.metadata AS extension,
           m.addressed_member_ids, m.client_entry_id, m.pinned_at,
           m.created_at, m.deleted_at,
           'message'::text AS legacy_kind, m.id AS legacy_id, 0 AS source_rank
    FROM messages m
    WHERE NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                      WHERE lm.legacy_kind = 'message' AND lm.legacy_id = m.id)
    UNION ALL
    SELECT e.room_id, 'event'::text,
           NULL, NULL, e.actor,
           NULL, 'text',
           e.event_type, e.issue,
           e.payload,
           NULL, NULL, NULL,
           e.created_at, NULL,
           'event'::text, e.id, 1
    FROM room_events e
    WHERE NOT EXISTS (SELECT 1 FROM room_entry_legacy_map lm
                      WHERE lm.legacy_kind = 'event' AND lm.legacy_id = e.id)
),
base AS (
    SELECT room_id, COALESCE(MAX(sequence), 0) AS maxseq FROM room_entries GROUP BY room_id
),
seq AS (
    SELECT u.*,
        COALESCE(b.maxseq, 0) + ROW_NUMBER() OVER (
            PARTITION BY u.room_id ORDER BY u.created_at ASC, u.source_rank ASC, u.legacy_id ASC
        ) AS sequence
    FROM unified u LEFT JOIN base b ON b.room_id = u.room_id
),
ins AS (
    INSERT INTO room_entries
        (room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
         event_type, issue, extension, addressed_member_ids, client_entry_id, pinned_at, created_at, deleted_at)
    SELECT room_id, sequence, kind, author_type, author_id, actor_label, body, content_type,
           event_type, issue, extension, addressed_member_ids, client_entry_id, pinned_at, created_at, deleted_at
    FROM seq
    RETURNING id AS entry_id, room_id, sequence
)
INSERT INTO room_entry_legacy_map (legacy_kind, legacy_id, entry_id)
SELECT s.legacy_kind, s.legacy_id, ins.entry_id
FROM ins JOIN seq s ON s.room_id = ins.room_id AND s.sequence = ins.sequence`

const backfillRemapReplySQL = `
UPDATE room_entries re
SET reply_to_entry_id = tgt.entry_id
FROM room_entry_legacy_map src
JOIN messages m ON m.id = src.legacy_id AND src.legacy_kind = 'message'
JOIN room_entry_legacy_map tgt ON tgt.legacy_kind = 'message' AND tgt.legacy_id = m.reply_to_entry_id
WHERE re.id = src.entry_id
  AND m.reply_to_entry_id IS NOT NULL
  AND re.reply_to_entry_id IS DISTINCT FROM tgt.entry_id`

const backfillRemapSupersedeSQL = `
UPDATE room_entries re
SET supersedes_entry_id = tgt.entry_id
FROM room_entry_legacy_map src
JOIN messages m ON m.id = src.legacy_id AND src.legacy_kind = 'message'
JOIN room_entry_legacy_map tgt ON tgt.legacy_kind = 'message' AND tgt.legacy_id = m.supersedes_entry_id
WHERE re.id = src.entry_id
  AND m.supersedes_entry_id IS NOT NULL
  AND re.supersedes_entry_id IS DISTINCT FROM tgt.entry_id`

// BackfillFromLegacy migrates legacy messages and room_events into room_entries and
// remaps reply/supersede references. It is idempotent: re-running inserts nothing for
// already-mapped rows. Returns the number of new entries inserted.
func (r *RoomEntryRepository) BackfillFromLegacy(ctx context.Context) (int, error) {
	var inserted int64
	err := r.pool.WithTx(ctx, func(tx Tx) error {
		tag, ierr := tx.Exec(ctx, backfillInsertSQL)
		if ierr != nil {
			return ierr
		}
		inserted = tag.RowsAffected()
		if _, rerr := tx.Exec(ctx, backfillRemapReplySQL); rerr != nil {
			return rerr
		}
		if _, serr := tx.Exec(ctx, backfillRemapSupersedeSQL); serr != nil {
			return serr
		}
		return nil
	})
	if err != nil {
		LogQueryError(ctx, "BackfillFromLegacy", "room_entries", err)
		return 0, err
	}
	return int(inserted), nil
}
