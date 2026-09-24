package handlers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// Same-room reference enforcement on the unified timeline: a reply or addressed
// participant that is not part of the posting room is a client error (400), never a
// 500, and never stored.

func TestPostMessageHandler_RejectsForeignReferences(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	otherRoom := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	foreign := postAgentMsg(t, msgRepo, otherRoom.ID, "other", "not yours")
	local := postAgentMsg(t, msgRepo, room.ID, "local", "yours")
	handler := newDeliveryHandler(pool)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/message", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
		w := httptest.NewRecorder()
		handler.PostMessage(w, req)
		return w
	}

	cases := []struct {
		name string
		body string
		want int
	}{
		{"cross-room reply", fmt.Sprintf(`{"agent_name":"p","content":"re","reply_to_entry_id":%d}`, foreign.ID), http.StatusBadRequest},
		{"unknown addressee", `{"agent_name":"p","content":"hey","addressed_member_ids":["agent-other"]}`, http.StatusBadRequest},
		{"same-room reply", fmt.Sprintf(`{"agent_name":"p","content":"re","reply_to_entry_id":%d}`, local.ID), http.StatusCreated},
		{"same-room addressee", `{"agent_name":"p","content":"hey","addressed_member_ids":["local"]}`, http.StatusCreated},
	}
	for _, tc := range cases {
		w := post(tc.body)
		if w.Code != tc.want {
			t.Errorf("%s: status %d, want %d: %s", tc.name, w.Code, tc.want, w.Body.String())
		}
		if tc.want == http.StatusBadRequest && !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
			t.Errorf("%s: body %s, want VALIDATION_ERROR", tc.name, w.Body.String())
		}
	}

	var stored int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM messages WHERE room_id = $1`, room.ID).Scan(&stored); err != nil {
		t.Fatalf("count: %v", err)
	}
	if stored != 3 { // the seed message plus the two accepted posts
		t.Errorf("stored messages = %d, want 3 (rejected references must not be stored)", stored)
	}
}

func TestPostHumanMessage_InvalidReferenceIs400(t *testing.T) {
	room := testRoomMsgRoom()
	handler := newTestHumanMsgHandler(room, db.ErrInvalidEntryReference, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/messages",
		strings.NewReader(`{"content":"re","reply_to_entry_id":999}`))
	req.Header.Set("Content-Type", "application/json")
	req = addRoomMsgJWTCtx(req, "user-123")
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)

	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "VALIDATION_ERROR") {
		t.Fatalf("status %d body %s, want 400 VALIDATION_ERROR", w.Code, w.Body.String())
	}
}
