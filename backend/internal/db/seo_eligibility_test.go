package db

import (
	"context"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 80: a public room transcript is worth indexing when it carries a two-way
// exchange: live messages from at least two distinct non-system authors as the page
// shows them. A word count plays no part. The room page's seo.indexable and the rooms
// sitemap use the same rule, so they can never disagree.
func TestRoomIndexable_RequiresATwoWayExchange(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx := context.Background()
	repo := NewRoomRepository(pool)
	sitemap := NewSitemapRepository(pool)

	newRoom := func(slug string, private bool) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, is_private)
			VALUES ($1, $2, $3) RETURNING id`, slug, slug, private).Scan(&id))
		return id
	}
	say := func(room uuid.UUID, name, authorType string, authorID *string) int64 {
		t.Helper()
		var id int64
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO messages (room_id, agent_name, author_type, author_id, content)
			VALUES ($1, $2, $3, $4, 'a message') RETURNING id`, room, name, authorType, authorID).Scan(&id))
		return id
	}
	indexable := func(room uuid.UUID) bool {
		t.Helper()
		ok, err := repo.IsIndexable(ctx, room)
		require.NoError(t, err)
		return ok
	}
	listed := func(slug string) bool {
		t.Helper()
		page, err := sitemap.GetPaginatedSitemapURLs(ctx, models.SitemapURLsOptions{Type: "rooms", Page: 1, PerPage: 100})
		require.NoError(t, err)
		all, err := sitemap.GetSitemapURLs(ctx)
		require.NoError(t, err)
		inPage, inAll := false, false
		for _, r := range page.Rooms {
			inPage = inPage || r.Slug == slug
		}
		for _, r := range all.Rooms {
			inAll = inAll || r.Slug == slug
		}
		require.Equal(t, inPage, inAll, "the paged and flat sitemap listings disagree on %s", slug)
		return inPage
	}
	count := func() int {
		t.Helper()
		c, err := sitemap.GetSitemapCounts(ctx)
		require.NoError(t, err)
		return c.Rooms
	}

	room := newRoom("seo-two-way", false)
	assert.False(t, indexable(room), "an empty room is thin")
	assert.False(t, listed("seo-two-way"))
	assert.Equal(t, 0, count())

	planner := "agent-planner-id"
	say(room, "planner", "agent", &planner)
	say(room, "planner", "agent", &planner)
	say(room, "system", "system", nil)
	assert.False(t, indexable(room), "one author talking alone plus a system line is not an exchange")
	assert.False(t, listed("seo-two-way"))

	// Two sessions of one agent key, talking as planner and executor, are a discussion
	// a reader sees (the room.activated rule), unlike the activation funnel's identity.
	executor := "agent-planner-id"
	reply := say(room, "executor", "agent", &executor)
	assert.True(t, indexable(room), "two authors exchanged messages")
	assert.True(t, listed("seo-two-way"))
	assert.Equal(t, 1, count())

	_, err := pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NOW() WHERE id = $1`, reply)
	require.NoError(t, err)
	assert.False(t, indexable(room), "a deleted reply no longer counts")
	assert.False(t, listed("seo-two-way"))
	assert.Equal(t, 0, count())

	// A human reply in the browser is a second participant too.
	say(room, "reviewer-human", "human", nil)
	assert.True(t, indexable(room), "a human and an agent exchanged messages")

	private := newRoom("seo-private", true)
	say(private, "a", "agent", &planner)
	say(private, "b", "agent", &executor)
	assert.False(t, indexable(private), "a private room is never indexable")
	assert.False(t, listed("seo-private"))

	gone := newRoom("seo-deleted", false)
	say(gone, "a", "agent", &planner)
	say(gone, "b", "agent", &executor)
	_, err = pool.Exec(ctx, `UPDATE rooms SET deleted_at = NOW() WHERE id = $1`, gone)
	require.NoError(t, err)
	assert.False(t, indexable(gone), "a deleted room is never indexable")
	assert.False(t, listed("seo-deleted"))
}
