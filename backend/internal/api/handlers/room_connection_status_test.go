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

// --- Pure unit tests: the connection-progress state machine (always run, no DB) ---

// TestComputeConnectionStatus_StateMachine pins the server-side derivation of a
// room's connection progress from real activity: the sticky two-way activation
// milestone plus the count of server-confirmed, unexpired agent presence.
func TestComputeConnectionStatus_StateMachine(t *testing.T) {
	cases := []struct {
		name        string
		activated   bool
		onlineCount int
		want        string
	}{
		// Step 1: a brand-new room with nobody present is Waiting for agents.
		{"no agents, not activated -> waiting for agents", false, 0, ConnectionStatusWaitingForAgents},
		// Step 2: the first agent joins -> Waiting for another agent.
		{"one agent online, not activated -> waiting for another", false, 1, ConnectionStatusWaitingForAnotherAgent},
		// Step 3/4: presence alone (even two present) is not the milestone; the room
		// stays "waiting for another agent" until a real two-way exchange happens.
		{"two agents online, not activated -> still waiting for the exchange", false, 2, ConnectionStatusWaitingForAnotherAgent},
		// Step 4: the two-way exchange milestone -> Conversation started.
		{"activated with a live exchange -> conversation started", true, 2, ConnectionStatusConversationStarted},
		// Step 5: the milestone is sticky — presence expiring back to zero must NOT
		// reset the room to a waiting state.
		{"activated but everyone left -> still conversation started", true, 0, ConnectionStatusConversationStarted},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			require.Equal(t, tc.want, ComputeConnectionStatus(tc.activated, tc.onlineCount))
		})
	}
}

// TestComputeConnectionStatus_NegativeOnlineCountIsWaiting is a defensive guard:
// a nonsensical negative count never marks a room connected (step 6 spirit).
func TestComputeConnectionStatus_NegativeOnlineCountIsWaiting(t *testing.T) {
	require.Equal(t, ConnectionStatusWaitingForAgents, ComputeConnectionStatus(false, -1))
}

// --- Integration test: GET /v1/rooms/{slug} carries the derived status end to end ---

// TestGetRoom_ConnectionStatusReflectsRealActivity walks a room through the whole
// progression against a real database and verifies the room-detail response
// reports connection_status and online_count from actual presence + the persisted
// activation milestone — never from prompt copies, registrations, or SSE viewers.
func TestGetRoom_ConnectionStatusReflectsRealActivity(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()

	slug := "conn-status-" + uuid.NewString()[:8]
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM room_events WHERE room_id IN (SELECT id FROM rooms WHERE slug=$1)`, slug)
		_, _ = pool.Exec(ctx, `DELETE FROM agent_presence WHERE room_id IN (SELECT id FROM rooms WHERE slug=$1)`, slug)
		_, _ = pool.Exec(ctx, `DELETE FROM rooms WHERE slug=$1`, slug)
	})

	var roomID uuid.UUID
	err := pool.QueryRow(ctx, `
		INSERT INTO rooms (slug, display_name, is_private)
		VALUES ($1, 'Conn Status Room', false)
		RETURNING id`, slug).Scan(&roomID)
	require.NoError(t, err)

	h := NewRoomHandler(
		db.NewRoomRepository(pool),
		db.NewMessageRepository(pool),
		db.NewAgentPresenceRepository(pool),
		db.NewRoomMemberRepository(pool),
		db.NewRoomAgentTokenRepository(pool),
		db.NewRoomEventRepository(pool),
	)

	getStatus := func(t *testing.T) (string, float64) {
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
				ConnectionStatus string  `json:"connection_status"`
				OnlineCount      float64 `json:"online_count"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper), "body: %s", w.Body.String())
		return wrapper.Data.ConnectionStatus, wrapper.Data.OnlineCount
	}

	// Step 1 + step 6: a fresh room with no presence and no exchange is Waiting for
	// agents. Copying a prompt, registering an agent, or opening a browser SSE stream
	// create neither presence nor messages, so none of them can advance this.
	status, online := getStatus(t)
	require.Equal(t, ConnectionStatusWaitingForAgents, status)
	require.Equal(t, float64(0), online)

	// Step 2: one agent present -> Waiting for another agent, online count = 1.
	presenceRepo := db.NewAgentPresenceRepository(pool)
	_, err = presenceRepo.Upsert(ctx, models.UpsertAgentPresenceParams{
		RoomID:     roomID,
		AgentName:  "planner-agent",
		CardJSON:   json.RawMessage(`{}`),
		TTLSeconds: 300,
	})
	require.NoError(t, err)
	status, online = getStatus(t)
	require.Equal(t, ConnectionStatusWaitingForAnotherAgent, status)
	require.Equal(t, float64(1), online)

	// Step 4: record the persisted two-way activation milestone -> Conversation started.
	eventRepo := db.NewRoomEventRepository(pool)
	_, err = eventRepo.Create(ctx, models.CreateRoomEventParams{
		RoomID:    roomID,
		EventType: db.RoomActivationEventType,
		Actor:     "system",
	})
	require.NoError(t, err)
	status, _ = getStatus(t)
	require.Equal(t, ConnectionStatusConversationStarted, status)

	// Step 5: every agent's presence expires -> the milestone is sticky and the room
	// stays Conversation started; the online count drops to zero honestly.
	_, err = pool.Exec(ctx, `DELETE FROM agent_presence WHERE room_id=$1`, roomID)
	require.NoError(t, err)
	status, online = getStatus(t)
	require.Equal(t, ConnectionStatusConversationStarted, status, "activation milestone must not reset when presence expires")
	require.Equal(t, float64(0), online)
}
