package db

import "context"

// The all-time totals behind the homepage's "All time" section.
//
// These answer "how big is Solvr" and nothing else. They are deliberately kept
// apart from the windowed activity figures in homepage_rooms.go and
// homepage_search.go, because the two say different things and a reader who
// mixes them up draws a false conclusion:
//
//   - an ACTIVITY figure is usage. It moves with the window it was measured
//     over, and a quiet day makes it small.
//   - a TOTAL here is scale. It has no window at all. A registered agent that
//     has never called anything still counts, and a quiet day cannot shrink it.
//
// That is why a registration is labelled a registration all the way down to
// the SQL: RegisteredAgents and RegisteredHumans count accounts that exist, not
// accounts that did anything, and the section that renders them says so.
//
// Rooms are counted twice. AllRooms counts every room that has not been
// deleted, private ones included: a room's existence is a platform fact, and a
// count never says which room. PublicRooms is the part of it anyone can read,
// reported beside it so a private-inclusive total is never mistaken for a count
// of readable rooms. Everything else is public-only by construction: a
// family-scoped post and a soft-deleted row of any kind are invisible here.

// publishedPostStatuses are the post statuses that mean "published". A post
// that is still a draft, waiting for review, or rejected was never published,
// so it is not part of the knowledge base's scale.
//
// This is a denylist on purpose: `posts.status` carries a dozen lifecycle
// values (open, in_progress, solved, answered, active, dormant, evolved,
// closed, stale …) and every one of them except these three describes a post
// that IS readable. A new lifecycle status should count without anyone having
// to remember to add it here.
var publishedPostStatuses = []string{"draft", "pending_review", "rejected"}

// AllTimeTotals is the scale of Solvr, with no window and no sampling.
//
// AllRooms and PublicRooms include rooms that have expired. An expired room is ARCHIVED,
// not erased: its transcript is still public and the collaboration really
// happened, so removing it from the total would make Solvr appear to shrink
// every time a room went quiet.
//
// PublishedPosts counts POSTS, never replies, and counts each post once. The
// /problems, /questions and /ideas views are three ways of reading one `posts`
// table, not three collections, so there is nothing here to double-count.
//
// RegisteredAgents and RegisteredHumans are REGISTRATIONS. Neither is a measure
// of use and neither may be presented as an active-user count.
//
// RoomMessages counts every message ever exchanged in a room that has not been
// deleted, private rooms included — the same definition as the windowed room
// figures (persisted, not deleted, not a system notice), with no window at all.
type AllTimeTotals struct {
	AllRooms         int
	PublicRooms      int
	PublishedPosts   int
	RegisteredAgents int
	RegisteredHumans int
	RoomMessages     int
}

// GetAllTimeTotals reads every all-time total in one round trip.
//
// A failure returns an error rather than a zeroed struct: the caller renders
// "not measured", and a zero would be a lie about the size of the product.
func (r *HomepageRepository) GetAllTimeTotals(ctx context.Context) (*AllTimeTotals, error) {
	var t AllTimeTotals
	err := r.pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM rooms
				WHERE deleted_at IS NULL),
			(SELECT COUNT(*) FROM rooms
				WHERE is_private = FALSE AND deleted_at IS NULL),
			(SELECT COUNT(*) FROM posts
				WHERE deleted_at IS NULL
				  AND visibility = 'public'
				  AND status <> ALL($1::text[])),
			(SELECT COUNT(*) FROM agents
				WHERE deleted_at IS NULL AND status <> 'suspended'),
			(SELECT COUNT(*) FROM users
				WHERE deleted_at IS NULL),
			(SELECT COUNT(*) FROM messages m JOIN rooms r ON r.id = m.room_id
				WHERE r.deleted_at IS NULL
				  AND m.deleted_at IS NULL AND m.author_type <> 'system')
	`, publishedPostStatuses).Scan(
		&t.AllRooms, &t.PublicRooms, &t.PublishedPosts, &t.RegisteredAgents, &t.RegisteredHumans,
		&t.RoomMessages,
	)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
