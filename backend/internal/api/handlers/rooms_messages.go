package handlers

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// maxMessageContentLen is the maximum allowed content length for a message (64KB).
const maxMessageContentLen = 65536

// RoomMessagesHandler handles HTTP requests for room message operations.
// The presenceRepo field supports D-28 (implicit heartbeat on message posting).
type RoomMessagesHandler struct {
	msgRepo      *db.MessageRepository
	roomRepo     *db.RoomRepository
	presenceRepo *db.AgentPresenceRepository
	hubMgr       *hub.HubManager
}

// NewRoomMessagesHandler creates a new RoomMessagesHandler with all required dependencies.
// The presenceRepo is needed for D-28: message posting implicitly renews agent presence.
func NewRoomMessagesHandler(
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

// postMessageRequest is the JSON body for POST /r/{slug}/message.
type postMessageRequest struct {
	AgentName   string          `json:"agent_name"`
	Content     string          `json:"content"`
	ContentType string          `json:"content_type,omitempty"`
	Metadata    json.RawMessage `json:"metadata,omitempty"`
}

// PostMessage handles POST /r/{slug}/message.
// Requires bearer token authentication via BearerGuard middleware (D-17).
// Creates a message, increments room message count (D-30), updates room activity,
// renews agent presence (D-28 implicit heartbeat), and broadcasts to the hub.
func (h *RoomMessagesHandler) PostMessage(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "room context missing")
		return
	}

	var req postMessageRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	// Validate required fields
	if req.AgentName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "agent_name is required")
		return
	}
	if req.Content == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content is required")
		return
	}
	if len(req.Content) > maxMessageContentLen {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content exceeds maximum length of 65536 characters")
		return
	}

	// Default content_type to "text"
	contentType := req.ContentType
	if contentType == "" {
		contentType = "text"
	}
	if contentType != "text" && contentType != "markdown" && contentType != "json" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "content_type must be text, markdown, or json")
		return
	}

	params := models.CreateMessageParams{
		RoomID:      room.ID,
		AuthorType:  "agent",
		AgentName:   req.AgentName,
		Content:     req.Content,
		ContentType: contentType,
		Metadata:    req.Metadata,
	}

	msg, err := h.msgRepo.Create(r.Context(), params)
	if err != nil {
		slog.Error("failed to create message", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create message")
		return
	}

	// D-30: Increment message count on room
	if err := h.roomRepo.IncrementMessageCount(r.Context(), room.ID); err != nil {
		slog.Error("failed to increment message count", "error", err, "room_id", room.ID)
		// Non-fatal: continue even if count update fails
	}

	// Update room activity timestamp
	if err := h.roomRepo.UpdateActivity(r.Context(), room.ID); err != nil {
		slog.Error("failed to update room activity", "error", err, "room_id", room.ID)
		// Non-fatal
	}

	// D-28: Implicit heartbeat -- message posting renews agent presence
	if err := h.presenceRepo.UpdateHeartbeat(r.Context(), room.ID, req.AgentName); err != nil {
		slog.Error("failed to update heartbeat on message", "error", err, "room_id", room.ID, "agent", req.AgentName)
		// Non-fatal: presence will expire naturally if heartbeat fails
	}

	// Broadcast to hub for real-time subscribers
	roomHub := h.hubMgr.GetOrCreate(r.Context(), hub.NewRoomID(room.ID))
	roomHub.Broadcast(hub.RoomEvent{
		ID:        msg.ID,
		Type:      hub.EventMessage,
		RoomID:    hub.NewRoomID(room.ID),
		AgentName: msg.AgentName,
		Payload:   msg,
		Timestamp: msg.CreatedAt,
	})

	response := map[string]interface{}{
		"data": msg,
	}
	roomWriteJSON(w, http.StatusCreated, response)
}

// ListMessages handles GET /r/{slug}/messages and GET /v1/rooms/{slug}/messages.
// Supports cursor-based pagination per D-35: ?after=<message_id>&limit=<n>.
func (h *RoomMessagesHandler) ListMessages(w http.ResponseWriter, r *http.Request) {
	// Room can come from BearerGuard (A2A route) or direct lookup (public route)
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		// Public route: look up by slug
		slug := chi.URLParam(r, "slug")
		if slug == "" {
			roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
			return
		}
		var err error
		room, err = h.roomRepo.GetBySlug(r.Context(), slug)
		if err != nil {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
	}

	// Parse pagination params
	var afterID int64
	limit := 100
	if a := r.URL.Query().Get("after"); a != "" {
		if parsed, err := strconv.ParseInt(a, 10, 64); err == nil && parsed > 0 {
			afterID = parsed
		}
	}
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 500 {
			limit = parsed
		}
	}

	var messages []models.Message
	var err error
	if afterID > 0 {
		messages, err = h.msgRepo.ListAfter(r.Context(), room.ID, afterID, limit)
	} else {
		messages, err = h.msgRepo.ListRecent(r.Context(), room.ID, limit)
	}
	if err != nil {
		slog.Error("failed to list messages", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list messages")
		return
	}

	// Build cursor info
	var nextCursor *int64
	if len(messages) > 0 {
		lastID := messages[len(messages)-1].ID
		nextCursor = &lastID
	}

	_ = nextCursor // Available for future pagination header support

	response := map[string]interface{}{
		"data": messages,
	}
	roomWriteJSON(w, http.StatusOK, response)
}
