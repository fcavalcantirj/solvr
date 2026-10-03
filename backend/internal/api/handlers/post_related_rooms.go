package handlers

import (
	"context"
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// postRoomLister is the slice of the room repository this handler needs to show a post's
// related public rooms.
type postRoomLister interface {
	FindPublicRoomsBySourcePost(ctx context.Context, postID string) ([]*models.Room, error)
}

// postSourceRoomFinder names the public, live room a post was saved from
// (db.RoomRepository.FindPublicSourceRoom). A repository without it names none.
type postSourceRoomFinder interface {
	FindPublicSourceRoom(ctx context.Context, postID string) (*db.SourceRoom, error)
}

// postReadChecker applies the GET /v1/posts/{id} read rule (exists, not deleted, public or
// the caller's family), so the list answers 404 whenever the post itself would.
type postReadChecker interface {
	VisibleTo(ctx context.Context, postID, callerHuman string) (bool, error)
}

// PostRelatedRoomsHandler serves GET /v1/posts/{id}/rooms: the public rooms started from a
// post via "Discuss with agents". This is the post→room half of the two-way link (the
// room→post half is source_room_id). Private rooms are excluded by the repository so a
// public post reader never learns about a private collaboration seeded from the post.
type PostRelatedRoomsHandler struct {
	rooms postRoomLister
	posts postReadChecker
}

// NewPostRelatedRoomsHandler wires the handler to the room and post repositories.
func NewPostRelatedRoomsHandler(rooms postRoomLister, posts postReadChecker) *PostRelatedRoomsHandler {
	return &PostRelatedRoomsHandler{rooms: rooms, posts: posts}
}

// GetRelatedRooms handles GET /v1/posts/{id}/rooms (public, no auth required).
func (h *PostRelatedRoomsHandler) GetRelatedRooms(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	visible, err := h.posts.VisibleTo(r.Context(), id, callerHumanID(r))
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to load post")
		return
	}
	if !visible {
		roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "post not found")
		return
	}

	rooms, err := h.rooms.FindPublicRoomsBySourcePost(r.Context(), id)
	if err != nil {
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list related rooms")
		return
	}

	// The room the post was saved from, when it is public and live (task idx 82): the
	// outcome links back to its conversation.
	var source any
	if finder, ok := h.rooms.(postSourceRoomFinder); ok {
		room, err := finder.FindPublicSourceRoom(r.Context(), id)
		if err != nil {
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to find the source room")
			return
		}
		if room != nil {
			source = room
		}
	}

	roomWriteJSON(w, http.StatusOK, map[string]any{"data": rooms, "source_room": source})
}
