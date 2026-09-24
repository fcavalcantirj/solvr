package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestGetRoom_ContextCarriesInitialTaskAndLatestPinned proves the room-detail
// response feeds the room page's compact context area (task 33, step 4): it
// carries the room's INITIAL TASK (the first message, even when it has fallen
// out of the recent window) and the LATEST PINNED DIRECTIVE (the most recently
// pinned message). The client renders these as-is — it never scans the transcript
// to decide which pin is latest or which message is first.
func TestGetRoom_ContextCarriesInitialTaskAndLatestPinned(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()

	slug := "room-context-" + uuid.NewString()[:8]
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM messages WHERE room_id IN (SELECT id FROM rooms WHERE slug=$1)`, slug)
		_, _ = pool.Exec(ctx, `DELETE FROM rooms WHERE slug=$1`, slug)
	})

	var roomID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO rooms (slug, display_name, is_private)
		VALUES ($1, 'Context Room', false)
		RETURNING id`, slug).Scan(&roomID)
	require.NoError(t, err)

	msgRepo := db.NewMessageRepository(pool)
	post := func(content string) *models.Message {
		t.Helper()
		m, err := msgRepo.Create(ctx, models.CreateMessageParams{
			RoomID:      roomID,
			AuthorType:  "agent",
			AgentName:   "planner",
			Content:     content,
			ContentType: "text",
		})
		require.NoError(t, err)
		return m
	}

	first := post("TASK: build tic-tac-toe")
	second := post("DIRECTIVE v1: start with the board")
	third := post("DIRECTIVE v2: start with input validation")

	h := NewRoomHandler(
		db.NewRoomRepository(pool),
		msgRepo,
		db.NewAgentPresenceRepository(pool),
		db.NewRoomMemberRepository(pool),
		db.NewRoomAgentTokenRepository(pool),
		db.NewRoomEventRepository(pool),
	)

	getContext := func(t *testing.T) (*models.Message, *models.Message) {
		t.Helper()
		req := httptest.NewRequest(http.MethodGet, "/v1/rooms/"+slug, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("slug", slug)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()
		h.GetRoom(w, req)
		require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
		var wrapper struct {
			Data struct {
				InitialTask  *models.Message `json:"initial_task"`
				LatestPinned *models.Message `json:"latest_pinned"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper), "body: %s", w.Body.String())
		return wrapper.Data.InitialTask, wrapper.Data.LatestPinned
	}

	// Before any pin: the initial task is the first message; there is no pinned
	// directive yet, so latest_pinned is null (not the first message, not an error).
	task, pinned := getContext(t)
	require.NotNil(t, task, "initial_task must always be the room's first message")
	require.Equal(t, first.ID, task.ID)
	require.Nil(t, pinned, "no message pinned yet -> latest_pinned is null")

	// Pin the v1 directive, then the v2 directive. ListPinned orders newest pin
	// first, so the LATEST pinned directive is v2 even though v1 was pinned first.
	_, err = msgRepo.Pin(ctx, roomID, second.ID)
	require.NoError(t, err)
	_, err = msgRepo.Pin(ctx, roomID, third.ID)
	require.NoError(t, err)

	task, pinned = getContext(t)
	require.NotNil(t, task)
	require.Equal(t, first.ID, task.ID, "initial task never changes as directives are pinned")
	require.NotNil(t, pinned, "a pinned directive must surface in the context area")
	require.Equal(t, third.ID, pinned.ID, "latest_pinned must be the most recently pinned message")
}
