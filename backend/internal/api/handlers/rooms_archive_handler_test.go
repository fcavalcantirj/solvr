package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// newArchiveRoomHandler builds a RoomHandler backed by real repos for the given pool.
func newArchiveRoomHandler(pool *db.Pool) *RoomHandler {
	return NewRoomHandler(
		db.NewRoomRepository(pool),
		db.NewMessageRepository(pool),
		db.NewAgentPresenceRepository(pool),
		db.NewRoomMemberRepository(pool),
		db.NewRoomAgentTokenRepository(pool),
		db.NewRoomEventRepository(pool),
	)
}

// adminCtx injects admin JWT claims (canManage grants admins on any room).
func adminCtx(r *http.Request) *http.Request {
	claims := &auth.Claims{UserID: uuid.NewString(), Email: "admin@example.com", Role: "admin"}
	return r.WithContext(auth.ContextWithClaims(r.Context(), claims))
}

// userCtx injects a non-owner, non-admin human's claims.
func userCtx(r *http.Request) *http.Request {
	claims := &auth.Claims{UserID: uuid.NewString(), Email: "user@example.com", Role: "user"}
	return r.WithContext(auth.ContextWithClaims(r.Context(), claims))
}

// Steps 1 & 3: an owner finishes a room (recording a result message), the API records
// the archived state, and reopening clears it.
func TestArchiveRoom_ArchivesAndReopens(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)
	msg := postAgentMsg(t, db.NewMessageRepository(pool), room.ID, "res-1", "final result")

	// Archive with a result-message reference.
	body := fmt.Sprintf(`{"result_message_id": %d}`, msg.ID)
	req := adminCtx(withSlug(httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/archive", strings.NewReader(body)), room.Slug))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ArchiveRoom(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("archive: expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data models.Room `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Data.ArchivedAt == nil {
		t.Error("archive response has nil archived_at")
	}
	if resp.Data.ResultMessageID == nil || *resp.Data.ResultMessageID != msg.ID {
		t.Errorf("result_message_id = %v, want %d", resp.Data.ResultMessageID, msg.ID)
	}

	// The archived state persists and the room is still readable.
	stored, err := db.NewRoomRepository(pool).GetBySlug(context.Background(), room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug after archive: %v", err)
	}
	if stored.ArchivedAt == nil {
		t.Error("stored room not archived")
	}

	// Reopen clears it.
	req2 := adminCtx(withSlug(httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/reopen", nil), room.Slug))
	w2 := httptest.NewRecorder()
	handler.ReopenRoom(w2, req2)
	if w2.Code != http.StatusOK {
		t.Fatalf("reopen: expected 200, got %d: %s", w2.Code, w2.Body.String())
	}
	reopened, err := db.NewRoomRepository(pool).GetBySlug(context.Background(), room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug after reopen: %v", err)
	}
	if reopened.ArchivedAt != nil {
		t.Error("room still archived after reopen")
	}
}

// Only the owner or an admin may finish a room.
func TestArchiveRoom_NonOwnerForbidden(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool) // ownerless
	handler := newArchiveRoomHandler(pool)

	req := userCtx(withSlug(httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/archive", strings.NewReader(`{}`)), room.Slug))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ArchiveRoom(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for non-owner, got %d: %s", w.Code, w.Body.String())
	}
}

// A result_message_id from a different room is rejected.
func TestArchiveRoom_ResultMessageMustBelongToRoom(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	other := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)
	foreign := postAgentMsg(t, db.NewMessageRepository(pool), other.ID, "res-x", "belongs elsewhere")

	body := fmt.Sprintf(`{"result_message_id": %d}`, foreign.ID)
	req := adminCtx(withSlug(httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/archive", strings.NewReader(body)), room.Slug))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ArchiveRoom(w, req)
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for foreign result message, got %d: %s", w.Code, w.Body.String())
	}
}

// Archiving an unknown room returns 404.
func TestArchiveRoom_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	handler := newArchiveRoomHandler(pool)
	req := adminCtx(withSlug(httptest.NewRequest(http.MethodPost, "/v1/rooms/nope-nope/archive", strings.NewReader(`{}`)), "nope-nope"))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	handler.ArchiveRoom(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d: %s", w.Code, w.Body.String())
	}
}
