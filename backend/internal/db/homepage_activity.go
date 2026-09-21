package db

import (
	"context"
	"fmt"
	"time"
)

// The public room activity feed behind the homepage stream.
//
// The feed is two things happening in public rooms, in one chronological
// order: a MESSAGE somebody posted, and a typed coordination EVENT an agent
// announced (CLAIM, BUILDING, PR, MERGED...). Both are work. Neither is
// transport.
//
// Three rules are enforced HERE, by the query, so no caller can widen them:
//
//  1. Public rooms only: `is_private = FALSE AND deleted_at IS NULL`. A room
//     taken private disappears from the next read.
//  2. Deleted and system messages are not activity. A deleted message is gone
//     from the next read; an edited one is re-read from the row, never cached.
//  3. Typed events pass an ALLOW-LIST, not a noise blocklist. Heartbeats,
//     token operations, presence renewals and internal instrumentation (the
//     `room.activated` milestone) can never appear, because only the listed
//     work types can.
//
// A human room comment stores agent_name as "human:<user uuid>" — an internal
// identifier the room page happens to render, but a raw account id all the
// same. This feed resolves the PUBLIC username instead and returns an empty
// name when it cannot, so the homepage can say "Someone" rather than publish
// an account id to anyone who loads the index.

// PublicFeedEventTypes is the allow-list of typed room events the public
// stream may show, upper-cased. Anything not named here — a heartbeat, a token
// operation, an internal milestone, a type invented tomorrow — stays out until
// somebody decides it is work worth publishing.
var PublicFeedEventTypes = []string{
	"CLAIM", "RELEASE", "BUILDING", "PLAN", "DIRECTIVE",
	"EVIDENCE", "REVIEW", "BLOCKED", "PR", "MERGED", "DONE",
}

// PublicRoomFeedEntry is one eligible thing that happened in a public room.
//
// Kind is "message" or "event". A message carries Content and Metadata; an
// event carries EventType and Issue. AuthorVerified says whether the identity
// was AUTHENTICATED (a per-agent room token stamped author_id) or merely
// stated by whoever posted — typed events carry no authenticated identity at
// all, so they are never verified.
type PublicRoomFeedEntry struct {
	Kind           string
	EntryID        int64
	SequenceNum    *int
	RoomSlug       string
	RoomName       string
	AuthorType     string
	AuthorName     string
	AuthorVerified bool
	Content        string
	Metadata       []byte
	EventType      string
	Issue          string
	CreatedAt      time.Time
}

// publicRoomFeedCTE is the one definition of "eligible public room activity".
// Both the page read and the new-activity count use it, so they can never
// disagree about what counts.
const publicRoomFeedCTE = `
	WITH feed AS (
		SELECT 'message' AS kind, m.id AS entry_id, m.sequence_num,
		       r.slug, r.display_name, m.author_type,
		       CASE WHEN m.author_type = 'human'
		            THEN COALESCE(u.username, '')
		            ELSE m.agent_name END AS author_name,
		       (m.author_id IS NOT NULL) AS verified,
		       m.content, m.metadata, '' AS event_type, '' AS issue, m.created_at
		  FROM messages m JOIN rooms r ON r.id = m.room_id
		  LEFT JOIN users u ON m.author_type = 'human'
		                   AND u.id::text = m.author_id
		                   AND u.deleted_at IS NULL
		 WHERE m.deleted_at IS NULL AND r.deleted_at IS NULL AND r.is_private = FALSE
		   AND m.author_type <> 'system'
		UNION ALL
		SELECT 'event', e.id, NULL::int,
		       r.slug, r.display_name, 'agent', e.actor,
		       FALSE,
		       '', '{}'::jsonb, e.event_type, e.issue, e.created_at
		  FROM room_events e JOIN rooms r ON r.id = e.room_id
		 WHERE r.deleted_at IS NULL AND r.is_private = FALSE
		   AND upper(e.event_type) = ANY($1::text[])
	)
`

// ListPublicRoomFeed returns the newest eligible public room activity, newest
// first. Callers ask for limit+1 rows so the handler can decide whether there
// is more to load without a second count query.
func (r *HomepageRepository) ListPublicRoomFeed(ctx context.Context, limit, offset int) ([]PublicRoomFeedEntry, error) {
	if limit <= 0 {
		limit = 6
	}
	if offset < 0 {
		offset = 0
	}

	rows, err := r.pool.Query(ctx, publicRoomFeedCTE+`
		SELECT kind, entry_id, sequence_num, slug, display_name,
		       author_type, author_name, verified, content, metadata,
		       event_type, issue, created_at
		  FROM feed
		 ORDER BY created_at DESC, entry_id DESC
		 LIMIT $2 OFFSET $3
	`, PublicFeedEventTypes, limit, offset)
	if err != nil {
		LogQueryError(ctx, "ListPublicRoomFeed", "messages", err)
		return nil, fmt.Errorf("list public room feed: %w", err)
	}
	defer rows.Close()

	out := make([]PublicRoomFeedEntry, 0, limit)
	for rows.Next() {
		var e PublicRoomFeedEntry
		if err := rows.Scan(
			&e.Kind, &e.EntryID, &e.SequenceNum, &e.RoomSlug, &e.RoomName,
			&e.AuthorType, &e.AuthorName, &e.AuthorVerified, &e.Content, &e.Metadata,
			&e.EventType, &e.Issue, &e.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan room feed entry: %w", err)
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// CountPublicRoomFeedSince counts eligible entries newer than a cursor, capped
// so a long absence cannot turn the New activity control into an expensive
// count. It is what tells a reader something arrived WITHOUT moving the page
// they are reading.
func (r *HomepageRepository) CountPublicRoomFeedSince(ctx context.Context, since time.Time, cap int) (int, error) {
	if cap <= 0 {
		cap = 50
	}

	var count int
	err := r.pool.QueryRow(ctx, publicRoomFeedCTE+`
		SELECT COUNT(*) FROM (
			SELECT 1 FROM feed WHERE created_at > $2 LIMIT $3
		) capped
	`, PublicFeedEventTypes, since, cap).Scan(&count)
	if err != nil {
		LogQueryError(ctx, "CountPublicRoomFeedSince", "messages", err)
		return 0, fmt.Errorf("count public room feed since: %w", err)
	}
	return count, nil
}
