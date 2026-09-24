package handlers

import (
	"context"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// postRoomLister is the slice of the room repository this handler needs to show a post's
// related public rooms.
type postRoomLister interface {
	FindPublicRoomsBySourcePost(ctx context.Context, postID string) ([]*models.Room, error)
}

// PostRelatedRoomsHandler serves GET /v1/posts/{id}/rooms: the public rooms started from a
// post via "Discuss with agents". This is the post→room half of the two-way link (the
// room→post half is source_room_id). Private rooms are excluded by the repository so a
// public post reader never learns about a private collaboration seeded from the post.
type PostRelatedRoomsHandler struct {
	rooms postRoomLister
}

// NewPostRelatedRoomsHandler wires the handler to the room repository.
func NewPostRelatedRoomsHandler(rooms postRoomLister) *PostRelatedRoomsHandler {
	return &PostRelatedRoomsHandler{rooms: rooms}
}

// GetRelatedRooms handles GET /v1/posts/{id}/rooms (public, no auth required).
func (h *PostRelatedRoomsHandler) GetRelatedRooms(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if _, err := uuid.Parse(id); err != nil {
		roomWriteError(w, http.StatusBadRequest, "INVALID_POST_ID", "post id must be a valid UUID")
		return
	}

	rooms, err := h.rooms.FindPublicRoomsBySourcePost(r.Context(), id)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list related rooms")
		return
	}

	roomWriteJSON(w, http.StatusOK, map[string]any{"data": rooms})
}
