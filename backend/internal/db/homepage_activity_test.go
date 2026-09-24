package db_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The public room activity feed.
//
// Everything asserted here is about showing WORK rather than TRANSPORT, and
// about an entry disappearing the moment it stops being eligible:
//
//   - messages and typed coordination events share one chronological timeline;
//   - heartbeats, token operations and internal instrumentation are invisible,
//     because typed events pass an allow-list rather than a noise blocklist;
//   - private rooms, deleted rooms, deleted messages and system notices never
//     appear, and a room taken private takes its history with it;
//   - an edited message is re-read from the row on the next request;
//   - an identity is only "verified" when a per-agent room token stamped
//     author_id — a typed event carries no authenticated identity at all.
//
// The whole suite shares one database and other packages post room messages
// while these tests run, so global counts are checked against an independently
// written census taken in the same breath rather than against a fixed number.

// feedFixture is one isolated slice of room activity, namespaced by suffix.
type feedFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *db.Pool
	suffix string
	rooms  []uuid.UUID
	agents []string
}

func newFeedFixture(t *testing.T, ctx context.Context, pool *db.Pool) *feedFixture {
	t.Helper()
	f := &feedFixture{
		t:      t,
		ctx:    ctx,
		pool:   pool,
		suffix: sanitizeSlugPart(fmt.Sprintf("hf%s", time.Now().Format("150405.000000")[:12])),
	}
	t.Cleanup(f.cleanup)
	return f
}

func (f *feedFixture) cleanup() {
	for _, id := range f.rooms {
		f.pool.Exec(f.ctx, `DELETE FROM room_events WHERE room_id = $1`, id)       //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM messages WHERE room_id = $1`, id)          //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM room_agent_tokens WHERE room_id = $1`, id) //nolint:errcheck
		f.pool.Exec(f.ctx, `DELETE FROM rooms WHERE id = $1`, id)                  //nolint:errcheck
	}
	for _, id := range f.agents {
		f.pool.Exec(f.ctx, `DELETE FROM agents WHERE id = $1`, id) //nolint:errcheck
	}
}

func (f *feedFixture) room(name string, private bool) uuid.UUID {
	f.t.Helper()
	slug := fmt.Sprintf("%s-%s", f.suffix, name)

	var id uuid.UUID
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO rooms (slug, display_name, is_private)
		VALUES ($1, $2, $3)
		RETURNING id
	`, slug, "Feed "+name, private).Scan(&id)
	require.NoError(f.t, err, "insert room %s", slug)

	f.rooms = append(f.rooms, id)
	return id
}

// message inserts one message. authorID nil means it was posted with the
// SHARED room token: real, but the name is only claimed, never authenticated.
func (f *feedFixture) message(roomID uuid.UUID, authorType, author string, authorID *string, content, metadata string, deleted bool) int64 {
	f.t.Helper()

	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}
	if metadata == "" {
		metadata = "{}"
	}

	var id int64
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO messages (room_id, author_type, author_id, agent_name, content, metadata, deleted_at, sequence_num)
		VALUES ($1, $2, $3, $4, $5, $6::jsonb, $7,
		        (SELECT COALESCE(MAX(sequence_num), 0) + 1 FROM messages WHERE room_id = $1 AND deleted_at IS NULL))
		RETURNING id
	`, roomID, authorType, authorID, author, content, metadata, deletedAt).Scan(&id)
	require.NoError(f.t, err, "insert message in %s", roomID)
	return id
}

func (f *feedFixture) event(roomID uuid.UUID, eventType, issue, actor string) int64 {
	f.t.Helper()

	var id int64
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO room_events (room_id, event_type, issue, actor)
		VALUES ($1, $2, $3, $4)
		RETURNING id
	`, roomID, eventType, issue, actor).Scan(&id)
	require.NoError(f.t, err, "insert event %s", eventType)
	return id
}

// mine returns only the entries belonging to this fixture's rooms, so ambient
// traffic from other packages cannot change what is asserted.
func (f *feedFixture) mine(entries []db.PublicRoomFeedEntry) []db.PublicRoomFeedEntry {
	out := make([]db.PublicRoomFeedEntry, 0, len(entries))
	for _, e := range entries {
		if len(e.RoomSlug) >= len(f.suffix) && e.RoomSlug[:len(f.suffix)] == f.suffix {
			out = append(out, e)
		}
	}
	return out
}

// censusFeedSince recomputes "eligible entries newer than t" from the stated
// definitions, written a different way (EXISTS subqueries instead of a UNION
// over joins). Ambient rows land in both numbers, so a wrong predicate still
// shows up as a disagreement while concurrent traffic cannot manufacture one.
func censusFeedSince(t *testing.T, ctx context.Context, pool *db.Pool, since time.Time) int {
	t.Helper()

	var count int
	err := pool.QueryRow(ctx, `
		SELECT
			(SELECT COUNT(*) FROM messages m
			  WHERE m.created_at > $1 AND m.deleted_at IS NULL AND m.author_type <> 'system'
			    AND EXISTS (SELECT 1 FROM rooms r
			                 WHERE r.id = m.room_id AND r.is_private = FALSE AND r.deleted_at IS NULL))
		  + (SELECT COUNT(*) FROM room_events e
			  WHERE e.created_at > $1
			    AND upper(e.event_type) = ANY($2::text[])
			    AND EXISTS (SELECT 1 FROM rooms r
			                 WHERE r.id = e.room_id AND r.is_private = FALSE AND r.deleted_at IS NULL))
	`, since, db.PublicFeedEventTypes).Scan(&count)
	require.NoError(t, err)
	return count
}

func TestPublicRoomFeed_MessagesAndTypedEventsShareOneTimeline(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	room := f.room("alpha", false)
	f.message(room, "agent", "planner_one", nil, "here is the plan", "", false)
	eventID := f.event(room, "CLAIM", "board render", "executor_one")

	entries, err := repo.ListPublicRoomFeed(ctx, 100, 0)
	require.NoError(t, err)

	mine := f.mine(entries)
	require.Len(t, mine, 2, "a message and a typed event are both activity")

	// Newest first: the event was written last.
	assert.Equal(t, "event", mine[0].Kind)
	assert.Equal(t, eventID, mine[0].EntryID)
	assert.Equal(t, "CLAIM", mine[0].EventType)
	assert.Equal(t, "board render", mine[0].Issue)
	assert.Equal(t, "executor_one", mine[0].AuthorName)
	assert.Empty(t, mine[0].Content, "an event has no message body")

	assert.Equal(t, "message", mine[1].Kind)
	assert.Equal(t, "here is the plan", mine[1].Content)
	assert.Equal(t, "agent", mine[1].AuthorType)
	assert.Equal(t, "planner_one", mine[1].AuthorName)
	assert.NotNil(t, mine[1].SequenceNum, "a room message carries its anchor")
}

func TestPublicRoomFeed_TransportNoiseAndInstrumentationAreInvisible(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	room := f.room("noise", false)
	for _, noisy := range []string{"HEARTBEAT", "PING", "TOKEN_ISSUED", "TOKEN_REVOKED", "PRESENCE", "JOIN"} {
		f.event(room, noisy, "", "executor_one")
	}
	f.event(room, db.RoomActivationEventType, "", "system") // internal milestone
	f.event(room, "MERGED", "board render", "executor_one")

	entries, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)

	mine := f.mine(entries)
	require.Len(t, mine, 1, "only the work event survives the allow-list: %+v", mine)
	assert.Equal(t, "MERGED", mine[0].EventType)
}

func TestPublicRoomFeed_PrivateDeletedAndSystemNeverAppear(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	public := f.room("public", false)
	private := f.room("private", true)

	f.message(public, "agent", "planner_one", nil, "visible work", "", false)
	f.message(public, "agent", "planner_one", nil, "deleted work", "", true)
	f.message(public, "system", "system", nil, "agent joined the room", "", false)
	f.message(private, "agent", "planner_one", nil, "confidential work", "", false)
	f.event(private, "CLAIM", "secret issue", "planner_one")

	entries, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)

	mine := f.mine(entries)
	require.Len(t, mine, 1)
	assert.Equal(t, "visible work", mine[0].Content)

	for _, e := range entries {
		assert.NotContains(t, e.Content, "confidential work")
		assert.NotContains(t, e.Content, "deleted work")
		assert.NotContains(t, e.RoomSlug, f.suffix+"-private")
		assert.NotEqual(t, "secret issue", e.Issue)
	}
}

func TestPublicRoomFeed_EligibilityIsRereadOnEveryRequest(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	room := f.room("moderated", false)
	edited := f.message(room, "agent", "planner_one", nil, "first wording", "", false)
	removed := f.message(room, "agent", "executor_one", nil, "will be removed", "", false)

	before, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)
	require.Len(t, f.mine(before), 2)

	_, err = pool.Exec(ctx, `UPDATE messages SET content = 'corrected wording' WHERE id = $1`, edited)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE messages SET deleted_at = NOW() WHERE id = $1`, removed)
	require.NoError(t, err)

	after, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)

	mine := f.mine(after)
	require.Len(t, mine, 1, "the removed message is gone from the next read")
	assert.Equal(t, "corrected wording", mine[0].Content, "the edit is re-read, never cached")

	// And a room taken private takes its history with it.
	_, err = pool.Exec(ctx, `UPDATE rooms SET is_private = TRUE WHERE id = $1`, room)
	require.NoError(t, err)

	hidden, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)
	assert.Empty(t, f.mine(hidden), "a room taken private leaves the public stream entirely")
}

func TestPublicRoomFeed_VerifiedIdentityComesFromAnAuthenticatedToken(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	room := f.room("identity", false)
	authorID := "agent_" + f.suffix
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name) VALUES ($1, $1) ON CONFLICT (id) DO NOTHING`, authorID)
	require.NoError(t, err)
	f.agents = append(f.agents, authorID)

	f.message(room, "agent", "claimed_name", nil, "posted with the shared token", "", false)
	f.message(room, "agent", "proven_name", &authorID, "posted with a per-agent token", "", false)
	f.event(room, "BUILDING", "board render", "proven_name")

	entries, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)

	mine := f.mine(entries)
	require.Len(t, mine, 3)

	byAuthor := map[string]db.PublicRoomFeedEntry{}
	for _, e := range mine {
		byAuthor[e.AuthorName+"/"+e.Kind] = e
	}

	assert.False(t, byAuthor["claimed_name/message"].AuthorVerified, "a shared-token name is only claimed")
	assert.True(t, byAuthor["proven_name/message"].AuthorVerified, "author_id is an authenticated identity")
	assert.False(t, byAuthor["proven_name/event"].AuthorVerified,
		"a typed event carries no authenticated identity, whatever name it states")
}

func TestCountPublicRoomFeedSince_AgreesWithAnIndependentCensus(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	var since time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT NOW()`).Scan(&since))

	room := f.room("since", false)
	f.message(room, "agent", "planner_one", nil, "new work", "", false)
	f.event(room, "CLAIM", "board render", "executor_one")
	f.event(room, "HEARTBEAT", "", "executor_one")                 // transport
	f.event(room, db.RoomActivationEventType, "", "system")        // instrumentation
	f.message(room, "system", "system", nil, "joined", "", false)  // system notice
	f.message(room, "agent", "planner_one", nil, "gone", "", true) // deleted

	count, err := repo.CountPublicRoomFeedSince(ctx, since, 500)
	require.NoError(t, err)

	assert.Equal(t, censusFeedSince(t, ctx, pool, since), count,
		"the new-activity count uses exactly the same eligibility as the page")
	assert.GreaterOrEqual(t, count, 2, "my own eligible message and event are both new")
}

func TestCountPublicRoomFeedSince_IsCappedSoALongAbsenceStaysCheap(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	var since time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT NOW()`).Scan(&since))

	room := f.room("capped", false)
	for i := 0; i < 5; i++ {
		f.message(room, "agent", "planner_one", nil, fmt.Sprintf("line %d", i), "", false)
	}

	count, err := repo.CountPublicRoomFeedSince(ctx, since, 3)
	require.NoError(t, err)
	assert.Equal(t, 3, count, "the count stops at the cap instead of scanning everything")
}

func TestPublicRoomFeed_HumanAuthorsAreNamedNeverIdentified(t *testing.T) {
	// A human room comment stores agent_name as "human:<user uuid>" — a raw
	// account identifier that must never reach a public page. The feed resolves
	// the public username instead, and says nothing at all when it cannot.
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	room := f.room("humans", false)

	var userID uuid.UUID
	username := "fx" + f.suffix
	if len(username) > 30 {
		username = username[:30]
	}
	err := pool.QueryRow(ctx, `
		INSERT INTO users (username, display_name, email, referral_code)
		VALUES ($1, $1, $2, substr(md5(random()::text), 1, 8))
		RETURNING id
	`, username, username+"@example.test").Scan(&userID)
	require.NoError(t, err)
	t.Cleanup(func() {
		pool.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID) //nolint:errcheck
	})

	known := userID.String()
	f.message(room, "human", "human:"+known, &known, "a person commented", "", false)

	unknown := uuid.New().String()
	f.message(room, "human", "human:"+unknown, &unknown, "a departed account commented", "", false)

	entries, err := repo.ListPublicRoomFeed(ctx, 200, 0)
	require.NoError(t, err)

	mine := f.mine(entries)
	require.Len(t, mine, 2)

	byContent := map[string]db.PublicRoomFeedEntry{}
	for _, e := range mine {
		byContent[e.Content] = e
		assert.NotContains(t, e.AuthorName, "human:", "raw account identifiers never leave the database")
		assert.NotContains(t, e.AuthorName, known)
		assert.NotContains(t, e.AuthorName, unknown)
	}

	assert.Equal(t, username, byContent["a person commented"].AuthorName,
		"the public username is the name a human is shown under")
	assert.Empty(t, byContent["a departed account commented"].AuthorName,
		"an unresolvable account is anonymous, not an identifier")
}

func TestPublicRoomFeed_SillyArgumentsFallBackInsteadOfFailing(t *testing.T) {
	// A caller that asks for nothing, or for a negative page, gets the default
	// page rather than an error or an unbounded scan.
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	f := newFeedFixture(t, ctx, pool)
	repo := db.NewHomepageRepository(pool)

	room := f.room("defaults", false)
	for i := 0; i < 8; i++ {
		f.message(room, "agent", "planner_one", nil, fmt.Sprintf("line %d", i), "", false)
	}

	entries, err := repo.ListPublicRoomFeed(ctx, 0, -5)
	require.NoError(t, err)
	assert.LessOrEqual(t, len(entries), 6, "no limit means the default page, not the whole table")
	assert.NotEmpty(t, entries)

	var since time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT NOW() - INTERVAL '1 minute'`).Scan(&since))

	count, err := repo.CountPublicRoomFeedSince(ctx, since, 0)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, count, 8, "no cap means the default cap, not zero")
}
