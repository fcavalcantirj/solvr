package db_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestClientEntry_ReuseWithDifferentPayloadConflicts pins the retry contract for timeline
// writes: a client_entry_id replays only the SAME write. Reusing it for a different
// payload (body, extension, references, event type, issue, or kind) is refused with
// ErrClientEntryConflict and stores nothing, on both the message and the entry paths.
// Semantically equal JSON and defaulted fields are the same payload.
func TestClientEntry_ReuseWithDifferentPayloadConflicts(t *testing.T) {
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool() error = %v", err)
	}
	defer pool.Close()

	slug := "entreuse-" + time.Now().Format("150405")
	roomID := createEntryTestRoom(t, ctx, pool, slug)
	t.Cleanup(func() {
		cctx, ccancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer ccancel()
		entryTestCleanup(cctx, pool, slug)
	})

	count := func() int {
		var n int
		if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM room_entries WHERE room_id = $1`, roomID).Scan(&n); err != nil {
			t.Fatalf("count: %v", err)
		}
		return n
	}

	msgRepo := db.NewMessageRepository(pool)
	author := "agent-reuse"
	key := "m-1"
	msg := models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: "planner",
		Content: "plan: step one", ContentType: "text",
		Metadata:      json.RawMessage(`{"a":1,"b":[1,2]}`),
		ClientEntryID: &key,
	}
	first, created, err := msgRepo.CreateWithClientEntry(ctx, msg)
	if err != nil || !created {
		t.Fatalf("first message: created=%v err=%v", created, err)
	}

	// Same write with reordered JSON keys and a different display label: a replay.
	same := msg
	same.Metadata = json.RawMessage(`{ "b": [1, 2], "a": 1 }`)
	same.AgentName = "planner-renamed"
	replay, created, err := msgRepo.CreateWithClientEntry(ctx, same)
	if err != nil || created || replay.ID != first.ID {
		t.Fatalf("equivalent retry must replay entry %d: got created=%v err=%v entry=%v", first.ID, created, err, replay)
	}

	msgCases := map[string]func(p *models.CreateMessageParams){
		"different body":         func(p *models.CreateMessageParams) { p.Content = "plan: step two" },
		"different content_type": func(p *models.CreateMessageParams) { p.ContentType = "markdown" },
		"different metadata":     func(p *models.CreateMessageParams) { p.Metadata = json.RawMessage(`{"a":2,"b":[1,2]}`) },
		"metadata dropped":       func(p *models.CreateMessageParams) { p.Metadata = nil },
		"reply reference added":  func(p *models.CreateMessageParams) { id := first.ID; p.ReplyToEntryID = &id },
	}
	for name, mutate := range msgCases {
		p := msg
		mutate(&p)
		got, created, err := msgRepo.CreateWithClientEntry(ctx, p)
		if !errors.Is(err, db.ErrClientEntryConflict) {
			t.Errorf("%s: want ErrClientEntryConflict, got entry=%v created=%v err=%v", name, got, created, err)
		}
	}
	if n := count(); n != 1 {
		t.Fatalf("conflicting reuse must store nothing: %d entries, want 1", n)
	}

	// A message without metadata replays against a retry that sends the {} default.
	bare := "m-bare"
	plain := models.CreateMessageParams{
		RoomID: roomID, AuthorType: "agent", AuthorID: &author, AgentName: "planner",
		Content: "no metadata", ContentType: "text", ClientEntryID: &bare,
	}
	if _, created, err := msgRepo.CreateWithClientEntry(ctx, plain); err != nil || !created {
		t.Fatalf("bare message: created=%v err=%v", created, err)
	}
	plain.Metadata = json.RawMessage(`{}`)
	if _, created, err := msgRepo.CreateWithClientEntry(ctx, plain); err != nil || created {
		t.Fatalf("empty-object metadata retry must replay: created=%v err=%v", created, err)
	}

	entryRepo := db.NewRoomEntryRepository(pool)
	evKey := "e-1"
	claim := "CLAIM"
	event := models.CreateRoomEntryParams{
		RoomID: roomID, Kind: models.RoomEntryKindEvent,
		AuthorType: strptr("agent"), AuthorID: &author, ActorLabel: "w1",
		EventType: &claim, Issue: "APP-1", Extension: json.RawMessage(`{"pr":7}`),
		ClientEntryID: &evKey,
	}
	firstEvent, created, err := entryRepo.Create(ctx, event)
	if err != nil || !created {
		t.Fatalf("first event: created=%v err=%v", created, err)
	}
	if again, created, err := entryRepo.Create(ctx, event); err != nil || created || again.ID != firstEvent.ID {
		t.Fatalf("identical event retry must replay: created=%v err=%v", created, err)
	}

	building := "BUILDING"
	eventCases := map[string]func(p *models.CreateRoomEntryParams){
		"different event_type": func(p *models.CreateRoomEntryParams) { p.EventType = &building },
		"different issue":      func(p *models.CreateRoomEntryParams) { p.Issue = "APP-2" },
		"different payload":    func(p *models.CreateRoomEntryParams) { p.Extension = json.RawMessage(`{"pr":8}`) },
		"payload dropped":      func(p *models.CreateRoomEntryParams) { p.Extension = nil },
		"message kind": func(p *models.CreateRoomEntryParams) {
			p.Kind, p.EventType, p.Issue, p.Body, p.Extension = models.RoomEntryKindMessage, nil, "", strptr("hello"), nil
		},
	}
	for name, mutate := range eventCases {
		p := event
		mutate(&p)
		got, created, err := entryRepo.Create(ctx, p)
		if !errors.Is(err, db.ErrClientEntryConflict) {
			t.Errorf("event %s: want ErrClientEntryConflict, got entry=%v created=%v err=%v", name, got, created, err)
		}
	}

	// The event's key reused for a message through the messages path is also a conflict,
	// not an internal error from the unique index.
	crossKind := msg
	crossKind.ClientEntryID = &evKey
	if got, created, err := msgRepo.CreateWithClientEntry(ctx, crossKind); !errors.Is(err, db.ErrClientEntryConflict) {
		t.Errorf("event key reused as a message: want ErrClientEntryConflict, got entry=%v created=%v err=%v", got, created, err)
	}
	if n := count(); n != 3 {
		t.Fatalf("after conflicts: %d entries, want 3 (two messages, one event)", n)
	}
}
