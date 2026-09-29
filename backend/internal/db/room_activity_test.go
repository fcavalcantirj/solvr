package db

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Room activity (rooms.message_count, rooms.last_active_at) is a projection of the room
// timeline (idx 77). These tests run on a scratch database so a rebuild of every room
// touches only rows they created.

type roomActivity struct {
	messages   int
	lastActive time.Time
	updatedAt  time.Time
}

func readRoomActivity(ctx context.Context, t *testing.T, pool *Pool, roomID uuid.UUID) roomActivity {
	t.Helper()
	var a roomActivity
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT message_count, last_active_at, updated_at FROM rooms WHERE id = $1`, roomID,
	).Scan(&a.messages, &a.lastActive, &a.updatedAt))
	return a
}

// newestMessageAt is the newest message entry's created_at (soft-deleted included), or
// the room's created_at when it has none.
func newestMessageAt(ctx context.Context, t *testing.T, pool *Pool, roomID uuid.UUID) time.Time {
	t.Helper()
	var at time.Time
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT GREATEST(r.created_at, COALESCE(MAX(e.created_at), r.created_at))
		FROM rooms r LEFT JOIN room_entries e ON e.room_id = r.id AND e.kind = 'message'
		WHERE r.id = $1 GROUP BY r.id, r.created_at`, roomID).Scan(&at))
	return at
}

func insertActivityRoom(ctx context.Context, t *testing.T, pool *Pool, slug string) uuid.UUID {
	t.Helper()
	var id uuid.UUID
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO rooms (slug, display_name) VALUES ($1, 'Activity Room') RETURNING id`, slug,
	).Scan(&id))
	return id
}

func postActivityMessage(ctx context.Context, t *testing.T, msgs *MessageRepository, roomID uuid.UUID, author, content string) *models.Message {
	t.Helper()
	msg, err := msgs.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: author,
		Content: content, ContentType: "text",
	})
	require.NoError(t, err)
	return msg
}

func TestRoomActivity_TheTimelineMovesTheProjectionInTheSameTransaction(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	msgs := NewMessageRepository(pool)
	entries := NewRoomEntryRepository(pool)
	rooms := NewRoomRepository(pool)

	roomID := insertActivityRoom(ctx, t, pool, "activity-timeline")
	start := readRoomActivity(ctx, t, pool, roomID)
	require.Equal(t, 0, start.messages)

	// A stored message counts and is the room's latest activity, with no extra call.
	first := postActivityMessage(ctx, t, msgs, roomID, "planner", "plan")
	second := postActivityMessage(ctx, t, msgs, roomID, "executor", "build")
	a := readRoomActivity(ctx, t, pool, roomID)
	require.Equal(t, 2, a.messages, "two stored messages")
	require.True(t, a.lastActive.Equal(second.CreatedAt), "last_active_at %v, want the newest message %v", a.lastActive, second.CreatedAt)
	require.True(t, a.updatedAt.After(start.updatedAt), "updated_at (the room ETag) advances with activity")

	// An event is not a message and not activity.
	actor := "agent"
	eventType := "CLAIM"
	_, created, err := entries.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindEvent, AuthorType: &actor, ActorLabel: "planner",
		EventType: &eventType, Issue: "APP-1",
	})
	require.NoError(t, err)
	require.True(t, created)
	b := readRoomActivity(ctx, t, pool, roomID)
	require.Equal(t, 2, b.messages, "an event is not counted")
	require.True(t, b.lastActive.Equal(a.lastActive), "an event is not message activity")

	// A replayed write (same author, same client_entry_id) stores nothing and counts nothing.
	author, key := "executor", "retry-1"
	params := models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: author,
		Content: "report", ContentType: "text", ClientEntryID: &key,
	}
	_, created, err = msgs.CreateWithClientEntry(ctx, params)
	require.NoError(t, err)
	require.True(t, created)
	_, created, err = msgs.CreateWithClientEntry(ctx, params)
	require.NoError(t, err)
	require.False(t, created, "the retry replays")
	require.Equal(t, 3, readRoomActivity(ctx, t, pool, roomID).messages, "a replay must not count twice")

	// A rolled-back write leaves no trace in the projection.
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO messages (room_id, agent_name, content) VALUES ($1, 'ghost', 'never committed')`, roomID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, 3, readRoomActivity(ctx, t, pool, roomID).messages, "a rolled-back message is not counted")

	// Soft delete and restore move the count; activity that happened stays.
	before := readRoomActivity(ctx, t, pool, roomID)
	_, err = pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NOW() WHERE id = $1`, first.ID)
	require.NoError(t, err)
	c := readRoomActivity(ctx, t, pool, roomID)
	require.Equal(t, 2, c.messages, "a soft-deleted message is not counted")
	require.True(t, c.lastActive.Equal(before.lastActive), "a soft delete does not rewind the latest activity")
	_, err = pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NULL WHERE id = $1`, first.ID)
	require.NoError(t, err)
	require.Equal(t, 3, readRoomActivity(ctx, t, pool, roomID).messages, "a restored message counts again")
	_, err = pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NOW() WHERE id = $1`, first.ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE room_entries SET body = 'edited' WHERE id = $1`, second.ID)
	require.NoError(t, err)
	require.Equal(t, 2, readRoomActivity(ctx, t, pool, roomID).messages, "an edit that is not a delete moves nothing")

	// Removing the newest message record makes the newest remaining one the latest activity.
	var newest int64
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT id FROM room_entries WHERE room_id = $1 AND kind = 'message' ORDER BY created_at DESC, id DESC LIMIT 1`, roomID,
	).Scan(&newest))
	_, err = pool.Exec(ctx, `DELETE FROM room_entries WHERE id = $1`, newest)
	require.NoError(t, err)
	d := readRoomActivity(ctx, t, pool, roomID)
	require.Equal(t, 1, d.messages, "a removed live message is not counted")
	require.True(t, d.lastActive.Equal(newestMessageAt(ctx, t, pool, roomID)), "last_active_at follows the newest remaining message")

	drift, err := rooms.ActivityDrift(ctx, &roomID)
	require.NoError(t, err)
	require.Empty(t, drift, "the maintained projection equals the timeline")
}

func TestRoomActivity_RebuildRecomputesTheProjectionFromTheTimeline(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	msgs := NewMessageRepository(pool)
	rooms := NewRoomRepository(pool)

	drifted := insertActivityRoom(ctx, t, pool, "activity-drifted")
	healthy := insertActivityRoom(ctx, t, pool, "activity-healthy")
	empty := insertActivityRoom(ctx, t, pool, "activity-empty")
	for i := 0; i < 3; i++ {
		postActivityMessage(ctx, t, msgs, drifted, "planner", fmt.Sprintf("drifted %d", i))
		postActivityMessage(ctx, t, msgs, healthy, "planner", fmt.Sprintf("healthy %d", i))
	}
	want := readRoomActivity(ctx, t, pool, drifted)
	healthyBefore := readRoomActivity(ctx, t, pool, healthy)

	// Nothing to repair: a rebuild is a no-op on a consistent database.
	n, err := rooms.RebuildActivity(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)

	// Break the stored projection the way a lost update would.
	future := time.Now().Add(48 * time.Hour).UTC().Truncate(time.Microsecond)
	_, err = pool.Exec(ctx, `UPDATE rooms SET message_count = 99, last_active_at = $2 WHERE id = $1`, drifted, future)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE rooms SET message_count = 5 WHERE id = $1`, empty)
	require.NoError(t, err)

	drift, err := rooms.ActivityDrift(ctx, nil)
	require.NoError(t, err)
	require.Len(t, drift, 2, "exactly the two broken rooms: %+v", drift)
	byRoom := map[uuid.UUID]RoomActivityDrift{}
	for _, d := range drift {
		byRoom[d.RoomID] = d
	}
	require.Equal(t, 99, byRoom[drifted].StoredMessageCount)
	require.Equal(t, 3, byRoom[drifted].TimelineMessageCount)
	require.True(t, byRoom[drifted].StoredLastActiveAt.Equal(future))
	require.True(t, byRoom[drifted].TimelineLastActiveAt.Equal(want.lastActive))
	require.Equal(t, 5, byRoom[empty].StoredMessageCount)
	require.Equal(t, 0, byRoom[empty].TimelineMessageCount)

	// One room at a time.
	n, err = rooms.RebuildActivity(ctx, &drifted)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	got := readRoomActivity(ctx, t, pool, drifted)
	require.Equal(t, 3, got.messages)
	require.True(t, got.lastActive.Equal(want.lastActive), "last_active_at %v, want %v", got.lastActive, want.lastActive)
	require.True(t, got.updatedAt.After(want.updatedAt), "a repaired room's ETag changes")
	still, err := rooms.ActivityDrift(ctx, nil)
	require.NoError(t, err)
	require.Len(t, still, 1, "the other broken room is untouched by a one-room rebuild")

	// Every room.
	n, err = rooms.RebuildActivity(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, 0, readRoomActivity(ctx, t, pool, empty).messages)
	healthyAfter := readRoomActivity(ctx, t, pool, healthy)
	require.Equal(t, healthyBefore, healthyAfter, "a consistent room is not rewritten")
	drift, err = rooms.ActivityDrift(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, drift)

	// Replaying the rebuild changes nothing.
	n, err = rooms.RebuildActivity(ctx, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n)
}

// Concurrent writers each count exactly once.
func TestRoomActivity_ConcurrentWritersCountExactlyOnce(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	msgs := NewMessageRepository(pool)
	rooms := NewRoomRepository(pool)
	roomID := insertActivityRoom(ctx, t, pool, "activity-writers")

	const writers, perWriter = 8, 25
	var wg sync.WaitGroup
	errs := make(chan error, writers*perWriter)
	for w := 0; w < writers; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			author := fmt.Sprintf("writer-%d", w)
			for i := 0; i < perWriter; i++ {
				if _, err := msgs.Create(ctx, models.CreateMessageParams{
					RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: author,
					Content: fmt.Sprintf("%s message %d", author, i), ContentType: "text",
				}); err != nil {
					errs <- err
				}
			}
		}(w)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, writers*perWriter, readRoomActivity(ctx, t, pool, roomID).messages)
	drift, err := rooms.ActivityDrift(ctx, &roomID)
	require.NoError(t, err)
	require.Empty(t, drift)
}

// A rebuild that meets an uncommitted message must not overwrite it: the rebuild waits for
// the room lock the insert holds, then counts the committed message. Without that lock
// the recount would be taken before the commit and written after it, losing the message.
func TestRoomActivity_ARebuildWaitsForAnInFlightMessage(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	msgs := NewMessageRepository(pool)
	rooms := NewRoomRepository(pool)
	roomID := insertActivityRoom(ctx, t, pool, "activity-inflight")
	for i := 0; i < 3; i++ {
		postActivityMessage(ctx, t, msgs, roomID, "planner", fmt.Sprintf("committed %d", i))
	}
	// The stored count is wrong, so the rebuild has something to write.
	_, err := pool.Exec(ctx, `UPDATE rooms SET message_count = 50 WHERE id = $1`, roomID)
	require.NoError(t, err)

	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background()) //nolint:errcheck
	_, err = tx.Exec(ctx, `INSERT INTO messages (room_id, agent_name, content) VALUES ($1, 'executor', 'in flight')`, roomID)
	require.NoError(t, err)

	done := make(chan error, 1)
	go func() {
		_, err := rooms.RebuildActivity(ctx, &roomID)
		done <- err
	}()

	// Commit only once the rebuild is blocked behind the in-flight message.
	deadline := time.Now().Add(15 * time.Second)
	for {
		var waiting int
		require.NoError(t, pool.QueryRow(ctx, `
			SELECT COUNT(*) FROM pg_stat_activity
			WHERE datname = current_database() AND wait_event_type = 'Lock'
			  AND query LIKE '%rebuild_room_activity%'`).Scan(&waiting))
		if waiting > 0 {
			break
		}
		select {
		case err := <-done:
			t.Fatalf("the rebuild finished (err=%v) while a message insert held the room", err)
		default:
		}
		require.False(t, time.Now().After(deadline), "the rebuild never waited for the in-flight message")
		time.Sleep(20 * time.Millisecond)
	}
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)

	require.Equal(t, 4, readRoomActivity(ctx, t, pool, roomID).messages, "the in-flight message is counted")
	drift, err := rooms.ActivityDrift(ctx, &roomID)
	require.NoError(t, err)
	require.Empty(t, drift)
}

// The presence reaper hard-deletes expired rooms (DeleteExpiredRooms); their entries go
// with them through the room_entries foreign key's cascade, and every cascaded message
// fires the projection's delete trigger against a room row that is already gone. That
// must be a no-op: the reaper succeeds, the entries are removed, and no other room's
// projection moves.
func TestRoomActivity_DeletingARoomCascadesThroughTheProjection(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	msgs := NewMessageRepository(pool)
	rooms := NewRoomRepository(pool)

	kept := insertActivityRoom(ctx, t, pool, "activity-kept")
	expired := insertActivityRoom(ctx, t, pool, "activity-expired")
	for i := 0; i < 2; i++ {
		postActivityMessage(ctx, t, msgs, kept, "planner", fmt.Sprintf("kept %d", i))
	}
	var doomed []*models.Message
	for i := 0; i < 3; i++ {
		doomed = append(doomed, postActivityMessage(ctx, t, msgs, expired, "executor", fmt.Sprintf("expired %d", i)))
	}
	_, err := pool.Exec(ctx, `UPDATE room_entries SET deleted_at = NOW() WHERE id = $1`, doomed[0].ID)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE rooms SET expires_at = NOW() - INTERVAL '1 hour' WHERE id = $1`, expired)
	require.NoError(t, err)
	keptBefore := readRoomActivity(ctx, t, pool, kept)

	n, err := rooms.DeleteExpiredRooms(ctx)
	require.NoError(t, err, "the cascade through the projection trigger must not fail the reaper")
	require.Equal(t, int64(1), n)

	var left int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1`, expired).Scan(&left))
	require.Zero(t, left, "the expired room's entries went with it")
	require.Equal(t, keptBefore, readRoomActivity(ctx, t, pool, kept), "another room's projection does not move")
	drift, err := rooms.ActivityDrift(ctx, nil)
	require.NoError(t, err)
	require.Empty(t, drift)
}
