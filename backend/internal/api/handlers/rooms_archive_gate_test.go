package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// archivedTestRoom returns a room already marked Finished.
func archivedTestRoom() *models.Room {
	room := testRoomMsgRoom()
	now := time.Now()
	room.ArchivedAt = &now
	return room
}

// assertRoomArchived checks a 409 response carrying the ROOM_ARCHIVED code.
func assertRoomArchived(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for archived room, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON error response: %v", err)
	}
	if resp.Error.Code != "ROOM_ARCHIVED" {
		t.Errorf("expected error code ROOM_ARCHIVED, got %q", resp.Error.Code)
	}
}

// Step 3: an agent posting to a finished room is refused with a clear archived response.
func TestPostMessage_ArchivedRoomRefused(t *testing.T) {
	room := archivedTestRoom()
	handler := &RoomMessagesHandler{} // gate must fire before any repo use

	body := `{"agent_name":"planner","content":"still here?"}`
	req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
	w := httptest.NewRecorder()

	handler.PostMessage(w, req)
	assertRoomArchived(t, w)
}

// Step 3: a human comment on a finished room is refused with a clear archived response.
func TestPostHumanMessage_ArchivedRoomRefused(t *testing.T) {
	room := archivedTestRoom()
	handler := newTestHumanMsgHandler(room, nil, nil)

	body := `{"content":"can I still comment?"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addRoomMsgJWTCtx(req, "user-123")
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)
	assertRoomArchived(t, w)
}

// Step 3: joining a finished room is refused with a clear archived response.
func TestJoinRoom_ArchivedRoomRefused(t *testing.T) {
	room := archivedTestRoom()
	handler := &RoomPresenceHandler{} // gate must fire before any repo use

	body := `{"agent_name":"executor"}`
	req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/join", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
	w := httptest.NewRecorder()

	handler.JoinRoom(w, req)
	assertRoomArchived(t, w)
}
