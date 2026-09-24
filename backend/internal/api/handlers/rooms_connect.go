package handlers

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// roomConnectEnvelope is the JSON response from GET /v1/rooms/{slug}/connect.
// It is the room-specific half of the connection contract: the prompt is
// already bound to the REAL room, with the real slug, the expected first participant identity
// inferred from the first message, and the initial task.
// The role parameter determines which prompt is returned (executor, reviewer, researcher, or custom).
type roomConnectEnvelope struct {
	InstructionVersion string `json:"instruction_version"`
	RoomSlug           string `json:"room_slug"`
	RoomURL            string `json:"room_url"`
	Private            bool   `json:"private"`
	Task               string `json:"task"`
	ExpectedPlanner    string `json:"expected_planner_identity"`
	ExecutorPrompt     string `json:"executor_prompt"` // Backward compatible
	Prompt             string `json:"prompt"`          // New generic prompt field
	Role               string `json:"role"`
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
// The room itself is resolved and access-checked by RoomAccessGuard (readGuard).
// For private rooms, non-members are turned away at 403 before this handler runs;
// members (the owner included) get the private join prompt, whose handshake step
// replaces sharing the room token. The handler ALSO refuses a private room that the
// guard did not admit, so calling it without the middleware still yields 403.
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
// first message to infer the first participant identity and initial task, and returns the
// room-specific join prompt envelope. The ?role= query parameter selects which role-specific
// prompt is returned (executor, reviewer, researcher, or custom role label). Defaults to executor.
func (h *RoomConnectHandler) GetRoomConnect(w http.ResponseWriter, r *http.Request) {
	slug := roomConnectSlugFromRequest(r)
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	// Extract role from query parameter (default: executor)
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "executor"
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

	// A private room's prompt is served only to a caller RoomAccessGuard admitted
	// (it injects the room it checked); the owner uses it to connect agents without
	// handing out the shared room token. Without that proof (e.g. the handler called
	// with no guard) a private room stays 403.
	if room.IsPrivate && !guardAdmitted(r, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "this room is closed to non-members")
		return
	}

	var firstMsg *models.Message
	firstMsg, err = h.msgs.GetFirstMessage(r.Context(), room.ID)
	if err != nil && !errors.Is(err, db.ErrRoomNotFound) {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load first message")
		return
	}

	envelope := buildRoomConnectEnvelope(room, firstMsg, role)
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": envelope})
}

// guardAdmitted reports whether RoomAccessGuard admitted this request for room.
func guardAdmitted(r *http.Request, room *models.Room) bool {
	checked := middleware.RoomFromContext(r.Context())
	return checked != nil && checked.ID == room.ID
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
// names the real room and tells the agent to read the first message before acting.
// The role parameter determines which role-specific prompt is generated.
func buildRoomConnectEnvelope(room *models.Room, firstMsg *models.Message, role string) roomConnectEnvelope {
	env := roomConnectEnvelope{
		InstructionVersion: ConnectInstructionVersion,
		RoomSlug:           room.Slug,
		RoomURL:            connectAppBaseURL + "/rooms/" + room.Slug,
		Private:            room.IsPrivate,
		Role:               role,
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

	// Generate role-specific prompt
	if role == "executor" {
		prompt := executorPromptText(room, firstMsg)
		env.Prompt = prompt
		env.ExecutorPrompt = prompt
	} else {
		env.Prompt = roleSpecificPromptText(room, firstMsg, role)
	}
	return env
}
