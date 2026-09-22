package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// roomConnectEnvelope is the JSON response from GET /v1/rooms/{slug}/connect.
// It is the room-specific half of the connection contract: the executor prompt is
// already bound to the REAL room, with the real slug, the expected planner identity
// inferred from the first message, and the initial task.
type roomConnectEnvelope struct {
	InstructionVersion string `json:"instruction_version"`
	RoomSlug           string `json:"room_slug"`
	RoomURL            string `json:"room_url"`
	Private            bool   `json:"private"`
	Task               string `json:"task"`
	ExpectedPlanner    string `json:"expected_planner_identity"`
	ExecutorPrompt     string `json:"executor_prompt"`
	FirstMessageID     int64  `json:"first_message_id"`
	FirstMessageURL    string `json:"first_message_url"`
}

// firstMessageLookup is the slice of the message repository this handler needs.
type firstMessageLookup interface {
	GetFirstMessage(ctx context.Context, roomID uuid.UUID) (*models.Message, error)
}

// RoomConnectHandler serves GET /v1/rooms/{slug}/connect — the room-specific side
// of the connection contract. It hands a logged-out visitor the executor prompt that
// is already bound to the real room, plus the envelope of metadata that surface needs
// to render that prompt: the slug, the room URL, visibility, the initial task, the
// expected planner identity, and a link to the first message.
//
// The room itself is resolved and access-checked by RoomAccessGuard (readGuard) on
// public routes. For private rooms, anonymous callers are turned away at 403 before
// this handler runs. The handler ALSO checks IsPrivate as a defensive measure so the
// unit tests that call it without the middleware still see the 403.
type RoomConnectHandler struct {
	rooms connectRoomLookup
	msgs  firstMessageLookup
}

// NewRoomConnectHandler wires the handler to the room and message repositories.
func NewRoomConnectHandler(rooms connectRoomLookup, msgs firstMessageLookup) *RoomConnectHandler {
	return &RoomConnectHandler{rooms: rooms, msgs: msgs}
}

// GetRoomConnect handles GET /v1/rooms/{slug}/connect (public read, same policy as
// room detail). It resolves the room, enforces the private-room 403, looks up the
// first message to infer the planner identity and initial task, and returns the
// room-specific executor prompt envelope.
func (h *RoomConnectHandler) GetRoomConnect(w http.ResponseWriter, r *http.Request) {
	slug := roomConnectSlugFromRequest(r)
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	room, err := h.rooms.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) || strings.Contains(err.Error(), "not found") {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load room")
		return
	}
	if room == nil {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
		return
	}

	// Defensive check: RoomAccessGuard already enforces this on the wired route,
	// but the unit tests call this handler directly without the middleware.
	if room.IsPrivate {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "this room is closed to non-members")
		return
	}

	var firstMsg *models.Message
	firstMsg, err = h.msgs.GetFirstMessage(r.Context(), room.ID)
	if err != nil && !errors.Is(err, db.ErrRoomNotFound) {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load first message")
		return
	}

	envelope := buildRoomConnectEnvelope(room, firstMsg)
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": envelope})
}

// roomConnectSlugFromRequest extracts the room slug from the request. It prefers the
// chi URL parameter (set by the router) and falls back to parsing the URL path so the
// handler can be unit-tested without a chi router.
func roomConnectSlugFromRequest(r *http.Request) string {
	slug := chi.URLParam(r, "slug")
	if slug != "" {
		return slug
	}
	// Fallback: parse /v1/rooms/{slug}/connect from the path.
	parts := strings.Split(strings.Trim(r.URL.Path, "/"), "/")
	// Expected: ["v1", "rooms", "{slug}", "connect"]
	if len(parts) >= 4 && parts[0] == "v1" && parts[1] == "rooms" && parts[3] == "connect" {
		return parts[2]
	}
	return ""
}

// buildRoomConnectEnvelope assembles the response envelope from a room and its first
// message. When firstMsg is nil (the room has no messages yet), the prompt and fields
// degrade gracefully: the planner identity and task are absent, and the prompt still
// names the real room and tells the executor to read the first message before acting.
func buildRoomConnectEnvelope(room *models.Room, firstMsg *models.Message) roomConnectEnvelope {
	env := roomConnectEnvelope{
		InstructionVersion: "1.0",
		RoomSlug:           room.Slug,
		RoomURL:            connectAppBaseURL + "/rooms/" + room.Slug,
		Private:            room.IsPrivate,
	}

	if firstMsg != nil {
		env.ExpectedPlanner = firstMsg.AgentName
		env.Task = firstMsg.Content
		env.FirstMessageID = firstMsg.ID
		if firstMsg.SequenceNum != nil {
			env.FirstMessageURL = connectAppBaseURL + "/rooms/" + room.Slug + "#message-" + strconv.FormatInt(int64(*firstMsg.SequenceNum), 10)
		} else {
			env.FirstMessageURL = connectAppBaseURL + "/rooms/" + room.Slug + "#message-" + strconv.FormatInt(firstMsg.ID, 10)
		}
	}

	env.ExecutorPrompt = executorPromptText(room, firstMsg)
	return env
}
