package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// executorPromptTestRoom builds a minimal room + first message for unit tests.
func executorPromptTestRoom() (*models.Room, *models.Message) {
	room := &models.Room{
		ID:          uuid.New(),
		Slug:        "tic-tac-toe-planner-exec",
		DisplayName: "Tic-Tac-Toe Planner/Executor",
		IsPrivate:   false,
	}
	seq := 1
	msg := &models.Message{
		ID:          1,
		RoomID:      room.ID,
		AgentName:   "raphael_tictactoe_planner",
		Content:     "Task: Build a tic-tac-toe AI that plays optimally. Directive: implement the minimax algorithm.",
		SequenceNum: &seq,
	}
	return room, msg
}

// TestRoomsConnect_ExecutorPromptNamesActualRoom verifies the executor prompt names
// the actual room (not the ROOM_SLUG placeholder), the exec role, the expected
// planner identity, the initial task, and the production endpoints the executor must
// call — and that it never leaks credentials, never uses shell-template syntax, and
// never names a localhost endpoint.
func TestRoomsConnect_ExecutorPromptNamesActualRoom(t *testing.T) {
	room, firstMsg := executorPromptTestRoom()

	prompt := executorPromptText(room, firstMsg)

	lower := strings.ToLower(prompt)

	// Step 2: names the actual room, executor role, expected planner identity,
	// initial task.
	require.Contains(t, prompt, connectAppBaseURL+"/rooms/"+room.Slug,
		"the prompt must name the REAL room URL, not a placeholder")
	require.NotContains(t, prompt, "ROOM_SLUG",
		"the room-specific prompt must not contain the placeholder")
	require.Contains(t, lower, "executor")
	require.Contains(t, lower, "planner")
	require.Contains(t, lower, firstMsg.AgentName,
		"the prompt must name the expected planner identity (the first author)")
	require.Contains(t, lower, "minimax",
		"the prompt must carry the initial task from the first message")
	require.Contains(t, lower, "retrieve the latest directive")

	// Step 3: names the real production endpoints (with the real slug substituted).
	require.Contains(t, prompt, connectAPIBaseURL+"/v1/rooms/"+room.Slug+"/handshake")
	require.Contains(t, prompt, connectAPIBaseURL+"/r/"+room.Slug+"/join")
	require.Contains(t, prompt, connectAPIBaseURL+"/r/"+room.Slug+"/message")
	require.Contains(t, prompt, connectAPIBaseURL+"/r/"+room.Slug+"/messages")
	require.Contains(t, lower, "your own identity")

	// No credentials may appear in the executor prompt.
	require.NotContains(t, lower, "solvr_sk_")
	require.NotContains(t, lower, "solvr_rt_")
	require.NotContains(t, lower, "api_key")
	require.NotContains(t, lower, "room_token")

	// No shell/template substitution.
	require.NotContains(t, prompt, "$")
	require.NotContains(t, prompt, "${")
	require.NotContains(t, prompt, "`")

	// Production endpoints only — no localhost.
	require.NotContains(t, prompt, "http://localhost")

	// The prompt must tell the executor to report failure as failure.
	require.Contains(t, lower, "never invent")
}

// TestRoomsConnect_ExecutorPromptNeverLeaksCredentials covers the same credential
// guarantees as a standalone assertion list so a regression in any source field is
// caught explicitly.
func TestRoomsConnect_ExecutorPromptNeverLeaksCredentials(t *testing.T) {
	room, firstMsg := executorPromptTestRoom()
	prompt := executorPromptText(room, firstMsg)

	for _, secret := range []string{"solvr_sk_", "solvr_rt_", "solvr_rm_", "api_key", "room_token"} {
		require.NotContains(t, prompt, secret,
			"executor prompt must not leak %q", secret)
	}
}

// TestRoomsConnect_PrivateRoomExecutorPromptMentionsPrivateContext verifies the
// prompt for a private room explains the shared room token context without exposing
// the token itself.
func TestRoomsConnect_PrivateRoomExecutorPromptMentionsPrivateContext(t *testing.T) {
	room := &models.Room{
		ID:        uuid.New(),
		Slug:      "private-room-slug",
		IsPrivate: true,
	}
	seq := 1
	msg := &models.Message{
		ID:          1,
		RoomID:      room.ID,
		AgentName:   "the_planner",
		Content:     "Task: secure collaboration. Directive: lock it down.",
		SequenceNum: &seq,
	}

	prompt := executorPromptText(room, msg)

	require.Contains(t, prompt, room.Slug)
	require.Contains(t, strings.ToLower(prompt), "private")
	require.NotContains(t, prompt, "ROOM_SLUG")
	require.NotContains(t, prompt, "$")
	require.NotContains(t, prompt, "${")
}

// TestRoomsConnect_ExecutorPromptForNoFirstMessage degrades gracefully when the room
// has no messages yet: the prompt must still be valid and contain no placeholder.
func TestRoomsConnect_ExecutorPromptForNoFirstMessage(t *testing.T) {
	room := &models.Room{
		ID:        uuid.New(),
		Slug:      "empty-room-123",
		IsPrivate: false,
	}

	prompt := executorPromptText(room, nil)

	// Even without a first message the prompt names the real room and the executor role.
	require.Contains(t, prompt, connectAppBaseURL+"/rooms/"+room.Slug)
	require.NotContains(t, prompt, "ROOM_SLUG")
	require.Contains(t, strings.ToLower(prompt), "executor")
	require.NotContains(t, prompt, "$")
	require.NotContains(t, prompt, "${")
}

// fakeRoomConnectRooms is a connectRoomLookup fake for unit tests.
type fakeRoomConnectRooms struct {
	room  *models.Room
	err   error
	asked string
}

func (f *fakeRoomConnectRooms) GetBySlug(_ context.Context, slug string) (*models.Room, error) {
	f.asked = slug
	if f.err != nil {
		return nil, f.err
	}
	return f.room, nil
}

// fakeFirstMessageLookup is a firstMessageLookup fake for unit tests.
type fakeFirstMessageLookup struct {
	msg *models.Message
	err error
}

func (f *fakeFirstMessageLookup) GetFirstMessage(_ context.Context, _ uuid.UUID) (*models.Message, error) {
	return f.msg, f.err
}

// TestRoomsConnect_HandlerServesRoomConnectEnvelope verifies the handler returns the
// JSON envelope with the executor prompt, room URL, task, and visibility — using
// fakes so no database is needed.
func TestRoomsConnect_HandlerServesRoomConnectEnvelope(t *testing.T) {
	room, firstMsg := executorPromptTestRoom()

	h := &RoomConnectHandler{
		rooms: &fakeRoomConnectRooms{room: room},
		msgs:  &fakeFirstMessageLookup{msg: firstMsg},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/"+room.Slug+"/connect", nil)
	w := httptest.NewRecorder()
	h.GetRoomConnect(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())

	var wrapper struct {
		Data roomConnectEnvelope `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper), "body: %s", w.Body.String())
	resp := wrapper.Data

	require.Equal(t, "1.0", resp.InstructionVersion)
	require.Equal(t, room.Slug, resp.RoomSlug)
	require.Equal(t, connectAppBaseURL+"/rooms/"+room.Slug, resp.RoomURL)
	require.False(t, resp.Private)
	require.Contains(t, resp.ExecutorPrompt, room.Slug)
	require.NotEmpty(t, resp.ExpectedPlanner)
	require.Equal(t, firstMsg.AgentName, resp.ExpectedPlanner)
	require.NotEmpty(t, resp.Task)
	require.Contains(t, resp.Task, "minimax")
	require.Equal(t, firstMsg.ID, resp.FirstMessageID)
	require.NotEmpty(t, resp.FirstMessageURL)

	// No credentials in the raw body.
	lower := strings.ToLower(w.Body.String())
	for _, secret := range []string{"solvr_sk_", "solvr_rt_", "api_key", "room_token"} {
		require.NotContains(t, lower, secret, "leaked %q", secret)
	}
}

// TestRoomsConnect_Handler404ForNonexistentRoom verifies the handler returns 404
// when the room does not exist.
func TestRoomsConnect_Handler404ForNonexistentRoom(t *testing.T) {
	h := &RoomConnectHandler{
		rooms: &fakeRoomConnectRooms{err: errRoomNotFound},
		msgs:  &fakeFirstMessageLookup{},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/missing-room/connect", nil)
	w := httptest.NewRecorder()
	h.GetRoomConnect(w, req)

	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "NOT_FOUND")
}

// TestRoomsConnect_Handler403ForPrivateRoomWhenUnauthenticated verifies that a
// private room returns 403 to an anonymous caller (same policy as room reads).
func TestRoomsConnect_Handler403ForPrivateRoomWhenUnauthenticated(t *testing.T) {
	privateRoom := &models.Room{
		ID:        uuid.New(),
		Slug:      "private-test-room",
		IsPrivate: true,
	}
	h := &RoomConnectHandler{
		rooms: &fakeRoomConnectRooms{room: privateRoom},
		msgs:  &fakeFirstMessageLookup{},
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/private-test-room/connect", nil)
	w := httptest.NewRecorder()
	h.GetRoomConnect(w, req)

	require.Equal(t, http.StatusForbidden, w.Code)
}

// TestRoomsConnect_RoleParameterGeneratesRoleSpecificPrompt verifies that different
// roles (reviewer, researcher, executor) generate appropriate role-specific join prompts.
func TestRoomsConnect_RoleParameterGeneratesRoleSpecificPrompt(t *testing.T) {
	room, firstMsg := executorPromptTestRoom()

	for _, tc := range []struct {
		name string
		role string
		want string
	}{
		{"executor", "executor", "executor"},
		{"reviewer", "reviewer", "reviewer"},
		{"researcher", "researcher", "researcher"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			prompt := roleSpecificPromptText(room, firstMsg, tc.role)
			require.Contains(t, strings.ToLower(prompt), tc.want,
				"prompt for role %q must mention the role", tc.role)
			require.Contains(t, prompt, connectAppBaseURL+"/rooms/"+room.Slug)
			require.NotContains(t, prompt, "ROOM_SLUG")
			require.NotContains(t, prompt, "$")
			require.NotContains(t, prompt, "${")
			// Credentials must never appear.
			require.NotContains(t, prompt, "solvr_sk_")
			require.NotContains(t, prompt, "solvr_rt_")
		})
	}
}

// TestRoomsConnect_CollaboratorRecruitsIntoExistingRoom verifies the public-room
// recruit contract (task 26): the default collaborator join prompt tells a NEW
// agent to self-register and take its OWN room token by handshake, then join the
// EXISTING room — it never creates a duplicate room and never grants owner rights.
func TestRoomsConnect_CollaboratorRecruitsIntoExistingRoom(t *testing.T) {
	room, firstMsg := executorPromptTestRoom()

	prompt := roleSpecificPromptText(room, firstMsg, "collaborator")

	// Names the role and the REAL room, never a placeholder slug.
	require.Contains(t, prompt, "Collaborator agent joining an existing Solvr room")
	require.Contains(t, prompt, connectAppBaseURL+"/rooms/"+room.Slug)
	require.NotContains(t, prompt, "ROOM_SLUG")

	// Self-registers if needed and takes its OWN per-agent room token by handshake.
	require.Contains(t, prompt, connectAPIBaseURL+"/v1/agents/register")
	require.Contains(t, prompt, "/rooms/"+room.Slug+"/handshake")

	// Never creates a duplicate room and never claims owner permissions.
	require.NotContains(t, prompt, "Create the room")
	require.NotContains(t, prompt, "you will own")

	// Never leaks credentials or impersonates another agent.
	require.NotContains(t, prompt, "solvr_sk_")
	require.NotContains(t, prompt, "solvr_rt_")
	require.Contains(t, prompt, "never impersonate")

	// The handler echoes the collaborator role and returns the same prompt for a
	// public room to an anonymous caller.
	h := &RoomConnectHandler{
		rooms: &fakeRoomConnectRooms{room: room},
		msgs:  &fakeFirstMessageLookup{msg: firstMsg},
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/"+room.Slug+"/connect?role=collaborator", nil)
	w := httptest.NewRecorder()
	h.GetRoomConnect(w, req)

	require.Equal(t, http.StatusOK, w.Code, "body: %s", w.Body.String())
	var wrapper struct {
		Data roomConnectEnvelope `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	require.Equal(t, "collaborator", wrapper.Data.Role)
	require.Contains(t, wrapper.Data.Prompt, "Collaborator agent joining an existing Solvr room")
}

// errRoomNotFound is returned by fakes to simulate a missing room.
var errRoomNotFound = db.ErrRoomNotFound
