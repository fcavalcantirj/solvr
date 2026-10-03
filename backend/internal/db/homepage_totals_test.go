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

// The all-time totals behind the homepage's "All time" section.
//
// These numbers answer "how big is Solvr", and the whole point of the section
// is that they are NOT usage. A registration is a registration: an agent that
// registered a key and never called anything still counts here, and this file
// holds the repository to saying so honestly.
//
// Rooms are reported twice, and the two figures must never be confused:
// AllRooms counts EVERY non-deleted room, private ones included — a room's
// existence is a platform fact — while PublicRooms counts only the rooms anyone
// can read, so a private room never inflates the public figure.
//
// Four exclusions matter and each one has its own assertion below:
//   - a PRIVATE room is never a PUBLIC room;
//   - a DELETED row of any kind is gone;
//   - a post that was never published (draft, pending review, rejected) is not
//     part of the published knowledge base, and neither is a family-scoped one;
//   - a SUSPENDED agent is not a registered agent any more.
//
// An EXPIRED public room, on the other hand, IS counted. It is archived, not
// erased: the collaboration happened, it is still readable, and dropping it
// would make Solvr look smaller every time a room went quiet.
//
// Every assertion is a DELTA against a baseline taken before the fixture is
// written, so the test is correct on a database that already holds rows.

type allTimeFixture struct {
	t      *testing.T
	ctx    context.Context
	pool   *db.Pool
	suffix string
	rooms  []uuid.UUID
	posts  []uuid.UUID
	agents []string
	users  []uuid.UUID
	author string
}

func newAllTimeFixture(t *testing.T, ctx context.Context, pool *db.Pool) *allTimeFixture {
	t.Helper()
	return &allTimeFixture{
		t:      t,
		ctx:    ctx,
		pool:   pool,
		suffix: sanitizeSlugPart(fmt.Sprintf("at%s", time.Now().Format("150405.000000")[:12])),
	}
}

// cleanup removes every row the fixture wrote.
//
// It must be DEFERRED by the caller, never registered with t.Cleanup: the pool
// is closed in a deferred func, deferred funcs run BEFORE t.Cleanup funcs, and
// a closed pool swallows every delete without erroring — which leaves the
// fixture in the database forever.
func (f *allTimeFixture) cleanup() {
	for _, id := range f.rooms {
		f.pool.Exec(f.ctx, `DELETE FROM rooms WHERE id = $1`, id) //nolint:errcheck
	}
	for _, id := range f.posts {
		f.pool.Exec(f.ctx, `DELETE FROM posts WHERE id = $1`, id) //nolint:errcheck
	}
	for _, id := range f.agents {
		f.pool.Exec(f.ctx, `DELETE FROM agents WHERE id = $1`, id) //nolint:errcheck
	}
	for _, id := range f.users {
		f.pool.Exec(f.ctx, `DELETE FROM users WHERE id = $1`, id) //nolint:errcheck
	}
}

func (f *allTimeFixture) room(name string, private bool, expiresAt *time.Time, deleted bool) uuid.UUID {
	f.t.Helper()

	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}

	var id uuid.UUID
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO rooms (slug, display_name, is_private, expires_at, deleted_at)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id
	`, fmt.Sprintf("%s-%s", f.suffix, name), "Room "+name, private, expiresAt, deletedAt).Scan(&id)
	require.NoError(f.t, err, "insert room %s", name)

	f.rooms = append(f.rooms, id)
	return id
}

func (f *allTimeFixture) post(name, status, visibility string, deleted bool) uuid.UUID {
	f.t.Helper()

	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}

	if f.author == "" {
		// A post names an existing author (000117). A suspended agent is not a registered
		// agent, so the author leaves the account totals to the accounts a test adds itself;
		// PublishedPosts counts posts, whatever their author's status.
		f.author = f.agent("author", "suspended", false)
	}

	var id uuid.UUID
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, visibility, deleted_at)
		VALUES ('post', $1, 'homepage totals fixture', 'agent', $2, $3, $4, $5)
		RETURNING id
	`, f.suffix+" "+name, f.author, status, visibility, deletedAt).Scan(&id)
	require.NoError(f.t, err, "insert post %s/%s/%s", name, status, visibility)

	f.posts = append(f.posts, id)
	return id
}

func (f *allTimeFixture) agent(name, status string, deleted bool) string {
	f.t.Helper()

	id := f.suffix + "-" + name
	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}

	_, err := f.pool.Exec(f.ctx, `
		INSERT INTO agents (id, display_name, status, deleted_at)
		VALUES ($1, $1, $2, $3)
	`, id, status, deletedAt)
	require.NoError(f.t, err, "insert agent %s/%s", name, status)

	f.agents = append(f.agents, id)
	return id
}

func (f *allTimeFixture) user(name string, deleted bool) uuid.UUID {
	f.t.Helper()

	var deletedAt *time.Time
	if deleted {
		now := time.Now()
		deletedAt = &now
	}

	handle := f.suffix + name
	var id uuid.UUID
	err := f.pool.QueryRow(f.ctx, `
		INSERT INTO users (username, display_name, email, auth_provider, auth_provider_id, referral_code, deleted_at)
		VALUES ($1, $1, $2, 'test', $1, $3, $4)
		RETURNING id
	`, handle, handle+"@example.test", handle[len(handle)-8:], deletedAt).Scan(&id)
	require.NoError(f.t, err, "insert user %s", name)

	f.users = append(f.users, id)
	return id
}

func TestGetAllTimeTotals_CountsScaleAndExcludesWhatIsNotPublic(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)

	before, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)
	require.NotNil(t, before)

	f := newAllTimeFixture(t, ctx, pool)
	defer f.cleanup()

	// Rooms: two live public rooms and one EXPIRED public room all count —
	// expiry archives a room, it does not erase the collaboration.
	expired := time.Now().Add(-48 * time.Hour)
	f.room("live-a", false, nil, false)
	f.room("live-b", false, nil, false)
	f.room("archived", false, &expired, false)
	// Neither of these may ever reach a public total.
	f.room("private", true, nil, false)
	f.room("deleted", false, nil, true)

	// Posts: only what was actually published counts, and each post is ONE
	// post no matter which legacy view (/problems, /questions, /ideas) reads it.
	f.post("open", "open", "public", false)
	f.post("stale", "stale", "public", false)
	f.post("draft", "draft", "public", false)
	f.post("queued", "pending_review", "public", false)
	f.post("rejected", "rejected", "public", false)
	f.post("family", "open", "family", false)
	f.post("gone", "open", "public", true)

	// Accounts: a registration, minus the ones that are gone or suspended.
	f.agent("registered", "active", false)
	f.agent("suspended", "suspended", false)
	f.agent("deleted", "active", true)
	f.user("live", false)
	f.user("gone", true)

	after, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)
	require.NotNil(t, after)

	assert.Equal(t, 3, after.PublicRooms-before.PublicRooms,
		"two live public rooms plus the expired one; the private and deleted rooms are invisible")
	assert.Equal(t, 2, after.PublishedPosts-before.PublishedPosts,
		"only the published, public, undeleted posts count")
	assert.Equal(t, 1, after.RegisteredAgents-before.RegisteredAgents,
		"a suspended or deleted agent is not a registered agent")
	assert.Equal(t, 1, after.RegisteredHumans-before.RegisteredHumans,
		"a deleted account is not a registered human")
}

func TestGetAllTimeTotals_CountsEveryRoomBesideThePublicOnes(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)

	before, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)
	require.NotNil(t, before)

	f := newAllTimeFixture(t, ctx, pool)
	defer f.cleanup()

	expired := time.Now().Add(-48 * time.Hour)
	f.room("public", false, nil, false)
	f.room("public-archived", false, &expired, false)
	f.room("private", true, nil, false)
	f.room("private-archived", true, &expired, false)
	f.room("public-deleted", false, nil, true)
	f.room("private-deleted", true, nil, true)

	after, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)
	require.NotNil(t, after)

	assert.Equal(t, 4, after.AllRooms-before.AllRooms,
		"every room that was not deleted counts, private and expired ones included")
	assert.Equal(t, 2, after.PublicRooms-before.PublicRooms,
		"the public figure beside it counts the public rooms only")

	var allRooms, publicRooms int
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT COUNT(*), COUNT(*) FILTER (WHERE NOT is_private) FROM rooms WHERE deleted_at IS NULL
	`).Scan(&allRooms, &publicRooms))
	assert.Equal(t, allRooms, after.AllRooms, "the total is the database's answer")
	assert.Equal(t, publicRooms, after.PublicRooms)
}

func TestGetAllTimeTotals_RepliesAreNotPosts(t *testing.T) {
	pool, ctx, done := newRoomStatsPool(t)
	defer done()

	repo := db.NewHomepageRepository(pool)
	f := newAllTimeFixture(t, ctx, pool)
	defer f.cleanup()

	problem := f.post("with-replies", "open", "public", false)
	author := f.agent("replier", "active", false)

	before, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)

	for i := range 3 {
		_, err := pool.Exec(ctx, `
			INSERT INTO replies (post_id, author_type, author_id, body)
			VALUES ($1, 'agent', $2, $3)
		`, problem, author, fmt.Sprintf("%s reply %d", f.suffix, i))
		require.NoError(t, err)
	}
	// Registered after f.cleanup so LIFO drops the replies before the post
	// they hang off.
	defer func() {
		pool.Exec(ctx, `DELETE FROM replies WHERE post_id = $1`, problem) //nolint:errcheck
	}()

	after, err := repo.GetAllTimeTotals(ctx)
	require.NoError(t, err)

	assert.Equal(t, before.PublishedPosts, after.PublishedPosts,
		"three replies on one post do not make four posts")
}
