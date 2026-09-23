package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// These integration tests pin the "dependable message delivery and recovery"
// contract: idempotent writes keyed by client_entry_id (step 4), before/after
// history pagination and stable single-message lookup for deep links (step 6),
// cursor replay without gaps (step 2), and durability across a fresh repo (step 5).
// They run against a local isolated Postgres (DATABASE_URL); they never SKIP silently
// when the harness sets DATABASE_URL, which is required to prove behavior.

func newDeliveryRoom(t *testing.T, pool *db.Pool) *models.Room {
	t.Helper()
	ctx := context.Background()
	roomRepo := db.NewRoomRepository(pool)
	slug := fmt.Sprintf("deliv-%d", time.Now().UnixNano())
	room, _, err := roomRepo.Create(ctx, models.CreateRoomParams{
		Slug:        slug,
		DisplayName: "Delivery Test Room",
		IsPrivate:   false,
		OwnerID:     uuid.Nil,
	})
	if err != nil {
		t.Fatalf("create room: %v", err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM messages WHERE room_id=$1`, room.ID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM rooms WHERE id=$1`, room.ID)
	})
	return room
}

func postAgentMsg(t *testing.T, msgRepo *db.MessageRepository, roomID uuid.UUID, authorID, content string) *models.Message {
	t.Helper()
	author := authorID
	msg, err := msgRepo.Create(context.Background(), models.CreateMessageParams{
		RoomID:      roomID,
		AuthorType:  "agent",
		AuthorID:    &author,
		AgentName:   "agent-" + authorID,
		Content:     content,
		ContentType: "text",
	})
	if err != nil {
		t.Fatalf("create message: %v", err)
	}
	return msg
}

// Step 4: a retry with the same authenticated author + client_entry_id returns the
// already-persisted entry instead of duplicating it.
func TestMessageDelivery_ClientEntryIdempotentRetry(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)

	author := "agent-idem-1"
	key := "client-entry-abc"
	params := models.CreateMessageParams{
		RoomID:        room.ID,
		AuthorType:    "agent",
		AuthorID:      &author,
		AgentName:     "planner",
		Content:       "first write",
		ContentType:   "text",
		ClientEntryID: &key,
	}

	first, created1, err := msgRepo.CreateWithClientEntry(ctx, params)
	if err != nil {
		t.Fatalf("first write: %v", err)
	}
	if !created1 {
		t.Fatalf("expected first write to create a new entry")
	}

	// Retry the same logical write (client re-sends after a lost response).
	params.Content = "retry payload should be ignored"
	second, created2, err := msgRepo.CreateWithClientEntry(ctx, params)
	if err != nil {
		t.Fatalf("retry write: %v", err)
	}
	if created2 {
		t.Fatalf("expected retry to be idempotent (created=false), got a new entry")
	}
	if second.ID != first.ID {
		t.Fatalf("expected retry to return existing entry id %d, got %d", first.ID, second.ID)
	}
	if second.Content != "first write" {
		t.Fatalf("expected original content preserved, got %q", second.Content)
	}

	var count int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM messages WHERE room_id=$1 AND deleted_at IS NULL`, room.ID).Scan(&count); err != nil {
		t.Fatalf("count: %v", err)
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 persisted message after retry, got %d", count)
	}

	// A different author reusing the same key is NOT deduplicated (idempotency is
	// scoped to the authenticated author).
	other := "agent-idem-2"
	params.AuthorID = &other
	params.Content = "different author, same key"
	_, createdOther, err := msgRepo.CreateWithClientEntry(ctx, params)
	if err != nil {
		t.Fatalf("other-author write: %v", err)
	}
	if !createdOther {
		t.Fatalf("expected a different author to create its own entry")
	}
}

// Step 6: bounded before-pagination returns earlier history anchored and contiguous.
func TestMessageDelivery_ListBeforeAnchoredNoGaps(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)

	var ids []int64
	for i := 0; i < 6; i++ {
		m := postAgentMsg(t, msgRepo, room.ID, "a", fmt.Sprintf("msg-%d", i))
		ids = append(ids, m.ID)
	}

	// Anchor on the 5th message; ask for the 3 before it.
	before := ids[4]
	earlier, err := msgRepo.ListBefore(ctx, room.ID, before, 3)
	if err != nil {
		t.Fatalf("ListBefore: %v", err)
	}
	if len(earlier) != 3 {
		t.Fatalf("expected 3 earlier messages, got %d", len(earlier))
	}
	// Must be ascending and contiguous: ids[1], ids[2], ids[3].
	want := []int64{ids[1], ids[2], ids[3]}
	for i, m := range earlier {
		if m.ID != want[i] {
			t.Fatalf("expected earlier[%d].ID=%d, got %d (want ascending contiguous)", i, want[i], m.ID)
		}
	}
}

// Step 6: stable single-message lookup, room-scoped so a deep link cannot fetch
// another room's message.
func TestMessageDelivery_GetByIDRoomScoped(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	roomA := newDeliveryRoom(t, pool)
	roomB := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)

	m := postAgentMsg(t, msgRepo, roomA.ID, "a", "deep-linkable message")

	got, err := msgRepo.GetByID(ctx, roomA.ID, m.ID)
	if err != nil {
		t.Fatalf("GetByID same room: %v", err)
	}
	if got.ID != m.ID || got.Content != "deep-linkable message" {
		t.Fatalf("GetByID returned wrong message: %+v", got)
	}

	// The same id looked up under a different room must not leak.
	if _, err := msgRepo.GetByID(ctx, roomB.ID, m.ID); err != db.ErrMessageNotFound {
		t.Fatalf("expected ErrMessageNotFound for cross-room lookup, got %v", err)
	}

	// A non-existent id is not found.
	if _, err := msgRepo.GetByID(ctx, roomA.ID, m.ID+999999); err != db.ErrMessageNotFound {
		t.Fatalf("expected ErrMessageNotFound for missing id, got %v", err)
	}
}

// Step 2: reconnect with a cursor and replay missed messages without gaps.
func TestMessageDelivery_ListAfterReplayNoGaps(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)

	first := postAgentMsg(t, msgRepo, room.ID, "a", "before disconnect")
	// Client disconnects here holding cursor = first.ID; three more arrive.
	var missed []int64
	for i := 0; i < 3; i++ {
		missed = append(missed, postAgentMsg(t, msgRepo, room.ID, "b", fmt.Sprintf("missed-%d", i)).ID)
	}

	replay, err := msgRepo.ListAfter(ctx, room.ID, first.ID, 100)
	if err != nil {
		t.Fatalf("ListAfter: %v", err)
	}
	if len(replay) != 3 {
		t.Fatalf("expected 3 replayed messages, got %d", len(replay))
	}
	for i, m := range replay {
		if m.ID != missed[i] {
			t.Fatalf("replay gap: expected id %d at %d, got %d", missed[i], i, m.ID)
		}
	}
}

// Step 5: persisted messages and cursors survive a fresh repository instance
// (a proxy for an API process restart — the durable store is the source of truth).
func TestMessageDelivery_MessagesSurviveFreshRepo(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	room := newDeliveryRoom(t, pool)

	writeRepo := db.NewMessageRepository(pool)
	m := postAgentMsg(t, writeRepo, room.ID, "a", "durable across restart")

	// A brand-new repository instance (simulating the process coming back up) can
	// still read the message and resume from its id.
	readRepo := db.NewMessageRepository(pool)
	got, err := readRepo.GetByID(ctx, room.ID, m.ID)
	if err != nil {
		t.Fatalf("read after fresh repo: %v", err)
	}
	if got.Content != "durable across restart" {
		t.Fatalf("expected durable content, got %q", got.Content)
	}
}

// --- Handler-level tests for the deep-link and pagination HTTP surface ---

func withSlugAndID(r *http.Request, slug, id string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	rctx.URLParams.Add("id", id)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

func newDeliveryHandler(pool *db.Pool) *RoomMessagesHandler {
	registry := hub.NewPresenceRegistry()
	hubMgr := hub.NewHubManager(context.Background(), registry, slog.Default(), 0)
	return NewRoomMessagesHandler(
		db.NewMessageRepository(pool),
		db.NewRoomRepository(pool),
		db.NewAgentPresenceRepository(pool),
		db.NewRoomEventRepository(pool),
		hubMgr,
	)
}

// Step 6: GET /v1/rooms/{slug}/messages/{id} fetches the correct message even when
// it would be outside the initial recent page, and never leaks across rooms.
func TestGetMessageHandler_FetchesAndScopes(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	otherRoom := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	target := postAgentMsg(t, msgRepo, room.ID, "a", "linked message")

	handler := newDeliveryHandler(pool)

	// Correct room + id -> 200 with the message.
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/rooms/%s/messages/%d", room.Slug, target.ID), nil)
	req = withSlugAndID(req, room.Slug, fmt.Sprintf("%d", target.ID))
	w := httptest.NewRecorder()
	handler.GetMessage(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	data, _ := resp["data"].(map[string]interface{})
	if data == nil || data["content"] != "linked message" {
		t.Fatalf("expected linked message content, got %v", resp["data"])
	}

	// Same id under a different room -> 404 (deep link cannot cross rooms).
	req2 := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/rooms/%s/messages/%d", otherRoom.Slug, target.ID), nil)
	req2 = withSlugAndID(req2, otherRoom.Slug, fmt.Sprintf("%d", target.ID))
	w2 := httptest.NewRecorder()
	handler.GetMessage(w2, req2)
	if w2.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for cross-room lookup, got %d", w2.Code)
	}
}

// Step 6: the list endpoint pages backwards with ?before=<id>.
func TestListMessagesHandler_BeforePagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	msgRepo := db.NewMessageRepository(pool)
	var ids []int64
	for i := 0; i < 5; i++ {
		ids = append(ids, postAgentMsg(t, msgRepo, room.ID, "a", fmt.Sprintf("m-%d", i)).ID)
	}

	handler := newDeliveryHandler(pool)
	req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/rooms/%s/messages?before=%d&limit=2", room.Slug, ids[3]), nil)
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()
	handler.ListMessages(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var resp struct {
		Data []models.Message `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if len(resp.Data) != 2 {
		t.Fatalf("expected 2 earlier messages, got %d", len(resp.Data))
	}
	// The two messages immediately before ids[3], ascending: ids[1], ids[2].
	if resp.Data[0].ID != ids[1] || resp.Data[1].ID != ids[2] {
		t.Fatalf("expected [%d,%d], got [%d,%d]", ids[1], ids[2], resp.Data[0].ID, resp.Data[1].ID)
	}
}

// Step 4: PostMessage accepts client_entry_id and the legacy client_message_id
// alias and creates a message (the field is wired through to storage).
func TestPostMessageHandler_AcceptsClientEntryFields(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	room := newDeliveryRoom(t, pool)
	handler := newDeliveryHandler(pool)

	post := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, "/r/"+room.Slug+"/message", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req = req.WithContext(context.WithValue(req.Context(), apimiddleware.RoomContextKey, room))
		w := httptest.NewRecorder()
		handler.PostMessage(w, req)
		return w
	}

	w := post(`{"agent_name":"planner","content":"canonical entry","client_entry_id":"ce-1"}`)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for client_entry_id, got %d: %s", w.Code, w.Body.String())
	}
	w2 := post(`{"agent_name":"planner","content":"legacy entry","client_message_id":"cm-1"}`)
	if w2.Code != http.StatusCreated {
		t.Fatalf("expected 201 for legacy client_message_id, got %d: %s", w2.Code, w2.Body.String())
	}
}
