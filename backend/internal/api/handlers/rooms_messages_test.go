package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// --- Mock repos for rooms_messages handler tests ---

// testRoomMsgRoom returns a minimal valid room for tests.
func testRoomMsgRoom() *models.Room {
	id, _ := uuid.NewRandom()
	return &models.Room{
		ID:           id,
		Slug:         "test-room-abc",
		DisplayName:  "Test Room",
		IsPrivate:    false,
		Tags:         []string{},
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		LastActiveAt: time.Now(),
	}
}

// newRoomMsgHandler creates a RoomMessagesHandler backed by nil repos.
// PostHumanMessage tests use newRoomMsgHandlerWithDeps below.
func newRoomMsgTestHandlerWithDeps(
	msgRepo *db.MessageRepository,
	roomRepo *db.RoomRepository,
	presenceRepo *db.AgentPresenceRepository,
	hubMgr *hub.HubManager,
) *RoomMessagesHandler {
	return &RoomMessagesHandler{
		msgRepo:      msgRepo,
		roomRepo:     roomRepo,
		presenceRepo: presenceRepo,
		hubMgr:       hubMgr,
	}
}

// addRoomMsgJWTCtx injects a Solvr JWT claims into the request context.
func addRoomMsgJWTCtx(r *http.Request, userID string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Email:  "human@example.com",
		Role:   "user",
	}
	return r.WithContext(auth.ContextWithClaims(r.Context(), claims))
}

// withSlug adds a chi URL parameter "slug" to the request context.
func withSlug(r *http.Request, slug string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("slug", slug)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}

// --- PostHumanMessage tests ---
// These tests use the humanMsgTestHandler which bypasses the concrete db repos
// by overriding the handler's behavior via the humanRoomRepo and humanMsgRepo
// test interfaces defined in rooms_messages.go.

func TestPostHumanMessage_Success(t *testing.T) {
	room := testRoomMsgRoom()
	handler := newTestHumanMsgHandler(room, nil, nil)

	body := `{"content": "Hello from human"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/"+room.Slug+"/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addRoomMsgJWTCtx(req, "user-123")
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid JSON response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["author_type"] != "human" {
		t.Errorf("expected author_type=human, got %v", data["author_type"])
	}
}

func TestPostHumanMessage_Unauthenticated(t *testing.T) {
	room := testRoomMsgRoom()
	handler := newTestHumanMsgHandler(room, nil, nil)

	body := `{"content": "Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/test-room/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	// No auth context added intentionally
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", w.Code)
	}
}

func TestPostHumanMessage_EmptyContent(t *testing.T) {
	room := testRoomMsgRoom()
	handler := newTestHumanMsgHandler(room, nil, nil)

	body := `{"content": ""}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/test-room/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addRoomMsgJWTCtx(req, "user-123")
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPostHumanMessage_ContentTooLong(t *testing.T) {
	room := testRoomMsgRoom()
	handler := newTestHumanMsgHandler(room, nil, nil)

	longContent := strings.Repeat("x", maxMessageContentLen+1)
	body, _ := json.Marshal(map[string]string{"content": longContent})
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/test-room/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addRoomMsgJWTCtx(req, "user-123")
	req = withSlug(req, room.Slug)
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestPostHumanMessage_RoomNotFound(t *testing.T) {
	handler := newTestHumanMsgHandlerNoRoom()

	body := `{"content": "Hello"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/rooms/nonexistent/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = addRoomMsgJWTCtx(req, "user-123")
	req = withSlug(req, "nonexistent")
	w := httptest.NewRecorder()

	handler.PostHumanMessage(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}

// --- Helpers to build handler with injectable deps for PostHumanMessage ---

// newTestHumanMsgHandler creates a RoomMessagesHandler with injected test deps.
// It uses the testable fields set directly on the struct (all package-internal).
func newTestHumanMsgHandler(room *models.Room, createErr error, _ error) *RoomMessagesHandler {
	return &RoomMessagesHandler{
		testRoomLookup: func(_ context.Context, _ string) (*models.Room, error) {
			return room, nil
		},
		testMsgCreate: func(_ context.Context, params models.CreateMessageParams) (*models.Message, error) {
			if createErr != nil {
				return nil, createErr
			}
			userID := ""
			if params.AuthorID != nil {
				userID = *params.AuthorID
			}
			return &models.Message{
				ID:          1,
				RoomID:      params.RoomID,
				AuthorType:  params.AuthorType,
				AuthorID:    &userID,
				AgentName:   params.AgentName,
				Content:     params.Content,
				ContentType: params.ContentType,
				CreatedAt:   time.Now(),
			}, nil
		},
	}
}

func newTestHumanMsgHandlerNoRoom() *RoomMessagesHandler {
	return &RoomMessagesHandler{
		testRoomLookup: func(_ context.Context, _ string) (*models.Room, error) {
			return nil, db.ErrRoomNotFound
		},
	}
}
