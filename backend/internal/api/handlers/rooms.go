package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// RoomHandler handles HTTP requests for room CRUD operations.
type RoomHandler struct {
	roomRepo     *db.RoomRepository
	msgRepo      *db.MessageRepository
	presenceRepo *db.AgentPresenceRepository
}

// NewRoomHandler creates a new RoomHandler with the required repositories.
func NewRoomHandler(roomRepo *db.RoomRepository, msgRepo *db.MessageRepository, presenceRepo *db.AgentPresenceRepository) *RoomHandler {
	return &RoomHandler{
		roomRepo:     roomRepo,
		msgRepo:      msgRepo,
		presenceRepo: presenceRepo,
	}
}

// createRoomRequest is the JSON body for POST /v1/rooms.
type createRoomRequest struct {
	DisplayName string   `json:"display_name"`
	Description *string  `json:"description,omitempty"`
	Category    *string  `json:"category,omitempty"`
	Tags        []string `json:"tags,omitempty"`
	Slug        string   `json:"slug,omitempty"`
	IsPrivate   bool     `json:"is_private"`
}

// CreateRoom handles POST /v1/rooms.
// Requires Solvr JWT or agent API key authentication.
// Returns the created room and the plaintext bearer token (shown only once).
func (h *RoomHandler) CreateRoom(w http.ResponseWriter, r *http.Request) {
	// Extract owner from Solvr JWT claims (Pitfall 2: NEVER use Quorum's mw.UserIDFromContext)
	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())

	var ownerID uuid.UUID
	if claims != nil {
		parsed, err := uuid.Parse(claims.UserID)
		if err != nil {
			roomWriteError(w, http.StatusBadRequest, "INVALID_USER_ID", "invalid user ID in token")
			return
		}
		ownerID = parsed
	} else if agent != nil {
		// Agent creating a room: use agent's human_id if linked, otherwise no owner
		if agent.HumanID != nil {
			parsed, err := uuid.Parse(*agent.HumanID)
			if err != nil {
				ownerID = uuid.Nil
			} else {
				ownerID = parsed
			}
		}
	} else {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	var req createRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	if req.DisplayName == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "display_name is required")
		return
	}

	params := models.CreateRoomParams{
		Slug:        req.Slug,
		DisplayName: req.DisplayName,
		Description: req.Description,
		Category:    req.Category,
		Tags:        req.Tags,
		IsPrivate:   req.IsPrivate,
		OwnerID:     ownerID,
	}

	room, plainToken, err := h.roomRepo.Create(r.Context(), params)
	if err != nil {
		if errors.Is(err, db.ErrRoomSlugExists) {
			roomWriteError(w, http.StatusConflict, "DUPLICATE_ROOM", "a room with this name already exists")
			return
		}
		slog.Error("failed to create room", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create room")
		return
	}

	// Token is returned ONCE at creation time (D-24). Never shown again in GET responses.
	response := map[string]interface{}{
		"data":  room,
		"token": plainToken,
	}
	roomWriteJSON(w, http.StatusCreated, response)
}

// GetRoom handles GET /v1/rooms/{slug}.
// Public endpoint, no authentication required (D-19).
// Returns room detail with agents and recent messages.
func (h *RoomHandler) GetRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room", "error", err, "slug", slug)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	// Fetch live agents
	agents, err := h.presenceRepo.ListByRoom(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to list presence", "error", err, "room_id", room.ID)
		agents = []models.AgentPresenceRecord{} // graceful degradation
	}

	// Fetch recent messages
	messages, err := h.msgRepo.ListRecent(r.Context(), room.ID, 50)
	if err != nil {
		slog.Error("failed to list messages", "error", err, "room_id", room.ID)
		messages = []models.Message{} // graceful degradation
	}

	response := map[string]interface{}{
		"data": map[string]interface{}{
			"room":            room,
			"agents":          agents,
			"recent_messages": messages,
		},
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// ListRooms handles GET /v1/rooms.
// Public endpoint, no authentication required (D-18).
// Returns a list of public rooms with stats.
func (h *RoomHandler) ListRooms(w http.ResponseWriter, r *http.Request) {
	limit := 20
	offset := 0

	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 && parsed <= 100 {
			limit = parsed
		}
	}
	if o := r.URL.Query().Get("offset"); o != "" {
		if parsed, err := strconv.Atoi(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	rooms, err := h.roomRepo.List(r.Context(), limit, offset)
	if err != nil {
		slog.Error("failed to list rooms", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list rooms")
		return
	}

	response := map[string]interface{}{
		"data": rooms,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// UpdateRoom handles PATCH /v1/rooms/{slug}.
// Requires authentication. Only the owner or admin can update (D-22).
// Slug is immutable after creation.
func (h *RoomHandler) UpdateRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for update", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	// Verify ownership or admin role
	if !isRoomOwnerOrAdmin(claims, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can update this room")
		return
	}

	var params models.UpdateRoomParams
	if err := json.NewDecoder(r.Body).Decode(&params); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	updated, err := h.roomRepo.Update(r.Context(), room.ID, params)
	if err != nil {
		slog.Error("failed to update room", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update room")
		return
	}

	response := map[string]interface{}{
		"data": updated,
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// DeleteRoom handles DELETE /v1/rooms/{slug}.
// Requires authentication. Only the owner or admin can delete (D-21).
func (h *RoomHandler) DeleteRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for delete", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	if !isRoomOwnerOrAdmin(claims, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can delete this room")
		return
	}

	if err := h.roomRepo.SoftDelete(r.Context(), room.ID); err != nil {
		slog.Error("failed to delete room", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete room")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// RotateToken handles POST /v1/rooms/{slug}/rotate-token.
// Requires authentication. Only the owner can rotate (D-25).
func (h *RoomHandler) RotateToken(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for token rotation", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	// Only owner can rotate token (D-25), not even admin
	if !isRoomOwner(claims, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner can rotate the token")
		return
	}

	newToken, err := h.roomRepo.RotateToken(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to rotate token", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to rotate token")
		return
	}

	response := map[string]interface{}{
		"data": map[string]string{
			"token": newToken,
		},
	}
	roomWriteJSON(w, http.StatusOK, response)
}

// isRoomOwnerOrAdmin checks if the authenticated user is the room owner or an admin.
func isRoomOwnerOrAdmin(claims *auth.Claims, room *models.Room) bool {
	if claims.Role == "admin" {
		return true
	}
	return isRoomOwner(claims, room)
}

// isRoomOwner checks if the authenticated user is the room owner.
func isRoomOwner(claims *auth.Claims, room *models.Room) bool {
	if room.OwnerID == nil {
		return false
	}
	ownerID, err := uuid.Parse(claims.UserID)
	if err != nil {
		return false
	}
	return *room.OwnerID == ownerID
}

// roomWriteJSON writes a JSON response with the given status code.
func roomWriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// roomWriteError writes a JSON error response.
func roomWriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
}
