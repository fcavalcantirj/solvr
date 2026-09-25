package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// Task: support the planner/executor review loop. These handler integration tests
// pin the review-loop HTTP surface — pin/unpin/list a directive (step 3) and post a
// revised directive that supersedes an earlier one (step 4) — against a local
// isolated Postgres (DATABASE_URL). They never SKIP silently when it is set.

// Step 3: an authenticated participant pins a directive, participants list pinned
// entries, and unpinning removes it.
func TestPinMessageHandler_PinListUnpin(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	directive := postAgentMsg(t, msgRepo, room.ID, "planner", "Directive: build the parser")
	handler := newDeliveryHandler(pool)

	withRoomAndID := func(method, id string) *http.Request {
		req := httptest.NewRequest(method, "/r/"+room.Slug+"/messages/"+id+"/pin", nil)
		req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
		return withSlugAndID(req, room.Slug, id)
	}

	// Pin.
	w := httptest.NewRecorder()
	handler.PinMessage(w, withRoomAndID(http.MethodPost, fmt.Sprintf("%d", directive.ID)))
	if w.Code != http.StatusOK {
		t.Fatalf("pin: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var pinResp struct {
		Data models.Message `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &pinResp); err != nil {
		t.Fatalf("pin: bad json: %v", err)
	}
	if pinResp.Data.PinnedAt == nil {
		t.Fatal("pin: expected pinned_at set in response")
	}

	// List pinned via GET /r/{slug}/pins.
	listReq := httptest.NewRequest(http.MethodGet, "/r/"+room.Slug+"/pins", nil)
	listReq = listReq.WithContext(context.WithValue(listReq.Context(), apimiddleware.RoomContextKey, room))
	lw := httptest.NewRecorder()
	handler.ListPinnedMessages(lw, listReq)
	if lw.Code != http.StatusOK {
		t.Fatalf("list pins: expected 200, got %d: %s", lw.Code, lw.Body.String())
	}
	var listResp struct {
		Data []models.Message `json:"data"`
	}
	if err := json.Unmarshal(lw.Body.Bytes(), &listResp); err != nil {
		t.Fatalf("list pins: bad json: %v", err)
	}
	if len(listResp.Data) != 1 || listResp.Data[0].ID != directive.ID {
		t.Fatalf("list pins: expected only the directive, got %+v", listResp.Data)
	}

	// Unpin.
	uw := httptest.NewRecorder()
	handler.UnpinMessage(uw, withRoomAndID(http.MethodDelete, fmt.Sprintf("%d", directive.ID)))
	if uw.Code != http.StatusOK {
		t.Fatalf("unpin: expected 200, got %d: %s", uw.Code, uw.Body.String())
	}
	lw2 := httptest.NewRecorder()
	handler.ListPinnedMessages(lw2, listReq)
	var listResp2 struct {
		Data []models.Message `json:"data"`
	}
	_ = json.Unmarshal(lw2.Body.Bytes(), &listResp2)
	if len(listResp2.Data) != 0 {
		t.Fatalf("list pins after unpin: expected 0, got %d", len(listResp2.Data))
	}
}

// Step 3: pinning an unknown message id in the room returns 404, not a fabricated pin.
func TestPinMessageHandler_UnknownID(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newDeliveryHandler(pool)

	req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/messages/999999/pin", nil)
	req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
	req = withSlugAndID(req, room.Slug, "999999")
	w := httptest.NewRecorder()
	handler.PinMessage(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for unknown message, got %d: %s", w.Code, w.Body.String())
	}
}

// Step 4: posting a revised directive with supersedes_entry_id succeeds and stores
// the explicit reference; the original stays in history.
func TestPostMessageHandler_Supersede(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	original := postAgentMsg(t, msgRepo, room.ID, "planner", "Directive v1: use REST")
	handler := newDeliveryHandler(pool)

	body := fmt.Sprintf(`{"agent_name":"planner","content":"Directive v2: use SSE","supersedes_entry_id":%d}`, original.ID)
	req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
	w := httptest.NewRecorder()
	handler.PostMessage(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data models.Message `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Data.SupersedesEntryID == nil || *resp.Data.SupersedesEntryID != original.ID {
		t.Fatalf("expected supersedes_entry_id=%d, got %v", original.ID, resp.Data.SupersedesEntryID)
	}

	// Original still readable in history.
	if _, err := msgRepo.GetByID(context.Background(), room.ID, original.ID); err != nil {
		t.Fatalf("original must remain in history: %v", err)
	}
}

// Step 4 (safety): a supersedes reference to a message in a DIFFERENT room is
// rejected with 400 rather than silently stored, and never leaks the other room.
func TestPostMessageHandler_SupersedeCrossRoomRejected(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	otherRoom := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	foreign := postAgentMsg(t, msgRepo, otherRoom.ID, "planner", "another room's directive")
	handler := newDeliveryHandler(pool)

	body := fmt.Sprintf(`{"agent_name":"planner","content":"revise","supersedes_entry_id":%d}`, foreign.ID)
	req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/message", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
	w := httptest.NewRecorder()
	handler.PostMessage(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for cross-room supersede, got %d: %s", w.Code, w.Body.String())
	}
}

// A revision of an entry that was already superseded is stale: it is refused with
// 409 SUPERSEDE_CONFLICT (idx 73 step 5) and nothing is stored, so a retried or
// stale directive can never fork or overwrite the newer one.
func TestPostMessageHandler_StaleSupersedeIsConflict(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	original := postAgentMsg(t, msgRepo, room.ID, "planner", "Directive v1: use REST")
	postAgentMsg(t, msgRepo, room.ID, "planner", "placeholder")
	handler := newDeliveryHandler(pool)

	post := func(content string) *httptest.ResponseRecorder {
		body := fmt.Sprintf(`{"agent_name":"planner","content":%q,"supersedes_entry_id":%d}`, content, original.ID)
		req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/message", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
		w := httptest.NewRecorder()
		handler.PostMessage(w, req)
		return w
	}

	if w := post("Directive v2: use SSE"); w.Code != http.StatusCreated {
		t.Fatalf("first revision: expected 201, got %d: %s", w.Code, w.Body.String())
	}
	w := post("Directive v2: use polling (stale)")
	if w.Code != http.StatusConflict {
		t.Fatalf("stale revision: expected 409, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Error.Code != "SUPERSEDE_CONFLICT" {
		t.Fatalf("error code = %q, want SUPERSEDE_CONFLICT", resp.Error.Code)
	}

	var n int
	if err := pool.QueryRow(context.Background(), `SELECT COUNT(*) FROM messages WHERE room_id = $1`, room.ID).Scan(&n); err != nil {
		t.Fatalf("count: %v", err)
	}
	if n != 3 {
		t.Fatalf("messages = %d, want 3 (the stale revision must not be stored)", n)
	}
}
