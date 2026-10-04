package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// FeaturedRoom is one room in the operator's homepage pool (SPEC Part 26,
// "Featured rooms"). AskSeq and OutcomeSeq optionally name the messages the
// card quotes; nil lets the API pick them.
type FeaturedRoom struct {
	RoomID     uuid.UUID
	Slug       string
	FeaturedAt time.Time
	AskSeq     *int
	OutcomeSeq *int
	// Public is false while the room is private or deleted: it stays in the
	// pool but is never shown until it is public again.
	Public bool
}

// FeaturedRoomRepository reads and curates the featured pool.
type FeaturedRoomRepository struct {
	pool *Pool
}

// NewFeaturedRoomRepository wires the repository to the pool.
func NewFeaturedRoomRepository(pool *Pool) *FeaturedRoomRepository {
	return &FeaturedRoomRepository{pool: pool}
}

// Feature adds a public, live room to the pool, or updates the messages it
// quotes while keeping its place. A missing, private or deleted room is
// ErrRoomNotFound: the homepage may never show it.
func (r *FeaturedRoomRepository) Feature(ctx context.Context, slug string, askSeq, outcomeSeq *int) (*FeaturedRoom, error) {
	fr := FeaturedRoom{Slug: slug, Public: true}
	err := r.pool.QueryRow(ctx, `
		INSERT INTO featured_rooms (room_id, ask_seq, outcome_seq)
		SELECT id, $2, $3 FROM rooms
		 WHERE slug = $1 AND is_private = false AND deleted_at IS NULL
		ON CONFLICT (room_id) DO UPDATE
		   SET ask_seq = EXCLUDED.ask_seq, outcome_seq = EXCLUDED.outcome_seq
		RETURNING room_id, featured_at, ask_seq, outcome_seq
	`, slug, askSeq, outcomeSeq).Scan(&fr.RoomID, &fr.FeaturedAt, &fr.AskSeq, &fr.OutcomeSeq)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrRoomNotFound
	}
	if err != nil {
		LogQueryError(ctx, "FeatureRoom", "featured_rooms", err)
		return nil, fmt.Errorf("feature room: %w", err)
	}
	return &fr, nil
}

// Unfeature removes a room from the pool. It reports whether the room was in it.
func (r *FeaturedRoomRepository) Unfeature(ctx context.Context, slug string) (bool, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM featured_rooms
		 WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)
	`, slug)
	if err != nil {
		LogQueryError(ctx, "UnfeatureRoom", "featured_rooms", err)
		return false, fmt.Errorf("unfeature room: %w", err)
	}
	return tag.RowsAffected() > 0, nil
}

// ListPublic returns the rooms the homepage may show, oldest feature first.
func (r *FeaturedRoomRepository) ListPublic(ctx context.Context) ([]FeaturedRoom, error) {
	return r.list(ctx, true)
}

// ListAll returns the whole pool, including rooms currently private or deleted.
func (r *FeaturedRoomRepository) ListAll(ctx context.Context) ([]FeaturedRoom, error) {
	return r.list(ctx, false)
}

func (r *FeaturedRoomRepository) list(ctx context.Context, publicOnly bool) ([]FeaturedRoom, error) {
	rows, err := r.pool.Query(ctx, `
		SELECT f.room_id, ro.slug, f.featured_at, f.ask_seq, f.outcome_seq,
		       (ro.is_private = false AND ro.deleted_at IS NULL) AS public
		  FROM featured_rooms f
		  JOIN rooms ro ON ro.id = f.room_id
		 WHERE NOT $1 OR (ro.is_private = false AND ro.deleted_at IS NULL)
		 ORDER BY f.featured_at ASC, ro.slug ASC
	`, publicOnly)
	if err != nil {
		LogQueryError(ctx, "ListFeaturedRooms", "featured_rooms", err)
		return nil, fmt.Errorf("list featured rooms: %w", err)
	}
	defer rows.Close()

	var out []FeaturedRoom
	for rows.Next() {
		var fr FeaturedRoom
		if err := rows.Scan(&fr.RoomID, &fr.Slug, &fr.FeaturedAt, &fr.AskSeq, &fr.OutcomeSeq, &fr.Public); err != nil {
			return nil, fmt.Errorf("scan featured room: %w", err)
		}
		out = append(out, fr)
	}
	return out, rows.Err()
}

// featuredMessageColumns are read from the messages view (room_entries).
const featuredMessageColumns = `id, room_id, author_type, author_id, agent_name, content,
	content_type, metadata, sequence_num, created_at, pinned_at`

// featuredSpoken limits a query to what participants said: no system entries,
// nothing deleted.
const featuredSpoken = `room_id = $1 AND deleted_at IS NULL AND author_type <> 'system'`

// FindRoomBookends returns what the room set out to do (ask) and what came out
// of it (outcome). The ask is the named message, else the earliest pinned entry,
// else the first message. The outcome comes after the ask and is never the
// same message: the named one, else the latest pinned entry, else the last
// message. An empty room has neither; a room with one message has no outcome.
func (r *FeaturedRoomRepository) FindRoomBookends(ctx context.Context, roomID uuid.UUID, askSeq, outcomeSeq *int) (*models.Message, *models.Message, error) {
	var ask *models.Message
	var err error
	if askSeq != nil {
		if ask, err = r.oneMessage(ctx, `AND sequence_num = $2 ORDER BY id LIMIT 1`, roomID, *askSeq); err != nil {
			return nil, nil, err
		}
	}
	if ask == nil {
		if ask, err = r.oneMessage(ctx, `AND pinned_at IS NOT NULL ORDER BY created_at ASC, id ASC LIMIT 1`, roomID); err != nil {
			return nil, nil, err
		}
	}
	if ask == nil {
		if ask, err = r.oneMessage(ctx, `ORDER BY created_at ASC, id ASC LIMIT 1`, roomID); err != nil {
			return nil, nil, err
		}
	}
	if ask == nil {
		return nil, nil, nil
	}

	// Everything the outcome may be: after the ask, never the ask itself.
	const after = `AND id <> $2 AND (created_at, id) > ($3, $2)`
	var outcome *models.Message
	if outcomeSeq != nil {
		if outcome, err = r.oneMessage(ctx, after+` AND sequence_num = $4 ORDER BY id LIMIT 1`, roomID, ask.ID, ask.CreatedAt, *outcomeSeq); err != nil {
			return nil, nil, err
		}
	}
	if outcome == nil {
		if outcome, err = r.oneMessage(ctx, after+` AND pinned_at IS NOT NULL ORDER BY created_at DESC, id DESC LIMIT 1`, roomID, ask.ID, ask.CreatedAt); err != nil {
			return nil, nil, err
		}
	}
	if outcome == nil {
		if outcome, err = r.oneMessage(ctx, after+` ORDER BY created_at DESC, id DESC LIMIT 1`, roomID, ask.ID, ask.CreatedAt); err != nil {
			return nil, nil, err
		}
	}
	return ask, outcome, nil
}

// oneMessage reads at most one spoken message matching the tail of a query.
func (r *FeaturedRoomRepository) oneMessage(ctx context.Context, tail string, args ...any) (*models.Message, error) {
	var m models.Message
	err := r.pool.QueryRow(ctx,
		`SELECT `+featuredMessageColumns+` FROM messages WHERE `+featuredSpoken+` `+tail, args...,
	).Scan(&m.ID, &m.RoomID, &m.AuthorType, &m.AuthorID, &m.AgentName, &m.Content,
		&m.ContentType, &m.Metadata, &m.SequenceNum, &m.CreatedAt, &m.PinnedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		LogQueryError(ctx, "FindRoomBookends", "messages", err)
		return nil, fmt.Errorf("find room bookend: %w", err)
	}
	return &m, nil
}
