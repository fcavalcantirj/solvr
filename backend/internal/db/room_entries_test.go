package db_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// entryTestCleanup removes all data a room_entries test created, in FK-safe order.
func entryTestCleanup(ctx context.Context, pool *db.Pool, roomSlug string) {
	pool.Exec(ctx, `DELETE FROM room_entry_legacy_map lm USING room_entries re
		WHERE lm.entry_id = re.id AND re.room_id = (SELECT id FROM rooms WHERE slug = $1)`, roomSlug) //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM room_entries WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", roomSlug) //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM messages WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", roomSlug)      //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM room_events WHERE room_id = (SELECT id FROM rooms WHERE slug = $1)", roomSlug)   //nolint:errcheck
	pool.Exec(ctx, "DELETE FROM rooms WHERE slug = $1", roomSlug)                                                //nolint:errcheck
}

func createEntryTestRoom(t *testing.T, ctx context.Context, pool *db.Pool, slug string) uuid.UUID {
	t.Helper()
	entryTestCleanup(ctx, pool, slug)
	_, err := pool.Exec(ctx, `
		INSERT INTO rooms (slug, display_name)
		VALUES ($1, 'Entry Test Room')
	`, slug)
	if err != nil {
		t.Fatalf("createEntryTestRoom: %v", err)
	}
	var roomID uuid.UUID
	if err := pool.QueryRow(ctx, `SELECT id FROM rooms WHERE slug = $1`, slug).Scan(&roomID); err != nil {
		t.Fatalf("createEntryTestRoom get ID: %v", err)
	}
	return roomID
}

func strptr(s string) *string { return &s }

// TestRoomEntryRepository_CreateMessageAndEvent verifies both kinds persist, share
// one per-room sequence space, and enforce their kind-specific required fields.
func TestRoomEntryRepository_CreateMessageAndEvent(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "entcr-" + time.Now().Format("150405")
	roomID := createEntryTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slug)
	})

	repo := db.NewRoomEntryRepository(pool)

	msg, created, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID:      roomID,
		Kind:        models.RoomEntryKindMessage,
		AuthorType:  strptr("agent"),
		AuthorID:    strptr("agent_planner"),
		ActorLabel:  "Planner",
		Body:        strptr("build the thing"),
		ContentType: "markdown",
	})
	if err != nil {
		t.Fatalf("Create message: %v", err)
	}
	if !created {
		t.Fatalf("expected created=true for a fresh message")
	}
	if msg.Kind != models.RoomEntryKindMessage || msg.Body == nil || *msg.Body != "build the thing" {
		t.Fatalf("message not persisted correctly: %+v", msg)
	}
	if msg.Sequence != 1 {
		t.Fatalf("first entry sequence = %d, want 1", msg.Sequence)
	}

	evt, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID:     roomID,
		Kind:       models.RoomEntryKindEvent,
		ActorLabel: "agent_planner",
		EventType:  strptr("CLAIM"),
		Issue:      "APP-1",
		Extension:  json.RawMessage(`{"note":"claimed"}`),
	})
	if err != nil {
		t.Fatalf("Create event: %v", err)
	}
	if evt.Kind != models.RoomEntryKindEvent || evt.EventType == nil || *evt.EventType != "CLAIM" {
		t.Fatalf("event not persisted correctly: %+v", evt)
	}
	if evt.Sequence != 2 {
		t.Fatalf("second entry sequence = %d, want 2 (shared timeline space)", evt.Sequence)
	}

	// message without a body must be rejected
	if _, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindMessage, ActorLabel: "x",
	}); err == nil {
		t.Fatalf("expected error creating a message with no body")
	}
	// event without an event_type must be rejected
	if _, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindEvent, ActorLabel: "x",
	}); err == nil {
		t.Fatalf("expected error creating an event with no event_type")
	}
}

// TestRoomEntryRepository_TranscriptAndCountsExcludeEvents verifies the transcript
// read and message count return messages only, while event queries return events.
func TestRoomEntryRepository_TranscriptAndCountsExcludeEvents(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "enttx-" + time.Now().Format("150405")
	roomID := createEntryTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slug)
	})

	repo := db.NewRoomEntryRepository(pool)
	for i := 0; i < 3; i++ {
		if _, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
			RoomID: roomID, Kind: models.RoomEntryKindMessage,
			AuthorType: strptr("agent"), AuthorID: strptr("a"), ActorLabel: "A",
			Body: strptr("m"), ContentType: "text",
		}); err != nil {
			t.Fatalf("create msg %d: %v", i, err)
		}
	}
	if _, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindEvent, ActorLabel: "A",
		EventType: strptr("BUILDING"), Extension: json.RawMessage(`{}`),
	}); err != nil {
		t.Fatalf("create event: %v", err)
	}

	transcript, err := repo.ListRecent(ctx, roomID, 50)
	if err != nil {
		t.Fatalf("ListRecent: %v", err)
	}
	if len(transcript) != 3 {
		t.Fatalf("transcript len = %d, want 3 (events excluded)", len(transcript))
	}
	for _, e := range transcript {
		if e.Kind != models.RoomEntryKindMessage {
			t.Fatalf("transcript contains a non-message entry: %+v", e)
		}
	}

	count, err := repo.CountMessages(ctx, roomID)
	if err != nil {
		t.Fatalf("CountMessages: %v", err)
	}
	if count != 3 {
		t.Fatalf("CountMessages = %d, want 3 (events excluded)", count)
	}

	events, err := repo.QueryEvents(ctx, models.QueryRoomEntryEventsParams{RoomID: roomID, EventType: "BUILDING"})
	if err != nil {
		t.Fatalf("QueryEvents: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("QueryEvents(BUILDING) len = %d, want 1", len(events))
	}
}

// TestRoomEntryRepository_SameRoomReplyEnforced verifies a reply that references an
// entry in a different room is rejected, while a same-room reply is accepted.
func TestRoomEntryRepository_SameRoomReplyEnforced(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slugA := "entra-" + time.Now().Format("150405")
	slugB := "entrb-" + time.Now().Format("150405")
	roomA := createEntryTestRoom(t, ctx, pool, slugA)
	roomB := createEntryTestRoom(t, ctx, pool, slugB)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slugA)
		entryTestCleanup(cctx, pool, slugB)
	})

	repo := db.NewRoomEntryRepository(pool)
	first, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomA, Kind: models.RoomEntryKindMessage,
		AuthorType: strptr("agent"), AuthorID: strptr("a"), ActorLabel: "A", Body: strptr("hi"), ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create first: %v", err)
	}

	// same-room reply is accepted
	if _, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomA, Kind: models.RoomEntryKindMessage,
		AuthorType: strptr("agent"), AuthorID: strptr("b"), ActorLabel: "B", Body: strptr("re"), ContentType: "text",
		ReplyToEntryID: &first.ID,
	}); err != nil {
		t.Fatalf("same-room reply should be accepted: %v", err)
	}

	// cross-room reply is rejected
	if _, _, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomB, Kind: models.RoomEntryKindMessage,
		AuthorType: strptr("agent"), AuthorID: strptr("c"), ActorLabel: "C", Body: strptr("bad"), ContentType: "text",
		ReplyToEntryID: &first.ID,
	}); err == nil {
		t.Fatalf("cross-room reply should be rejected")
	}
}

// TestRoomEntryRepository_ClientEntryIdempotent verifies a retry with the same
// client_entry_id returns the existing entry rather than duplicating it.
func TestRoomEntryRepository_ClientEntryIdempotent(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "entidem-" + time.Now().Format("150405")
	roomID := createEntryTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slug)
	})

	repo := db.NewRoomEntryRepository(pool)
	cid := "cid-abc"
	first, created1, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindMessage,
		AuthorType: strptr("agent"), AuthorID: strptr("a"), ActorLabel: "A", Body: strptr("once"), ContentType: "text",
		ClientEntryID: &cid,
	})
	if err != nil || !created1 {
		t.Fatalf("first create: entry=%v created=%v err=%v", first, created1, err)
	}
	second, created2, err := repo.Create(ctx, models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindMessage,
		AuthorType: strptr("agent"), AuthorID: strptr("a"), ActorLabel: "A", Body: strptr("once"), ContentType: "text",
		ClientEntryID: &cid,
	})
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if created2 {
		t.Fatalf("retry with same client_entry_id must not create a new entry")
	}
	if second.ID != first.ID {
		t.Fatalf("retry returned id %d, want existing %d", second.ID, first.ID)
	}
}

// TestRoomEntryRepository_BackfillFromLegacy verifies legacy messages and events
// merge into one ordered timeline, map old IDs to new entry IDs, remap replies,
// and re-run without duplicating.
func TestRoomEntryRepository_BackfillFromLegacy(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "entbf-" + time.Now().Format("150405")
	roomID := createEntryTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slug)
	})

	base := time.Now().Add(-time.Hour).Truncate(time.Second)
	// Pre-cutover rows live in the frozen legacy_* archive (migration 000094); the
	// messages/room_events names are now views that write the timeline directly.
	// legacy message 1 (oldest)
	var msg1ID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO legacy_messages (room_id, author_type, author_id, agent_name, content, content_type, created_at)
		VALUES ($1,'agent','a','A','first',$2,$3) RETURNING id`,
		roomID, "text", base).Scan(&msg1ID); err != nil {
		t.Fatalf("insert msg1: %v", err)
	}
	// legacy event between the two messages
	if _, err := pool.Exec(ctx, `
		INSERT INTO legacy_room_events (room_id, event_type, issue, actor, payload, created_at)
		VALUES ($1,'CLAIM','APP-1','A','{}',$2)`,
		roomID, base.Add(time.Minute)); err != nil {
		t.Fatalf("insert event: %v", err)
	}
	// legacy message 2 replying to message 1
	var msg2ID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO legacy_messages (room_id, author_type, author_id, agent_name, content, content_type, reply_to_entry_id, created_at)
		VALUES ($1,'agent','b','B','second',$2,$3,$4) RETURNING id`,
		roomID, "text", msg1ID, base.Add(2*time.Minute)).Scan(&msg2ID); err != nil {
		t.Fatalf("insert msg2: %v", err)
	}

	repo := db.NewRoomEntryRepository(pool)
	n, err := repo.BackfillFromLegacy(ctx)
	if err != nil {
		t.Fatalf("BackfillFromLegacy: %v", err)
	}
	if n < 3 {
		t.Fatalf("backfill inserted %d rows, want >= 3", n)
	}

	// order preserved: msg1, event, msg2 by created_at
	transcriptAll, err := repo.ListAll(ctx, roomID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(transcriptAll) != 3 {
		t.Fatalf("timeline len = %d, want 3", len(transcriptAll))
	}
	if transcriptAll[0].Kind != models.RoomEntryKindMessage ||
		transcriptAll[1].Kind != models.RoomEntryKindEvent ||
		transcriptAll[2].Kind != models.RoomEntryKindMessage {
		t.Fatalf("timeline order wrong: %v %v %v", transcriptAll[0].Kind, transcriptAll[1].Kind, transcriptAll[2].Kind)
	}
	if transcriptAll[0].Sequence != 1 || transcriptAll[1].Sequence != 2 || transcriptAll[2].Sequence != 3 {
		t.Fatalf("sequences not monotonic: %d %d %d", transcriptAll[0].Sequence, transcriptAll[1].Sequence, transcriptAll[2].Sequence)
	}

	// legacy id maps to the new entry id
	entry1, err := repo.ResolveLegacy(ctx, "message", msg1ID)
	if err != nil {
		t.Fatalf("ResolveLegacy msg1: %v", err)
	}
	if entry1 != transcriptAll[0].ID {
		t.Fatalf("ResolveLegacy(message,%d) = %d, want %d", msg1ID, entry1, transcriptAll[0].ID)
	}

	// reply_to remapped from legacy message id to new entry id
	if transcriptAll[2].ReplyToEntryID == nil || *transcriptAll[2].ReplyToEntryID != entry1 {
		t.Fatalf("reply_to not remapped: got %v want %d", transcriptAll[2].ReplyToEntryID, entry1)
	}

	// idempotent: re-running inserts nothing new
	n2, err := repo.BackfillFromLegacy(ctx)
	if err != nil {
		t.Fatalf("BackfillFromLegacy re-run: %v", err)
	}
	if n2 != 0 {
		t.Fatalf("re-run inserted %d rows, want 0 (idempotent)", n2)
	}
	after, _ := repo.ListAll(ctx, roomID)
	if len(after) != 3 {
		t.Fatalf("timeline len after re-run = %d, want 3", len(after))
	}
}
