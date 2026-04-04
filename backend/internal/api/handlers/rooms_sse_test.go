package handlers

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// testSSERoom returns a minimal valid room for SSE tests.
func testSSERoom() *models.Room {
	id, _ := uuid.NewRandom()
	return &models.Room{
		ID:           id,
		Slug:         "sse-test-room",
		DisplayName:  "SSE Test Room",
		IsPrivate:    false,
		Tags:         []string{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}
}

// newTestSSEHandler builds a RoomSSEHandler with an injectable room lookup for tests.
func newTestSSEHandler(room *models.Room, roomErr error) *RoomSSEHandler {
	return &RoomSSEHandler{
		testRoomLookup: func(_ context.Context, _ string) (*models.Room, error) {
			if roomErr != nil {
				return nil, roomErr
			}
			return room, nil
		},
	}
}

// --- Tests for PublicStream ---

// TestPublicStream_NotFound checks that GET /{nonexistent}/stream returns 404.
func TestPublicStream_NotFound(t *testing.T) {
	handler := newTestSSEHandler(nil, db.ErrRoomNotFound)

	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/nonexistent/stream", nil)
	req = withSlug(req, "nonexistent")
	w := httptest.NewRecorder()

	handler.PublicStream(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}

// TestPublicStream_StreamHeadersAndNoAuth checks that the SSE stream:
// - Returns 200 without any bearer token (public route)
// - Sets Content-Type: text/event-stream
func TestPublicStream_StreamHeadersAndNoAuth(t *testing.T) {
	room := testSSERoom()

	// Build a hub manager with short-lived context so stream exits quickly.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(ctx, registry, logger, 100)

	handler := &RoomSSEHandler{
		hubMgr: hubMgr,
		testRoomLookup: func(_ context.Context, _ string) (*models.Room, error) {
			return room, nil
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/"+room.Slug+"/stream", nil)
	req = withSlug(req, room.Slug)
	// No Authorization header — public stream requires no auth
	w := &flushRecorder{ResponseRecorder: httptest.NewRecorder()}

	done := make(chan struct{})
	go func() {
		defer close(done)
		handler.PublicStream(w, req)
	}()

	select {
	case <-done:
	case <-time.After(500 * time.Millisecond):
		// Stream runs until context is cancelled — this is expected
	}

	ct := w.Header().Get("Content-Type")
	if ct != "text/event-stream" {
		t.Errorf("expected Content-Type=text/event-stream, got %q", ct)
	}
}

// flushRecorder wraps httptest.ResponseRecorder to implement http.Flusher.
type flushRecorder struct {
	*httptest.ResponseRecorder
}

func (f *flushRecorder) Flush() {
	f.ResponseRecorder.Flush()
}
