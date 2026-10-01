package db

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// GET /v1/overview names public rooms (activity feed, previews, recent rooms: slug, display name,
// description) from a 30 s snapshot on each API instance, and only RoomHandler announced a room
// change (visibility, archive, reopen, delete; a rename through the API did not). Measured on HEAD
// before migration 000124 (idx 77 slice 11 spike, live API): a room made private, soft-deleted,
// row-deleted or renamed by SQL stayed on the overview until the snapshot ran out, while GET
// /v1/rooms/{slug} already answered 403/404. Every write that changes how a public room is shown
// now announces it on OverviewChannel at commit, whoever writes it; writes the overview cannot see
// stay quiet.
func TestOverviewChanges_WritesThatChangeAPublicRoomAnnounceIt(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	author := authorAgent(ctx, t, pool, "rov_"+uuid.NewString()[:8])
	msgs := NewMessageRepository(pool)
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}
	room := func(slug string, private bool) uuid.UUID {
		t.Helper()
		var id uuid.UUID
		require.NoError(t, pool.QueryRow(ctx, `INSERT INTO rooms (slug, display_name, description, is_private)
			VALUES ($1, $2, 'what the room is for', $3) RETURNING id`, slug, "Room "+slug, private).Scan(&id))
		return id
	}
	notices := listenOverviewNotices(ctx, t, pool)
	quiet := func(what string) {
		t.Helper()
		require.Equal(t, 0, notices.since(ctx, t), "%s must not announce an overview change", what)
	}
	once := func(what string) {
		t.Helper()
		require.Equal(t, 1, notices.since(ctx, t), "%s must announce exactly one overview change", what)
	}

	public := room("rov-public", false)
	quiet("a new room (it appears with the next snapshot)")
	postActivityMessage(ctx, t, msgs, public, author, "a message moves message_count and last_active_at")
	quiet("a message")
	exec(`UPDATE rooms SET tags = '{a,b}', category = 'ops', updated_at = NOW() WHERE id = $1`, public)
	quiet("a tags/category edit (not shown)")
	exec(`UPDATE rooms SET display_name = display_name, description = description, is_private = FALSE,
		deleted_at = NULL, archived_at = NULL, expires_at = NULL, slug = slug WHERE id = $1`, public)
	quiet("watched columns rewritten to their own values")

	private := room("rov-private", true)
	exec(`UPDATE rooms SET display_name = 'renamed in private', description = 'x', archived_at = NOW() WHERE id = $1`, private)
	exec(`UPDATE rooms SET deleted_at = NOW() WHERE id = $1`, private)
	quiet("a private room renamed, archived and deleted (never named)")
	exec(`DELETE FROM rooms WHERE id = $1`, private)
	quiet("a private room's row deleted")

	exec(`UPDATE rooms SET display_name = 'Renamed room' WHERE id = $1`, public)
	once("a public room renamed")
	exec(`UPDATE rooms SET description = 'a new purpose' WHERE id = $1`, public)
	once("a public room's description edited")
	exec(`UPDATE rooms SET slug = 'rov-public-moved' WHERE id = $1`, public)
	once("a public room's slug changed")
	exec(`UPDATE rooms SET archived_at = NOW() WHERE id = $1`, public)
	once("a public room archived")
	exec(`UPDATE rooms SET archived_at = NULL WHERE id = $1`, public)
	once("a public room reopened")
	exec(`UPDATE rooms SET expires_at = NOW() + interval '1 hour' WHERE id = $1`, public)
	once("a public room's expiry changed")
	exec(`UPDATE rooms SET is_private = TRUE WHERE id = $1`, public)
	once("a public room made private")
	exec(`UPDATE rooms SET is_private = FALSE WHERE id = $1`, public)
	once("a private room made public")
	exec(`UPDATE rooms SET deleted_at = NOW() WHERE id = $1`, public)
	once("a public room soft-deleted")
	exec(`UPDATE rooms SET display_name = 'renamed after deletion' WHERE id = $1`, public)
	quiet("a deleted room renamed")
	exec(`DELETE FROM rooms WHERE id = $1`, public)
	quiet("the row of an already soft-deleted room deleted")

	gone := room("rov-row-deleted", false)
	postActivityMessage(ctx, t, msgs, gone, author, "listed in the activity feed")
	_ = notices.since(ctx, t)
	exec(`DELETE FROM rooms WHERE id = $1`, gone)
	once("a public room's row deleted (with its messages)")

	for _, slug := range []string{"rov-bulk-1", "rov-bulk-2", "rov-bulk-3", "rov-bulk-4"} {
		room(slug, false)
	}
	_ = notices.since(ctx, t)
	exec(`UPDATE rooms SET deleted_at = NOW() WHERE slug LIKE 'rov-bulk-%'`)
	once("one statement soft-deleting 4 public rooms")
}
