package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

func newFunnelTestPool(t *testing.T) (*db.Pool, context.Context) {
	t.Helper()
	url := getTestDatabaseURL(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	t.Cleanup(cancel)
	pool, err := db.NewPool(ctx, url)
	if err != nil {
		t.Fatalf("NewPool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool, ctx
}

func TestFunnelEventRepository_RecordBrowserEvent(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	flow := "f_browser_" + time.Now().Format("150405.000000")

	err := repo.RecordBrowserEvent(ctx, db.BrowserFunnelEvent{
		FlowID:             flow,
		EventName:          models.FunnelConnectionStarted,
		ActorType:          models.FunnelActorAnonymous,
		Preset:             "plan-and-build",
		EntrySurface:       "homepage_panel",
		InstructionVersion: "1.0",
	})
	if err != nil {
		t.Fatalf("RecordBrowserEvent: %v", err)
	}

	events, err := repo.ListByFlow(ctx, flow)
	if err != nil {
		t.Fatalf("ListByFlow: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("len(events) = %d; want 1", len(events))
	}
	e := events[0]
	if e.EventName != models.FunnelConnectionStarted {
		t.Errorf("EventName = %q; want %q", e.EventName, models.FunnelConnectionStarted)
	}
	if e.SourceChannel != models.FunnelSourceBrowser {
		t.Errorf("SourceChannel = %q; want browser", e.SourceChannel)
	}
	if e.EntrySurface != "homepage_panel" {
		t.Errorf("EntrySurface = %q; want homepage_panel", e.EntrySurface)
	}
	if e.Preset != "plan-and-build" {
		t.Errorf("Preset = %q; want plan-and-build", e.Preset)
	}
}

// RejectsServerEventShape is a guard at the model layer: a browser cannot report
// a server-only step. The repo trusts the caller, so the classification lives in
// models and is asserted here so the ingest handler can rely on it.
func TestFunnelEventClassification(t *testing.T) {
	if !models.IsBrowserFunnelEvent(models.FunnelConnectionStarted) {
		t.Error("connection_started should be a browser event")
	}
	if models.IsBrowserFunnelEvent(models.FunnelRoomCreated) {
		t.Error("room_created must NOT be a browser event (a client cannot fake it)")
	}
	if !models.IsServerFunnelEvent(models.FunnelFirstTwoWayExchange) {
		t.Error("first_two_way_exchange should be a server event")
	}
	if models.ValidFunnelEventName("not_a_real_event") {
		t.Error("unknown event name must be rejected")
	}
}

func TestFunnelEventRepository_RoomCreatedDedup(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	room := createMemberTestRoom(ctx, t, pool, "fn-created", false)
	flow := "f_created_" + time.Now().Format("150405.000000")

	for i := 0; i < 2; i++ {
		if err := repo.RecordRoomCreated(ctx, room.ID, models.FunnelActorAgent, "actorhashA", flow); err != nil {
			t.Fatalf("RecordRoomCreated #%d: %v", i, err)
		}
	}

	events, err := repo.ListByRoom(ctx, room.ID)
	if err != nil {
		t.Fatalf("ListByRoom: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("room_created recorded %d times; want 1 (deduped per room)", len(events))
	}
	if events[0].FlowID != flow {
		t.Errorf("FlowID = %q; want %q (flow persisted with the room)", events[0].FlowID, flow)
	}
	if events[0].SourceChannel != models.FunnelSourceServer {
		t.Errorf("SourceChannel = %q; want server", events[0].SourceChannel)
	}
}

func TestFunnelEventRepository_ParticipantJoinedOrdinalAndDedup(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	room := createMemberTestRoom(ctx, t, pool, "fn-joined", false)
	flow := "f_joined_" + time.Now().Format("150405.000000")

	// room_created carries the flow; joins inherit it.
	if err := repo.RecordRoomCreated(ctx, room.ID, models.FunnelActorAgent, "planner", flow); err != nil {
		t.Fatalf("RecordRoomCreated: %v", err)
	}

	ord1, rec1, err := repo.RecordParticipantJoined(ctx, room.ID, models.FunnelActorAgent, "planner")
	if err != nil || !rec1 || ord1 != 1 {
		t.Fatalf("first join: ord=%d rec=%v err=%v; want ord=1 rec=true", ord1, rec1, err)
	}
	ord2, rec2, err := repo.RecordParticipantJoined(ctx, room.ID, models.FunnelActorAgent, "executor")
	if err != nil || !rec2 || ord2 != 2 {
		t.Fatalf("second join: ord=%d rec=%v err=%v; want ord=2 rec=true", ord2, rec2, err)
	}
	// A reconnecting planner must NOT inflate the count or change its ordinal.
	_, rec3, err := repo.RecordParticipantJoined(ctx, room.ID, models.FunnelActorAgent, "planner")
	if err != nil {
		t.Fatalf("rejoin: %v", err)
	}
	if rec3 {
		t.Error("rejoin recorded a new participant_joined; want deduped (recorded=false)")
	}

	joins := 0
	events, _ := repo.ListByRoom(ctx, room.ID)
	for _, e := range events {
		if e.EventName == models.FunnelParticipantJoined {
			joins++
			if e.FlowID != flow {
				t.Errorf("join FlowID = %q; want %q (inherited from room_created)", e.FlowID, flow)
			}
		}
	}
	if joins != 2 {
		t.Errorf("participant_joined rows = %d; want 2 (rejoin deduped)", joins)
	}
}

func TestFunnelEventRepository_FirstTwoWayExchange(t *testing.T) {
	pool, ctx := newFunnelTestPool(t)
	repo := db.NewFunnelEventRepository(pool)
	msgRepo := db.NewMessageRepository(pool)
	room := createMemberTestRoom(ctx, t, pool, "fn-twoway", false)

	postAgent := func(name string) {
		if _, err := msgRepo.Create(ctx, models.CreateMessageParams{
			RoomID: room.ID, AuthorType: "agent", AgentName: name,
			Content: "hello from " + name, ContentType: "text",
		}); err != nil {
			t.Fatalf("post message: %v", err)
		}
	}

	// One agent has posted: not a two-way exchange yet.
	postAgent("agent-one")
	rec, err := repo.RecordFirstTwoWayExchange(ctx, room.ID)
	if err != nil {
		t.Fatalf("RecordFirstTwoWayExchange (single author): %v", err)
	}
	if rec {
		t.Fatal("recorded first_two_way_exchange with only one distinct agent")
	}

	// A second distinct agent posts: now it is a two-way exchange.
	postAgent("agent-two")
	rec, err = repo.RecordFirstTwoWayExchange(ctx, room.ID)
	if err != nil {
		t.Fatalf("RecordFirstTwoWayExchange (two authors): %v", err)
	}
	if !rec {
		t.Fatal("did not record first_two_way_exchange after a second distinct agent posted")
	}

	// A later message must not record the milestone twice.
	postAgent("agent-two")
	rec, err = repo.RecordFirstTwoWayExchange(ctx, room.ID)
	if err != nil {
		t.Fatalf("RecordFirstTwoWayExchange (repeat): %v", err)
	}
	if rec {
		t.Fatal("recorded first_two_way_exchange a second time; want deduped once per room")
	}
}

func TestPseudonymizeActor(t *testing.T) {
	if got := db.PseudonymizeActor(""); got != "" {
		t.Errorf("empty id ref = %q; want empty", got)
	}
	raw := "agent-uuid-1234"
	ref := db.PseudonymizeActor(raw)
	if ref == "" || ref == raw {
		t.Errorf("ref = %q; must be non-empty and never the raw id", ref)
	}
	if db.PseudonymizeActor(raw) != ref {
		t.Error("pseudonymization is not stable for the same id")
	}
	if db.PseudonymizeActor("different") == ref {
		t.Error("distinct ids collided to the same ref")
	}
}
