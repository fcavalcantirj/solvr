package hub_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/hub"
)

// timeline is a test EntrySource: an ordered, committed room timeline the relay reads
// back after its cursor. It records every read so tests can bound the work done.
type timeline struct {
	mu      sync.Mutex
	entries map[uuid.UUID][]hub.RoomEvent
	reads   int
	maxRead int
	failing bool
}

func newTimeline() *timeline { return &timeline{entries: map[uuid.UUID][]hub.RoomEvent{}} }

func (tl *timeline) commit(room uuid.UUID, n int) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	for i := 0; i < n; i++ {
		seq := len(tl.entries[room]) + 1
		tl.entries[room] = append(tl.entries[room], hub.RoomEvent{
			ID: int64(seq * 10), Sequence: seq, Type: hub.EventMessage, RoomID: hub.NewRoomID(room),
		})
	}
}

func (tl *timeline) MaxSequence(_ context.Context, room uuid.UUID) (int, error) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if tl.failing {
		return 0, errors.New("db down")
	}
	return len(tl.entries[room]), nil
}

func (tl *timeline) FramesAfter(_ context.Context, room uuid.UUID, after, limit int) ([]hub.RoomEvent, error) {
	tl.mu.Lock()
	defer tl.mu.Unlock()
	tl.reads++
	if limit > tl.maxRead {
		tl.maxRead = limit
	}
	if tl.failing {
		return nil, errors.New("db down")
	}
	all := tl.entries[room]
	if after >= len(all) {
		return nil, nil
	}
	end := min(after+limit, len(all))
	return append([]hub.RoomEvent(nil), all[after:end]...), nil
}

// relayRoom starts a relay-backed manager and subscribes one listener to a room.
func relayRoom(t *testing.T, tl *timeline) (*hub.HubManager, uuid.UUID, <-chan hub.RoomEvent) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	mgr := hub.NewHubManager(ctx, hub.NewPresenceRegistry(), makeLogger(), 0)
	mgr.EnableRelay(tl)
	room := uuid.New()
	ch, err := mgr.GetOrCreate(ctx, hub.NewRoomID(room)).SubscribeStream("_browser_listener")
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}
	t.Cleanup(func() {
		cancel()
		mgr.WaitIdle()
	})
	return mgr, room, ch
}

// receiveSeqs reads timeline frames until n arrived or the deadline passes.
func receiveSeqs(t *testing.T, ch <-chan hub.RoomEvent, n int) []int {
	t.Helper()
	var seqs []int
	deadline := time.After(2 * time.Second)
	for len(seqs) < n {
		select {
		case evt := <-ch:
			if evt.Sequence > 0 {
				seqs = append(seqs, evt.Sequence)
			}
		case <-deadline:
			t.Fatalf("received %v, want %d frames", seqs, n)
		}
	}
	return seqs
}

// expectQuiet fails if another timeline frame arrives within a short window.
func expectQuiet(t *testing.T, ch <-chan hub.RoomEvent) {
	t.Helper()
	select {
	case evt := <-ch:
		if evt.Sequence > 0 {
			t.Fatalf("unexpected extra frame seq=%d", evt.Sequence)
		}
	case <-time.After(100 * time.Millisecond):
	}
}

func TestRelay_AnnounceDeliversCommittedEntriesInOrderOnce(t *testing.T) {
	tl := newTimeline()
	mgr, room, ch := relayRoom(t, tl)

	tl.commit(room, 3)
	mgr.Announce(hub.NewRoomID(room))
	mgr.Announce(hub.NewRoomID(room)) // a duplicate wakeup is not duplicate delivery
	if got := receiveSeqs(t, ch, 3); got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Fatalf("got %v, want [1 2 3]", got)
	}
	mgr.WaitIdle()
	expectQuiet(t, ch)
}

func TestRelay_HistoryBeforeTheHubExistedIsNotRebroadcast(t *testing.T) {
	tl := newTimeline()
	ctx, cancel := context.WithCancel(context.Background())
	defer func() { cancel() }()
	mgr := hub.NewHubManager(ctx, hub.NewPresenceRegistry(), makeLogger(), 0)
	mgr.EnableRelay(tl)
	room := uuid.New()
	tl.commit(room, 5) // committed before any local subscriber

	ch, err := mgr.GetOrCreate(ctx, hub.NewRoomID(room)).Subscribe("listener", nil)
	if err != nil {
		t.Fatal(err)
	}
	tl.commit(room, 1)
	mgr.Announce(hub.NewRoomID(room))
	if got := receiveSeqs(t, ch, 1); got[0] != 6 {
		t.Fatalf("got %v, want [6]", got)
	}
	mgr.WaitIdle()
	expectQuiet(t, ch)
	cancel()
	mgr.WaitIdle()
}

func TestRelay_AnnounceForARoomWithoutALocalHubIsANoop(t *testing.T) {
	tl := newTimeline()
	mgr, _, _ := relayRoom(t, tl)
	other := uuid.New()
	tl.commit(other, 2)
	mgr.Announce(hub.NewRoomID(other))
	mgr.WaitIdle()
	if mgr.Get(hub.NewRoomID(other)) != nil {
		t.Fatal("announce must not create a hub")
	}
}

func TestRelay_WakeAllCatchesUpMissedNotifications(t *testing.T) {
	tl := newTimeline()
	mgr, room, ch := relayRoom(t, tl)

	tl.commit(room, 4) // committed while notifications were lost: no Announce
	mgr.WakeAll()
	if got := receiveSeqs(t, ch, 4); got[3] != 4 {
		t.Fatalf("got %v, want [1 2 3 4]", got)
	}
	mgr.WaitIdle()
	mgr.WakeAll()
	mgr.WaitIdle()
	expectQuiet(t, ch)
}

func TestRelay_BacklogIsReadInBoundedPages(t *testing.T) {
	tl := newTimeline()
	mgr, room, ch := relayRoom(t, tl)

	tl.commit(room, 250)
	mgr.Announce(hub.NewRoomID(room))
	// Read everything the stream is given: all 250, or a gap-free prefix if this reader
	// fell behind and the hub closed its stream (it would then resume from its cursor).
	var got []int
	for evt := range ch {
		if evt.Sequence > 0 {
			got = append(got, evt.Sequence)
		}
		if len(got) == 250 {
			break
		}
	}
	if len(got) == 0 {
		t.Fatal("no frames")
	}
	for i, seq := range got {
		if seq != i+1 {
			t.Fatalf("frame %d has seq %d: out of order or a hole", i, seq)
		}
	}
	mgr.WaitIdle()
	tl.mu.Lock()
	defer tl.mu.Unlock()
	if tl.maxRead > hub.RelayPageSize {
		t.Fatalf("read page of %d exceeds bound %d", tl.maxRead, hub.RelayPageSize)
	}
}

func TestRelay_FailedReadIsRetriedOnTheNextWake(t *testing.T) {
	tl := newTimeline()
	mgr, room, ch := relayRoom(t, tl)

	tl.mu.Lock()
	tl.failing = true
	tl.mu.Unlock()
	tl.commit(room, 2)
	mgr.Announce(hub.NewRoomID(room))
	mgr.WaitIdle()
	expectQuiet(t, ch)

	tl.mu.Lock()
	tl.failing = false
	tl.mu.Unlock()
	mgr.WakeAll()
	if got := receiveSeqs(t, ch, 2); got[0] != 1 || got[1] != 2 {
		t.Fatalf("got %v, want [1 2]", got)
	}
}

func TestRelay_PublishWithoutRelayBroadcastsTheFrameDirectly(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	mgr := hub.NewHubManager(ctx, hub.NewPresenceRegistry(), makeLogger(), 0)
	room := hub.NewRoomID(uuid.New())
	ch, err := mgr.GetOrCreate(ctx, room).Subscribe("listener", nil)
	if err != nil {
		t.Fatal(err)
	}
	mgr.Publish(room, hub.RoomEvent{ID: 7, Sequence: 1, Type: hub.EventMessage, RoomID: room})
	if got := receiveSeqs(t, ch, 1); got[0] != 1 {
		t.Fatalf("got %v", got)
	}
	cancel()
	mgr.WaitIdle()
}

func TestRelay_PublishWithRelayReadsTheCommittedEntryBack(t *testing.T) {
	tl := newTimeline()
	mgr, room, ch := relayRoom(t, tl)
	tl.commit(room, 1)
	// The frame handed to Publish is ignored: the committed row is the delivery record.
	mgr.Publish(hub.NewRoomID(room), hub.RoomEvent{ID: 999, Sequence: 1, Type: hub.EventMessage})
	select {
	case evt := <-ch:
		if evt.ID != 10 {
			t.Fatalf("delivered id %d, want the committed row's 10", evt.ID)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("no frame")
	}
}

func TestHub_SlowStreamSubscriberIsClosedNotSkipped(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	room := hub.NewRoomID(uuid.New())
	h := hub.NewRoomHub(room, registry, makeLogger(), 0)
	cancel := startHub(t, h, registry)
	defer cancel()

	slow, err := h.SubscribeStream("_browser_slow")
	if err != nil {
		t.Fatal(err)
	}
	fast, err := h.SubscribeStream("_browser_fast")
	if err != nil {
		t.Fatal(err)
	}
	const n = 200
	var fastSeqs []int
	fastDone := make(chan struct{})
	go func() {
		defer close(fastDone)
		for evt := range fast {
			if evt.Sequence > 0 {
				fastSeqs = append(fastSeqs, evt.Sequence)
				if len(fastSeqs) == n {
					return
				}
			}
		}
	}()
	for i := 1; i <= n; i++ {
		h.Broadcast(hub.RoomEvent{Sequence: i, Type: hub.EventMessage, RoomID: room})
	}
	<-fastDone
	if len(fastSeqs) != n {
		t.Fatalf("fast subscriber got %d of %d frames: a slow stream must not hurt others", len(fastSeqs), n)
	}

	// The slow stream holds a gap-free prefix and is then closed, so its client resumes
	// from the last frame it saw instead of silently missing one.
	var got []int
	timeout := time.After(2 * time.Second)
	for closed := false; !closed; {
		select {
		case evt, ok := <-slow:
			if !ok {
				closed = true
				break
			}
			if evt.Sequence > 0 {
				got = append(got, evt.Sequence)
			}
		case <-timeout:
			t.Fatalf("slow stream was never closed (got %d frames)", len(got))
		}
	}
	if len(got) == 0 || len(got) >= n {
		t.Fatalf("slow stream got %d frames, want a bounded prefix", len(got))
	}
	for i, seq := range got {
		if seq != i+1 {
			t.Fatalf("slow stream frame %d has seq %d: a hole", i, seq)
		}
	}
	h.Unsubscribe("_browser_slow") // the handler's deferred unsubscribe is a no-op now
	h.Unsubscribe("_browser_fast")
}
