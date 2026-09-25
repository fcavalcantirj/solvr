package hub

import (
	"context"
	"io"
	"log/slog"
	"testing"

	"github.com/google/uuid"
)

func newAccessTestManager(t *testing.T) *HubManager {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	return NewHubManager(ctx, NewPresenceRegistry(), slog.New(slog.NewTextHandler(io.Discard, nil)), 0)
}

func signaled(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestAccessChanged_SignalsOnlyThatRoomsWatchers(t *testing.T) {
	m := newAccessTestManager(t)
	a, b := NewRoomID(uuid.New()), NewRoomID(uuid.New())
	a1, stopA1 := m.WatchAccess(a)
	defer stopA1()
	a2, stopA2 := m.WatchAccess(a)
	defer stopA2()
	b1, stopB1 := m.WatchAccess(b)
	defer stopB1()

	m.AccessChanged(a)
	if !signaled(a1) || !signaled(a2) {
		t.Fatal("every watcher of the changed room is signaled")
	}
	if signaled(b1) {
		t.Fatal("watchers of other rooms are not signaled")
	}
}

func TestAccessChanged_SignalsCoalesceAndNeverBlock(t *testing.T) {
	m := newAccessTestManager(t)
	id := NewRoomID(uuid.New())
	ch, stop := m.WatchAccess(id)
	defer stop()
	for i := 0; i < 5; i++ {
		m.AccessChanged(id) // an unread watcher must not block the caller
	}
	if !signaled(ch) {
		t.Fatal("pending signal delivered")
	}
	if signaled(ch) {
		t.Fatal("repeated changes coalesce into one pending signal")
	}
}

func TestAccessChangedAll_SignalsEveryRoomAndCancelUnregisters(t *testing.T) {
	m := newAccessTestManager(t)
	a, b := NewRoomID(uuid.New()), NewRoomID(uuid.New())
	chA, stopA := m.WatchAccess(a)
	chB, stopB := m.WatchAccess(b)
	defer stopB()

	m.AccessChangedAll()
	if !signaled(chA) || !signaled(chB) {
		t.Fatal("every watcher of every room is signaled")
	}

	stopA()
	stopA() // idempotent
	m.AccessChanged(a)
	if signaled(chA) {
		t.Fatal("a cancelled watcher is no longer signaled")
	}
	m.access.mu.Lock()
	_, kept := m.access.rooms[a]
	m.access.mu.Unlock()
	if kept {
		t.Fatal("a room without watchers is dropped from the registry")
	}
}
