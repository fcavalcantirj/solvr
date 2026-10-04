package handlers

import (
	"context"
	"errors"
	"net/http"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// roomConnectEnvelope is the JSON response from GET /v1/rooms/{slug}/connect: the
// sentence for an agent joining this real room in a role, in the same shape as the first
// agent's (connect_slim.go), plus what a page shows beside it.
type roomConnectEnvelope struct {
	InstructionVersion string     `json:"instruction_version"`
	RoomSlug           string     `json:"room_slug"`
	RoomURL            string     `json:"room_url"`
	Private            bool       `json:"private"`
	Role               string     `json:"role"`
	Task               string     `json:"task"`
	Prompt             SlimPrompt `json:"prompt"`
	// CurrentDirective is the directive in force (idx 92): the newest pin followed to its
	// latest revision. Omitted when the room has none. The skill tells a joining agent to
	// follow the room's latest_pinned, so the sentence itself never repeats it.
	CurrentDirective *roomConnectDirective `json:"current_directive,omitempty"`
}

// firstMessageLookup is the slice of the message repository this handler needs.
type firstMessageLookup interface {
	GetFirstMessage(ctx context.Context, roomID uuid.UUID) (*models.Message, error)
}

// RoomConnectHandler serves GET /v1/rooms/{slug}/connect — the room-specific side of
// the connection contract. It hands a logged-out visitor the sentence for an agent
// joining the real room, plus what a page shows beside it: the slug, the room URL,
// visibility, the role and the room's task.
//
// The room itself is resolved and access-checked by RoomAccessGuard (readGuard).
// For private rooms, non-members are turned away at 403 before this handler runs;
// members (the owner included) get the private join prompt, whose handshake step
// replaces sharing the room token. The handler ALSO refuses a private room that the
// guard did not admit, so calling it without the middleware still yields 403.
type RoomConnectHandler struct {
	rooms      connectRoomLookup
	msgs       firstMessageLookup
	directives directiveLookup
}

// NewRoomConnectHandler wires the handler to the room and message repositories.
func NewRoomConnectHandler(rooms connectRoomLookup, msgs firstMessageLookup) *RoomConnectHandler {
	return &RoomConnectHandler{rooms: rooms, msgs: msgs}
}

// GetRoomConnect handles GET /v1/rooms/{slug}/connect (public read, same policy as
// room detail). It resolves the room, enforces the private-room 403, reads the first
// message as the room's task, and returns the joining agent's sentence for ?role=
// (executor by default; reviewer, expert, learner, planner, builder, collaborator, or
// any short custom label).
func (h *RoomConnectHandler) GetRoomConnect(w http.ResponseWriter, r *http.Request) {
	slug := roomConnectSlugFromRequest(r)
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	// ?role= names the joining agent's role (default: executor). It is shown in capitals
	// inside the sentence, so only a short lowercase label is accepted.
	role := r.URL.Query().Get("role")
	if role == "" {
		role = "executor"
	}
	if !validRoomRole(role) {
		roomWriteError(w, http.StatusBadRequest, "INVALID_ROLE",
			"role must be a short lowercase label, such as executor, reviewer, expert, planner or collaborator")
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
	h.addCurrentDirective(r.Context(), room, &envelope)
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
// message (the room's task). A room with no messages yet still has its sentence: the
// title and the link name the room.
func buildRoomConnectEnvelope(room *models.Room, firstMsg *models.Message, role string) roomConnectEnvelope {
	env := roomConnectEnvelope{
		InstructionVersion: ConnectInstructionVersion,
		RoomSlug:           room.Slug,
		RoomURL:            connectAppBaseURL + "/rooms/" + room.Slug,
		Private:            room.IsPrivate,
		Role:               role,
		Prompt:             slimRoomPrompt(room, role),
	}
	if firstMsg != nil {
		env.Task = firstMsg.Content
	}
	return env
}
