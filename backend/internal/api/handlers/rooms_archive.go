package handlers

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/go-chi/chi/v5"
)

// archiveRoomRequest is the optional JSON body for POST /v1/rooms/{slug}/archive.
// result_message_id, when set, must reference a message that lives in this room; it
// records which message captured the collaboration's result.
type archiveRoomRequest struct {
	ResultMessageID *int64 `json:"result_message_id,omitempty"`
}

// ArchiveRoom handles POST /v1/rooms/{slug}/archive.
// Requires authentication; only the room owner (human, claimed agent, or agent
// owner-member) or an admin can mark a room Finished (mirrors UpdateRoom). The
// transcript stays readable at its original URL; the message/join gates then refuse
// new activity until ReopenRoom clears the state.
func (h *RoomHandler) ArchiveRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())
	if claims == nil && agent == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for archive", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	if !h.canManage(r.Context(), claims, agent, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can finish this room")
		return
	}

	// Body is optional: an empty body archives without a result reference.
	var req archiveRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		roomWriteError(w, http.StatusBadRequest, "INVALID_JSON", "invalid request body")
		return
	}

	// A supplied result message must belong to this room.
	if req.ResultMessageID != nil {
		if _, err := h.msgRepo.GetByID(r.Context(), room.ID, *req.ResultMessageID); err != nil {
			if errors.Is(err, db.ErrMessageNotFound) {
				roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "result_message_id does not reference a message in this room")
				return
			}
			slog.Error("failed to verify result message", "error", err, "room_id", room.ID)
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to verify result message")
			return
		}
	}

	archived, err := h.roomRepo.Archive(r.Context(), room.ID, req.ResultMessageID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to archive room", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to archive room")
		return
	}

	// A finished room must not linger as live activity in the public overview.
	h.invalidateOverview(r.Context())

	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": archived})
}

// ReopenRoom handles POST /v1/rooms/{slug}/reopen.
// Same ownership rules as ArchiveRoom. Clears the archived state so the room is Live
// again and new messages/joins are accepted; the recorded result is retained.
func (h *RoomHandler) ReopenRoom(w http.ResponseWriter, r *http.Request) {
	slug := chi.URLParam(r, "slug")
	if slug == "" {
		roomWriteError(w, http.StatusBadRequest, "VALIDATION_ERROR", "slug is required")
		return
	}

	claims := auth.ClaimsFromContext(r.Context())
	agent := auth.AgentFromContext(r.Context())
	if claims == nil && agent == nil {
		roomWriteError(w, http.StatusUnauthorized, "UNAUTHORIZED", "authentication required")
		return
	}

	room, err := h.roomRepo.GetBySlug(r.Context(), slug)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to get room for reopen", "error", err)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}

	if !h.canManage(r.Context(), claims, agent, room) {
		roomWriteError(w, http.StatusForbidden, "FORBIDDEN", "only the room owner or admin can reopen this room")
		return
	}

	reopened, err := h.roomRepo.Reopen(r.Context(), room.ID)
	if err != nil {
		if errors.Is(err, db.ErrRoomNotFound) {
			roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
			return
		}
		slog.Error("failed to reopen room", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to reopen room")
		return
	}

	h.invalidateOverview(r.Context())

	roomWriteJSON(w, http.StatusOK, map[string]interface{}{"data": reopened})
}
