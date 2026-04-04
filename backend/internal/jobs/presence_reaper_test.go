package jobs

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// testLogger returns a discard logger suitable for unit tests.
func testLogger() *slog.Logger {
	return slog.New(slog.DiscardHandler)
}

// --- Mocks ---

// mockPresenceExpirer implements PresenceExpirer for testing.
type mockPresenceExpirer struct {
	expired []models.ExpiredPresence
	err     error
	called  bool
}

func (m *mockPresenceExpirer) DeleteExpired(ctx context.Context) ([]models.ExpiredPresence, error) {
	m.called = true
	if m.err != nil {
		return nil, m.err
	}
	return m.expired, nil
}

// mockRoomExpirer implements RoomExpirer for testing.
type mockRoomExpirer struct {
	expiredCount int64
	err          error
	called       bool
}

func (m *mockRoomExpirer) DeleteExpiredRooms(ctx context.Context) (int64, error) {
	m.called = true
	if m.err != nil {
		return 0, m.err
	}
	return m.expiredCount, nil
}

// mockHubForReaper tracks Unsubscribe calls on a per-room basis.
type mockHubForReaper struct {
	unsubscribeCalls []unsubscribeCall
}

type unsubscribeCall struct {
	roomID    hub.RoomID
	agentName string
}

// --- Tests ---

// TestPresenceReaper_RunOnce_NoExpired tests that RunOnce returns zero counts when nothing expired.
func TestPresenceReaper_RunOnce_NoExpired(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(registry, testLogger(), 0)

	pe := &mockPresenceExpirer{expired: []models.ExpiredPresence{}}
	re := &mockRoomExpirer{expiredCount: 0}

	job := NewPresenceReaperJob(pe, re, registry, hubMgr)
	result := job.RunOnce(context.Background())

	if result.ExpiredAgents != 0 {
		t.Errorf("ExpiredAgents = %d, want 0", result.ExpiredAgents)
	}
	if result.ExpiredRooms != 0 {
		t.Errorf("ExpiredRooms = %d, want 0", result.ExpiredRooms)
	}
	if !pe.called {
		t.Error("PresenceExpirer.DeleteExpired was not called")
	}
	if !re.called {
		t.Error("RoomExpirer.DeleteExpiredRooms was not called")
	}
}

// TestPresenceReaper_RunOnce_WithExpiredAgents tests that RunOnce handles expired agents
// by removing them from registry and unsubscribing from hub.
func TestPresenceReaper_RunOnce_WithExpiredAgents(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(registry, testLogger(), 0)

	roomID1 := uuid.New()
	roomID2 := uuid.New()

	// Pre-populate registry so Remove has something to remove.
	registry.Add(hub.NewRoomID(roomID1), "agent-alpha", nil)
	registry.Add(hub.NewRoomID(roomID2), "agent-beta", nil)

	pe := &mockPresenceExpirer{
		expired: []models.ExpiredPresence{
			{RoomID: roomID1, AgentName: "agent-alpha"},
			{RoomID: roomID2, AgentName: "agent-beta"},
		},
	}
	re := &mockRoomExpirer{expiredCount: 0}

	job := NewPresenceReaperJob(pe, re, registry, hubMgr)
	result := job.RunOnce(context.Background())

	if result.ExpiredAgents != 2 {
		t.Errorf("ExpiredAgents = %d, want 2", result.ExpiredAgents)
	}

	// Verify agents were removed from registry.
	if _, found := registry.Get(hub.NewRoomID(roomID1), "agent-alpha"); found {
		t.Error("agent-alpha should have been removed from registry")
	}
	if _, found := registry.Get(hub.NewRoomID(roomID2), "agent-beta"); found {
		t.Error("agent-beta should have been removed from registry")
	}
}

// TestPresenceReaper_RunOnce_WithExpiredRooms tests that RunOnce reports expired room count.
func TestPresenceReaper_RunOnce_WithExpiredRooms(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(registry, testLogger(), 0)

	pe := &mockPresenceExpirer{expired: []models.ExpiredPresence{}}
	re := &mockRoomExpirer{expiredCount: 3}

	job := NewPresenceReaperJob(pe, re, registry, hubMgr)
	result := job.RunOnce(context.Background())

	if result.ExpiredAgents != 0 {
		t.Errorf("ExpiredAgents = %d, want 0", result.ExpiredAgents)
	}
	if result.ExpiredRooms != 3 {
		t.Errorf("ExpiredRooms = %d, want 3", result.ExpiredRooms)
	}
}

// TestPresenceReaper_RunOnce_PresenceLeaveEvent verifies that the reaper emits
// presence_leave events through hub.Unsubscribe when agents expire (D-27).
func TestPresenceReaper_RunOnce_PresenceLeaveEvent(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(registry, testLogger(), 0)

	roomID := uuid.New()
	roomIDHub := hub.NewRoomID(roomID)

	// Create a hub for the room so Get() returns non-nil.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	roomHub := hubMgr.GetOrCreate(ctx, roomIDHub)

	// Subscribe an agent so Unsubscribe has something to remove.
	registry.Add(roomIDHub, "expiring-agent", nil)
	ch, err := roomHub.Subscribe("expiring-agent", nil)
	if err != nil {
		t.Fatalf("Subscribe failed: %v", err)
	}

	pe := &mockPresenceExpirer{
		expired: []models.ExpiredPresence{
			{RoomID: roomID, AgentName: "expiring-agent"},
		},
	}
	re := &mockRoomExpirer{expiredCount: 0}

	job := NewPresenceReaperJob(pe, re, registry, hubMgr)
	result := job.RunOnce(context.Background())

	if result.ExpiredAgents != 1 {
		t.Errorf("ExpiredAgents = %d, want 1", result.ExpiredAgents)
	}

	// The subscriber channel should be closed by Unsubscribe.
	// Try to drain it — we expect it to be closed eventually.
	select {
	case _, ok := <-ch:
		if ok {
			// May receive a presence_leave event before close — that's fine.
			// Drain until closed.
			for range ch {
			}
		}
		// Channel closed — Unsubscribe was called, presence_leave event emitted.
	case <-time.After(2 * time.Second):
		t.Error("subscriber channel was not closed by Unsubscribe within timeout")
	}
}

// TestPresenceReaper_RunOnce_ErrorsDoNotCrash tests that errors in one step don't prevent others.
func TestPresenceReaper_RunOnce_ErrorsDoNotCrash(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(registry, testLogger(), 0)

	pe := &mockPresenceExpirer{err: errors.New("db connection lost")}
	re := &mockRoomExpirer{expiredCount: 2}

	job := NewPresenceReaperJob(pe, re, registry, hubMgr)
	result := job.RunOnce(context.Background())

	// Presence step failed, but room step should still run.
	if result.ExpiredAgents != 0 {
		t.Errorf("ExpiredAgents = %d, want 0 (error case)", result.ExpiredAgents)
	}
	if result.ExpiredRooms != 2 {
		t.Errorf("ExpiredRooms = %d, want 2", result.ExpiredRooms)
	}
}

// TestPresenceReaper_RunScheduled_ContextCancellation tests that RunScheduled
// stops cleanly when its context is canceled.
func TestPresenceReaper_RunScheduled_ContextCancellation(t *testing.T) {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(registry, testLogger(), 0)

	pe := &mockPresenceExpirer{expired: []models.ExpiredPresence{}}
	re := &mockRoomExpirer{expiredCount: 0}

	job := NewPresenceReaperJob(pe, re, registry, hubMgr)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})

	go func() {
		job.RunScheduled(ctx, 10*time.Millisecond)
		close(done)
	}()

	// Wait for at least one tick.
	time.Sleep(50 * time.Millisecond)

	// Stop the job.
	cancel()

	// Wait for clean shutdown.
	select {
	case <-done:
		// Success — job stopped cleanly.
	case <-time.After(2 * time.Second):
		t.Fatal("RunScheduled did not stop within timeout after context cancellation")
	}
}

// TestPresenceReaper_DefaultInterval verifies the default interval constant.
func TestPresenceReaper_DefaultInterval(t *testing.T) {
	if DefaultPresenceReaperInterval != 60*time.Second {
		t.Errorf("DefaultPresenceReaperInterval = %v, want 60s", DefaultPresenceReaperInterval)
	}
}
