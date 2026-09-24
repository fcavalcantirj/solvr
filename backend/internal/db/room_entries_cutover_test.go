package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// Cutover tests for "Store room messages and coordination events in one ordered room
// timeline": after migration 000094 the messages and room_events names are views over
// room_entries, so every existing reader and writer goes through one storage table.

func cutoverPool(t *testing.T) (context.Context, *db.Pool) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	t.Cleanup(pool.Close)
	return ctx, pool
}

func cutoverRoom(t *testing.T, ctx context.Context, pool *db.Pool, prefix string) uuid.UUID {
	t.Helper()
	slug := prefix + "-" + uuid.NewString()[:8]
	roomID := createEntryTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slug)
	})
	return roomID
}

func postMessage(t *testing.T, ctx context.Context, repo *db.MessageRepository, roomID uuid.UUID, body string) *models.Message {
	t.Helper()
	author := "agent-" + body
	msg, err := repo.Create(ctx, models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: "Agent",
		Content: body, ContentType: "text",
	})
	if err != nil {
		t.Fatalf("MessageRepository.Create(%q): %v", body, err)
	}
	return msg
}

func postEvent(t *testing.T, ctx context.Context, repo *db.RoomEventRepository, roomID uuid.UUID, eventType, issue string) *models.RoomEvent {
	t.Helper()
	ev, err := repo.Create(ctx, models.CreateRoomEventParams{
		RoomID: roomID, EventType: eventType, Issue: issue, Actor: "Agent", Payload: json.RawMessage(`{"k":1}`),
	})
	if err != nil {
		t.Fatalf("RoomEventRepository.Create(%s): %v", eventType, err)
	}
	return ev
}

func TestRoomTimelineCutover_LegacyNamesAreViews(t *testing.T) {
	ctx, pool := cutoverPool(t)
	for _, name := range []string{"messages", "room_events"} {
		var kind string
		if err := pool.QueryRow(ctx,
			`SELECT relkind::text FROM pg_class WHERE relname = $1 AND relnamespace = 'public'::regnamespace`,
			name).Scan(&kind); err != nil {
			t.Fatalf("lookup %s: %v", name, err)
		}
		if kind != "v" {
			t.Errorf("%s relkind = %q, want view 'v' (parallel storage must be replaced)", name, kind)
		}
	}
}

func TestRoomTimelineCutover_EachActionStoredOnce(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomID := cutoverRoom(t, ctx, pool, "cutone")
	msgs := db.NewMessageRepository(pool)
	events := db.NewRoomEventRepository(pool)

	m1 := postMessage(t, ctx, msgs, roomID, "one")
	ev := postEvent(t, ctx, events, roomID, "CLAIM", "APP-9")
	m2 := postMessage(t, ctx, msgs, roomID, "two")

	all, err := db.NewRoomEntryRepository(pool).ListAll(ctx, roomID)
	if err != nil {
		t.Fatalf("ListAll: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("room_entries rows = %d, want 3 (one per action, no duplicates)", len(all))
	}
	wantIDs := []int64{m1.ID, ev.ID, m2.ID}
	wantKinds := []string{models.RoomEntryKindMessage, models.RoomEntryKindEvent, models.RoomEntryKindMessage}
	for i, e := range all {
		if e.ID != wantIDs[i] || e.Kind != wantKinds[i] || e.Sequence != i+1 {
			t.Errorf("entry[%d] = id %d kind %s seq %d; want id %d kind %s seq %d",
				i, e.ID, e.Kind, e.Sequence, wantIDs[i], wantKinds[i], i+1)
		}
	}
	if m2.SequenceNum == nil || *m2.SequenceNum != 3 {
		t.Errorf("second message sequence_num = %v, want 3 (shared timeline position)", m2.SequenceNum)
	}

	var total int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1`, roomID).Scan(&total); err != nil {
		t.Fatalf("count: %v", err)
	}
	if total != 3 {
		t.Errorf("room_entries count = %d, want 3", total)
	}
}

func TestRoomTimelineCutover_CountsExcludeEventsAndFiltersRetained(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomID := cutoverRoom(t, ctx, pool, "cutcnt")
	msgs := db.NewMessageRepository(pool)
	events := db.NewRoomEventRepository(pool)
	entries := db.NewRoomEntryRepository(pool)

	postMessage(t, ctx, msgs, roomID, "a")
	postEvent(t, ctx, events, roomID, "CLAIM", "APP-1")
	postEvent(t, ctx, events, roomID, "BUILDING", "APP-1")
	postEvent(t, ctx, events, roomID, "CLAIM", "APP-2")
	postMessage(t, ctx, msgs, roomID, "b")

	n, err := entries.CountMessages(ctx, roomID)
	if err != nil || n != 2 {
		t.Fatalf("CountMessages = %d, %v; want 2", n, err)
	}
	var viewCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE room_id = $1 AND deleted_at IS NULL`, roomID).Scan(&viewCount); err != nil || viewCount != 2 {
		t.Fatalf("messages view count = %d, %v; want 2", viewCount, err)
	}
	recent, err := msgs.ListRecent(ctx, roomID, 50)
	if err != nil || len(recent) != 2 {
		t.Fatalf("transcript = %d, %v; want 2 messages only", len(recent), err)
	}

	byType, err := events.Query(ctx, models.QueryRoomEventsParams{RoomID: roomID, EventType: "CLAIM"})
	if err != nil || len(byType) != 2 {
		t.Fatalf("type filter = %d, %v; want 2", len(byType), err)
	}
	byIssue, err := events.Query(ctx, models.QueryRoomEventsParams{RoomID: roomID, Issue: "APP-1"})
	if err != nil || len(byIssue) != 2 {
		t.Fatalf("issue filter = %d, %v; want 2", len(byIssue), err)
	}
	both, err := events.Query(ctx, models.QueryRoomEventsParams{RoomID: roomID, EventType: "CLAIM", Issue: "APP-2"})
	if err != nil || len(both) != 1 || both[0].Issue != "APP-2" || string(both[0].Payload) == "" {
		t.Fatalf("type+issue filter = %+v, %v; want the single APP-2 CLAIM", both, err)
	}
}

func TestRoomTimelineCutover_LegacyLinksAndCursorsResolve(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomID := cutoverRoom(t, ctx, pool, "cutlnk")
	msgs := db.NewMessageRepository(pool)
	entries := db.NewRoomEntryRepository(pool)

	// A message that only exists in the frozen pre-cutover archive, as production rows
	// do until the cutover migration backfills them.
	var legacyID int64
	if err := pool.QueryRow(ctx, `
		INSERT INTO legacy_messages (room_id, author_type, author_id, agent_name, content, created_at)
		VALUES ($1, 'agent', 'old', 'Old', 'from before', NOW() - INTERVAL '1 hour') RETURNING id`,
		roomID).Scan(&legacyID); err != nil {
		t.Fatalf("insert legacy message: %v", err)
	}
	if _, err := entries.BackfillFromLegacy(ctx); err != nil {
		t.Fatalf("BackfillFromLegacy: %v", err)
	}

	// Old deep link: the legacy message id still addresses the same message.
	old, err := msgs.GetByID(ctx, roomID, legacyID)
	if err != nil || old.Content != "from before" {
		t.Fatalf("GetByID(legacy %d) = %+v, %v; want the archived message", legacyID, old, err)
	}
	resolved, err := entries.ResolveLegacy(ctx, "message", legacyID)
	if err != nil || resolved != legacyID {
		t.Fatalf("ResolveLegacy(message,%d) = %d, %v; want identity", legacyID, resolved, err)
	}

	// Old SSE cursor: replay after the legacy id returns only newer messages.
	fresh := postMessage(t, ctx, msgs, roomID, "after")
	if fresh.ID <= legacyID {
		t.Fatalf("new entry id %d must sort after legacy id %d", fresh.ID, legacyID)
	}
	replay, err := msgs.ListAfter(ctx, roomID, legacyID, 100)
	if err != nil || len(replay) != 1 || replay[0].ID != fresh.ID {
		t.Fatalf("ListAfter(legacy cursor) = %+v, %v; want only the new message", replay, err)
	}
}

func TestRoomTimelineCutover_ConcurrentWritesKeepStableOrder(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomID := cutoverRoom(t, ctx, pool, "cutcon")
	msgs := db.NewMessageRepository(pool)
	events := db.NewRoomEventRepository(pool)

	const writers = 24
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			var err error
			if i%3 == 0 {
				_, err = events.Create(ctx, models.CreateRoomEventParams{RoomID: roomID, EventType: "PING", Actor: "Agent"})
			} else {
				author := "w"
				_, err = msgs.Create(ctx, models.CreateMessageParams{
					RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: "W", Content: "x", ContentType: "text",
				})
			}
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatalf("concurrent write: %v", err)
		}
	}

	all, err := db.NewRoomEntryRepository(pool).ListAll(ctx, roomID)
	if err != nil || len(all) != writers {
		t.Fatalf("ListAll = %d, %v; want %d", len(all), err, writers)
	}
	for i, e := range all {
		if e.Sequence != i+1 {
			t.Fatalf("entry[%d] sequence = %d, want %d (gap-free per-room order)", i, e.Sequence, i+1)
		}
		if i > 0 && e.ID <= all[i-1].ID {
			t.Fatalf("id order diverges from sequence at %d: %d after %d", i, e.ID, all[i-1].ID)
		}
	}
}

func TestRoomTimelineCutover_CrossRoomReplyRejected(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomA := cutoverRoom(t, ctx, pool, "cutxa")
	roomB := cutoverRoom(t, ctx, pool, "cutxb")
	msgs := db.NewMessageRepository(pool)

	target := postMessage(t, ctx, msgs, roomA, "target")
	author := "x"
	if _, err := msgs.Create(ctx, models.CreateMessageParams{
		RoomID: roomB, AuthorType: "agent", AuthorID: &author, AgentName: "X",
		Content: "reply", ContentType: "text", ReplyToEntryID: &target.ID,
	}); err == nil {
		t.Fatal("reply to another room's entry was accepted; want rejection")
	}
	if _, err := msgs.Create(ctx, models.CreateMessageParams{
		RoomID: roomB, AuthorType: "agent", AuthorID: &author, AgentName: "X",
		Content: "supersede", ContentType: "text", SupersedesEntryID: &target.ID,
	}); err == nil {
		t.Fatal("supersede of another room's entry was accepted; want rejection")
	}
}

func TestRoomTimelineCutover_AddressingReferencesSameRoomParticipants(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomA := cutoverRoom(t, ctx, pool, "cutada")
	roomB := cutoverRoom(t, ctx, pool, "cutadb")
	msgs := db.NewMessageRepository(pool)

	// Participants of room A: an author, a present agent, and an admitted member.
	postMessage(t, ctx, msgs, roomA, "hi") // author_id "agent-hi", label "Agent"
	postMessage(t, ctx, msgs, roomB, "elsewhere")
	if _, err := pool.Exec(ctx, `INSERT INTO agent_presence (room_id, agent_name, card_json) VALUES ($1, 'present-bot', '{}')`, roomA); err != nil {
		t.Fatalf("presence: %v", err)
	}
	memberID := "cutad_member_" + uuid.NewString()[:8]
	if _, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, api_key_hash, status) VALUES ($1, $1, 'x', 'active')`, memberID); err != nil {
		t.Fatalf("agent: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), `DELETE FROM agents WHERE id = $1`, memberID) }) //nolint:errcheck
	if _, err := db.NewRoomMemberRepository(pool).Add(ctx, models.AddRoomMemberParams{
		RoomID: roomA, AgentID: memberID, Role: "member", AddedBy: "system",
	}); err != nil {
		t.Fatalf("add member: %v", err)
	}

	address := func(ids string) error {
		author := "addresser"
		_, err := msgs.Create(ctx, models.CreateMessageParams{
			RoomID: roomA, AuthorType: "agent", AuthorID: &author, AgentName: "Addresser",
			Content: "for you", ContentType: "text", AddressedMemberIDs: json.RawMessage(ids),
		})
		return err
	}

	for _, ok := range []string{`["agent-hi"]`, `["Agent"]`, `["present-bot"]`, `["` + memberID + `"]`, `[]`} {
		if err := address(ok); err != nil {
			t.Errorf("addressing %s in its own room rejected: %v", ok, err)
		}
	}
	for _, bad := range []string{`["agent-elsewhere"]`, `["nobody-at-all"]`, `["agent-hi","nobody"]`, `{"id":"agent-hi"}`, `[42]`} {
		err := address(bad)
		if err == nil {
			t.Errorf("addressing %s was accepted; want rejection", bad)
			continue
		}
		if !errors.Is(err, db.ErrInvalidEntryReference) {
			t.Errorf("addressing %s: err = %v, want ErrInvalidEntryReference", bad, err)
		}
	}
}

func TestRoomTimelineCutover_CrossRoomReferenceIsTypedError(t *testing.T) {
	ctx, pool := cutoverPool(t)
	roomA := cutoverRoom(t, ctx, pool, "cutrea")
	roomB := cutoverRoom(t, ctx, pool, "cutreb")
	msgs := db.NewMessageRepository(pool)
	target := postMessage(t, ctx, msgs, roomA, "target")

	author := "x"
	_, err := msgs.Create(ctx, models.CreateMessageParams{
		RoomID: roomB, AuthorType: "agent", AuthorID: &author, AgentName: "X",
		Content: "reply", ContentType: "text", ReplyToEntryID: &target.ID,
	})
	if !errors.Is(err, db.ErrInvalidEntryReference) {
		t.Fatalf("cross-room reply err = %v, want ErrInvalidEntryReference", err)
	}
}
