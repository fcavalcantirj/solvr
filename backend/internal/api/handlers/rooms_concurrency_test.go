package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// ============================================================================
// PATCH /v1/rooms/{slug} — optimistic concurrency (If-Match) tests
// Pins idx 73 step 5: "Require version or If-Match on conflicting edits and
// reject stale updates; ensure retries cannot overwrite a newer directive,
// change ownership, or duplicate a publication."
// Integration tests: they need DATABASE_URL pointed at a local test database.
// ============================================================================

// roomPatchRequest builds an admin PATCH /v1/rooms/{slug} request carrying the
// given JSON body and an optional If-Match precondition.
func roomPatchRequest(slug, body, ifMatch string) *http.Request {
	req := httptest.NewRequest(http.MethodPatch, "/v1/rooms/"+slug, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if ifMatch != "" {
		req.Header.Set("If-Match", ifMatch)
	}
	return adminCtx(withSlug(req, slug))
}

// TestGetRoom_ExposesETag verifies GET /v1/rooms/{slug} hands the client the
// room's version validator so it can be echoed as an If-Match precondition.
func TestGetRoom_ExposesETag(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)

	w := httptest.NewRecorder()
	handler.GetRoom(w, withSlug(httptest.NewRequest(http.MethodGet, "/v1/rooms/"+room.Slug, nil), room.Slug))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if got, want := w.Header().Get("ETag"), roomETag(room.UpdatedAt); got != want {
		t.Errorf("ETag = %q, want %q", got, want)
	}
}

// TestUpdateRoom_StaleIfMatchRejected verifies a stale If-Match yields 412
// PRECONDITION_FAILED, echoes the current validator, and persists nothing.
func TestUpdateRoom_StaleIfMatchRejected(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)

	w := httptest.NewRecorder()
	handler.UpdateRoom(w, roomPatchRequest(room.Slug, `{"description":"stale write"}`, `"0"`))
	if w.Code != http.StatusPreconditionFailed {
		t.Fatalf("expected 412, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if resp.Error.Code != "PRECONDITION_FAILED" {
		t.Errorf("error.code = %q, want PRECONDITION_FAILED", resp.Error.Code)
	}
	if got, want := w.Header().Get("ETag"), roomETag(room.UpdatedAt); got != want {
		t.Errorf("ETag = %q, want current %q", got, want)
	}

	stored, err := db.NewRoomRepository(pool).GetBySlug(context.Background(), room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if stored.Description != nil && *stored.Description == "stale write" {
		t.Error("a stale update must not be persisted")
	}
	if !stored.UpdatedAt.Equal(room.UpdatedAt) {
		t.Errorf("updated_at moved from %v to %v on a rejected write", room.UpdatedAt, stored.UpdatedAt)
	}
}

// TestUpdateRoom_MatchingIfMatchThenReplayRejected verifies a current If-Match
// is applied and returns the new validator, and that replaying the same
// request with the old validator cannot overwrite the newer revision.
func TestUpdateRoom_MatchingIfMatchThenReplayRejected(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)
	original := roomETag(room.UpdatedAt)

	w := httptest.NewRecorder()
	handler.UpdateRoom(w, roomPatchRequest(room.Slug, `{"description":"first edit"}`, original))
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	stored, err := db.NewRoomRepository(pool).GetBySlug(context.Background(), room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if stored.Description == nil || *stored.Description != "first edit" {
		t.Fatalf("description = %v, want first edit", stored.Description)
	}
	if got, want := w.Header().Get("ETag"), roomETag(stored.UpdatedAt); got != want {
		t.Errorf("response ETag = %q, want new validator %q", got, want)
	}

	// A retry still carrying the original validator must not clobber the edit.
	w2 := httptest.NewRecorder()
	handler.UpdateRoom(w2, roomPatchRequest(room.Slug, `{"description":"retried stale"}`, original))
	if w2.Code != http.StatusPreconditionFailed {
		t.Fatalf("replay: expected 412, got %d: %s", w2.Code, w2.Body.String())
	}
	after, err := db.NewRoomRepository(pool).GetBySlug(context.Background(), room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if after.Description == nil || *after.Description != "first edit" {
		t.Errorf("description = %v, want first edit preserved", after.Description)
	}
}

// TestUpdateRoom_MissingIfMatchIs428 verifies If-Match is required (spec.json
// idx 74 step 5, owner decision 8 of 2026-09-30): an edit without it is 428
// PRECONDITION_REQUIRED, persists nothing, and hands out no ETag.
func TestUpdateRoom_MissingIfMatchIs428(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)

	w := httptest.NewRecorder()
	handler.UpdateRoom(w, roomPatchRequest(room.Slug, `{"description":"unconditional"}`, ""))
	if w.Code != http.StatusPreconditionRequired {
		t.Fatalf("expected 428, got %d: %s", w.Code, w.Body.String())
	}
	if code := errorCode(t, w); code != "PRECONDITION_REQUIRED" {
		t.Errorf("error.code = %q, want PRECONDITION_REQUIRED", code)
	}
	if got := w.Header().Get("ETag"); got != "" {
		t.Errorf("a 428 must not hand out the current ETag, got %q", got)
	}
	stored, err := db.NewRoomRepository(pool).GetBySlug(context.Background(), room.Slug)
	if err != nil {
		t.Fatalf("GetBySlug: %v", err)
	}
	if !stored.UpdatedAt.Equal(room.UpdatedAt) || (stored.Description != nil && *stored.Description == "unconditional") {
		t.Errorf("an edit without If-Match changed the room: %v %v", stored.UpdatedAt, stored.Description)
	}
}

// TestUpdateRoom_NonManagerWithoutIfMatchGetsForbidden verifies the manage
// check runs before the precondition is required: 403, not 428.
func TestUpdateRoom_NonManagerWithoutIfMatchGetsForbidden(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)

	req := httptest.NewRequest(http.MethodPatch, "/v1/rooms/"+room.Slug, strings.NewReader(`{"description":"x"}`))
	w := httptest.NewRecorder()
	handler.UpdateRoom(w, userCtx(withSlug(req, room.Slug)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}

// TestUpdateRoom_ConcurrentEditsAtOneVersionHaveOneWinner verifies the check
// and the write are one decision: edits racing with the same current If-Match
// cannot all pass the check and then overwrite each other. Exactly one is
// applied; every other one is 412.
func TestUpdateRoom_ConcurrentEditsAtOneVersionHaveOneWinner(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)
	version := roomETag(room.UpdatedAt)

	const writers = 12
	codes := make(chan int, writers)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < writers; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			w := httptest.NewRecorder()
			handler.UpdateRoom(w, roomPatchRequest(room.Slug, `{"description":"writer `+strconv.Itoa(i)+`"}`, version))
			codes <- w.Code
		}(i)
	}
	close(start)
	wg.Wait()
	close(codes)
	counts := map[int]int{}
	for c := range codes {
		counts[c]++
	}
	if counts[http.StatusOK] != 1 || counts[http.StatusPreconditionFailed] != writers-1 {
		t.Errorf("%d concurrent edits at one version: %v; want exactly one 200 and %d x 412", writers, counts, writers-1)
	}
}

// TestUpdateRoom_NonManagerGetsForbiddenBeforePrecondition verifies the
// ownership check runs first, so a non-manager learns nothing about a room's
// version from a 412 and receives 403.
func TestUpdateRoom_NonManagerGetsForbiddenBeforePrecondition(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newArchiveRoomHandler(pool)

	req := httptest.NewRequest(http.MethodPatch, "/v1/rooms/"+room.Slug, strings.NewReader(`{"description":"x"}`))
	req.Header.Set("If-Match", `"0"`)
	w := httptest.NewRecorder()
	handler.UpdateRoom(w, userCtx(withSlug(req, room.Slug)))
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d: %s", w.Code, w.Body.String())
	}
}
