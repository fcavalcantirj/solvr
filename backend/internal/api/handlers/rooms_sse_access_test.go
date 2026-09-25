package handlers

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// An open stream re-authorizes on access-change signals and on every heartbeat, so a
// caller who lost access stops receiving the room even when no signal reaches this
// instance.

// accessStreamRun serves one stream whose guard left recheck in the request context and
// returns a channel closed when the handler returned, plus the recorder and a stop func.
func accessStreamRun(t *testing.T, heartbeat time.Duration, recheck apimiddleware.RoomAccessRecheck) (*hub.HubManager, *models.Room, chan struct{}, *flushRecorder, func()) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	hubMgr := hub.NewHubManager(ctx, hub.NewPresenceRegistry(), slog.New(slog.NewTextHandler(io.Discard, nil)), 100)
	room := testSSERoom()
	handler := &RoomSSEHandler{hubMgr: hubMgr, heartbeatInterval: heartbeat}

	reqCtx, stop := context.WithCancel(context.Background())
	t.Cleanup(stop)
	reqCtx = context.WithValue(reqCtx, apimiddleware.RoomContextKey, room)
	reqCtx = apimiddleware.WithRoomAccessRecheck(reqCtx, recheck)
	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/"+room.Slug+"/stream", nil).WithContext(reqCtx)
	w := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}
	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.PublicStream(w, req)
	}()
	return hubMgr, room, done, w, stop
}

// allowedFor returns a recheck that allows the first n calls and refuses after.
func allowedFor(n int32) (apimiddleware.RoomAccessRecheck, *atomic.Int32) {
	var calls atomic.Int32
	return func(context.Context) (bool, error) {
		return calls.Add(1) <= n, nil
	}, &calls
}

func ended(done chan struct{}, d time.Duration) bool {
	select {
	case <-done:
		return true
	case <-time.After(d):
		return false
	}
}

func TestStreamAccess_HeartbeatRecheckEndsStreamWithoutAnySignal(t *testing.T) {
	recheck, _ := allowedFor(1) // the check right after watching passes
	_, _, done, w, _ := accessStreamRun(t, 50*time.Millisecond, recheck)
	if !ended(done, 2*time.Second) {
		t.Fatal("stream must end at the next heartbeat once access is gone")
	}
	if !strings.Contains(w.Body.String(), "event: access_revoked") {
		t.Fatalf("stream must say why it ended, got %q", w.Body.String())
	}
}

func TestStreamAccess_SignalRecheckEndsStreamBeforeHeartbeat(t *testing.T) {
	recheck, calls := allowedFor(1)
	hubMgr, room, done, _, _ := accessStreamRun(t, time.Hour, recheck)
	deadline := time.Now().Add(time.Second)
	for calls.Load() < 1 && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if ended(done, 100*time.Millisecond) {
		t.Fatal("stream ended before access changed")
	}
	hubMgr.AccessChanged(hub.NewRoomID(room.ID))
	if !ended(done, 2*time.Second) {
		t.Fatal("an access-change signal must re-check at once, not at the next heartbeat")
	}
}

func TestStreamAccess_RevokedBetweenGuardAndWatchIsCaughtAtOnce(t *testing.T) {
	recheck, _ := allowedFor(0)
	_, _, done, w, _ := accessStreamRun(t, time.Hour, recheck)
	if !ended(done, 2*time.Second) {
		t.Fatal("a revocation that landed before the watch started must still end the stream")
	}
	if !strings.Contains(w.Body.String(), "event: access_revoked") {
		t.Fatalf("stream must say why it ended, got %q", w.Body.String())
	}
}

func TestStreamAccess_UnreadableDecisionKeepsStream(t *testing.T) {
	var calls atomic.Int32
	recheck := func(context.Context) (bool, error) {
		calls.Add(1)
		return false, errors.New("database unavailable")
	}
	_, _, done, _, stop := accessStreamRun(t, 20*time.Millisecond, recheck)
	if ended(done, 300*time.Millisecond) {
		t.Fatal("a failed re-check must not end the stream")
	}
	if calls.Load() < 3 {
		t.Fatalf("expected repeated re-checks on heartbeats, got %d", calls.Load())
	}
	stop()
	if !ended(done, 2*time.Second) {
		t.Fatal("stream ends with its request")
	}
}
