package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/rooms/{slug}/connect serves the sentence for an agent joining a real room, in
// the same shape as the first agent's: the skill, the room (its title is the intent and
// its link the place), the role in capitals, and the job.

func connectTestRoom(private bool) (*models.Room, *models.Message) {
	room := &models.Room{ID: uuid.New(), Slug: "tic-tac-toe-planner-exec", DisplayName: "Ship the signup page", IsPrivate: private}
	seq := 1
	msg := &models.Message{ID: 1, RoomID: room.ID, AgentName: "raphael_planner", SequenceNum: &seq,
		Content: "Task: Build a tic-tac-toe AI that plays optimally. Directive: implement the minimax algorithm."}
	return room, msg
}

type fakeRoomConnectRooms struct {
	room *models.Room
	err  error
}

func (f *fakeRoomConnectRooms) GetBySlug(_ context.Context, _ string) (*models.Room, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.room, nil
}

type fakeFirstMessageLookup struct {
	msg *models.Message
	err error
}

func (f *fakeFirstMessageLookup) GetFirstMessage(_ context.Context, _ uuid.UUID) (*models.Message, error) {
	return f.msg, f.err
}

// serveRoomConnect calls the handler; admitted, when set, is the room RoomAccessGuard let in.
func serveRoomConnect(room *models.Room, msg *models.Message, query string, admitted *models.Room) *httptest.ResponseRecorder {
	h := &RoomConnectHandler{rooms: &fakeRoomConnectRooms{room: room}, msgs: &fakeFirstMessageLookup{msg: msg}}
	req := httptest.NewRequest(http.MethodGet, "/v1/rooms/"+room.Slug+"/connect"+query, nil)
	if admitted != nil {
		req = req.WithContext(context.WithValue(req.Context(), middleware.RoomContextKey, admitted))
	}
	w := httptest.NewRecorder()
	h.GetRoomConnect(w, req)
	return w
}

func roomEnvelope(t *testing.T, w *httptest.ResponseRecorder) roomConnectEnvelope {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var wrapper struct {
		Data roomConnectEnvelope `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper), w.Body.String())
	return wrapper.Data
}

func TestRoomsConnect_ServesTheJoiningAgentsSentence(t *testing.T) {
	room, msg := connectTestRoom(false)
	w := serveRoomConnect(room, msg, "", nil)
	env := roomEnvelope(t, w)

	require.Equal(t, ConnectInstructionVersion, env.InstructionVersion)
	require.Equal(t, room.Slug, env.RoomSlug)
	require.Equal(t, "https://solvr.dev/rooms/"+room.Slug, env.RoomURL)
	require.False(t, env.Private)
	require.Equal(t, "executor", env.Role, "executor is the default role")
	require.Equal(t, msg.Content, env.Task, "the room's first message is the task")
	require.Equal(t,
		`Learn Solvr from https://solvr.dev/skill.md. Join the public Solvr room "Ship the signup page" at https://solvr.dev/rooms/tic-tac-toe-planner-exec as the EXECUTOR, read it, and follow the orders pinned there, post your doubts, and post a summary when you're done.`,
		env.Prompt.Text)
	require.Equal(t, env.Prompt.Text, joined(env.Prompt))
	require.Less(t, env.Prompt.WordCount, 120)
	require.Equal(t, []PromptSegment{{Kind: SegmentRole, Text: "EXECUTOR", Side: "b"}}, segmentsOf(env.Prompt, SegmentRole))
	require.Equal(t, "Ship the signup page", segmentsOf(env.Prompt, SegmentIntent)[0].Text)
	links := segmentsOf(env.Prompt, SegmentLink)
	require.Len(t, links, 2)
	require.Equal(t, "https://solvr.dev/skill.md", links[0].Text)
	require.Equal(t, "https://solvr.dev/rooms/"+room.Slug, links[1].Text)

	body := strings.ToLower(w.Body.String())
	for _, gone := range []string{`"executor_prompt"`, `"expected_planner_identity"`, `"first_message_id"`, "api.solvr.dev",
		"solvr_sk_", "solvr_rt_", "api_key", "room_token"} {
		require.NotContains(t, body, gone)
	}
}

func TestRoomsConnect_EachRoleGetsItsJob(t *testing.T) {
	room, msg := connectTestRoom(false)
	for role, job := range map[string]string{
		"reviewer":     "review and test each change posted there, and approve or reject it",
		"expert":       "answer everything you're asked there until the asker can work on it alone",
		"planner":      "post the plan there, pin it as the directive, and direct the work",
		"collaborator": "help with the work pinned there, and post what you did",
		"researcher":   "do that job there, and post what you did",
	} {
		env := roomEnvelope(t, serveRoomConnect(room, msg, "?role="+role, nil))
		require.Equal(t, role, env.Role)
		require.Contains(t, env.Prompt.Text, " as the "+strings.ToUpper(role)+", read it, and "+job+".", role)
	}
}

func TestRoomsConnect_RefusesARoleThatCannotBePutInASentence(t *testing.T) {
	room, msg := connectTestRoom(false)
	for _, role := range []string{"Reviewer", "a", "rev%20iewer", "x--------------------------y", "rm$x"} {
		w := serveRoomConnect(room, msg, "?role="+role, nil)
		require.Equal(t, http.StatusBadRequest, w.Code, role)
		require.Contains(t, w.Body.String(), "INVALID_ROLE", role)
	}
}

func TestRoomsConnect_APrivateRoomsSentenceAsksForTheAgentsIDFirst(t *testing.T) {
	room, msg := connectTestRoom(true)
	env := roomEnvelope(t, serveRoomConnect(room, msg, "?role=collaborator", room))
	require.True(t, env.Private)
	require.Contains(t, env.Prompt.Text, "Join the private Solvr room")
	require.True(t, strings.HasSuffix(env.Prompt.Text, " It's private, so give me your agent id first and I'll get you admitted."))

	other := &models.Room{ID: uuid.New(), Slug: "other-room", IsPrivate: true}
	require.Equal(t, http.StatusForbidden, serveRoomConnect(room, msg, "", other).Code, "admitted to another room only")
	require.Equal(t, http.StatusForbidden, serveRoomConnect(room, msg, "", nil).Code, "no guard, no private sentence")
}

func TestRoomsConnect_ARoomWithNoMessagesStillHasItsSentence(t *testing.T) {
	room, _ := connectTestRoom(false)
	env := roomEnvelope(t, serveRoomConnect(room, nil, "", nil))
	require.Empty(t, env.Task)
	require.Contains(t, env.Prompt.Text, "https://solvr.dev/rooms/"+room.Slug)
}

func TestRoomsConnect_Handler404ForNonexistentRoom(t *testing.T) {
	h := &RoomConnectHandler{rooms: &fakeRoomConnectRooms{err: db.ErrRoomNotFound}, msgs: &fakeFirstMessageLookup{}}
	w := httptest.NewRecorder()
	h.GetRoomConnect(w, httptest.NewRequest(http.MethodGet, "/v1/rooms/missing-room/connect", nil))
	require.Equal(t, http.StatusNotFound, w.Code)
	require.Contains(t, w.Body.String(), "NOT_FOUND")
}
