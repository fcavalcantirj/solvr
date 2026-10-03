package handlers

import (
	"errors"
	"log/slog"
	"net/http"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/seo"
	"github.com/go-chi/chi/v5"
)

// RoomSEO is what a room's page tells search engines (task idx 80, SPEC.md Part 27).
// The API decides it; the page renders it as robots, title and description metadata.
type RoomSEO struct {
	// Indexable is true for a public, live room carrying a two-way exchange
	// (db.roomIndexablePredicate), the same rule the rooms sitemap lists by.
	Indexable bool `json:"indexable"`
	// Title is the room's own name.
	Title string `json:"title"`
	// Description is the room's stated purpose (seo.RoomDescription).
	Description string `json:"description"`
}

// GetRoomSEO handles GET /v1/rooms/{slug}/seo: the room page's search verdict. It is
// mounted behind the same read policy as GET /v1/rooms/{slug}, so it answers 404 and
// 403 exactly when the room read does.
func (h *RoomHandler) GetRoomSEO(w http.ResponseWriter, r *http.Request) {
	room := apimiddleware.RoomFromContext(r.Context())
	if room == nil {
		var err error
		room, err = h.roomRepo.GetBySlug(r.Context(), chi.URLParam(r, "slug"))
		if err != nil {
			if errors.Is(err, db.ErrRoomNotFound) {
				roomWriteError(w, http.StatusNotFound, "NOT_FOUND", "room not found")
				return
			}
			slog.Error("failed to get room", "error", err)
			roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
			return
		}
	}

	// A page told "not indexable" during a database hiccup could drop out of search;
	// a retryable 500 cannot.
	indexable, err := h.roomRepo.IsIndexable(r.Context(), room.ID)
	if err != nil {
		slog.Error("failed to derive room seo", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}
	task := ""
	first, err := h.msgRepo.GetFirstMessage(r.Context(), room.ID)
	switch {
	case err == nil:
		task = first.Content
	case !errors.Is(err, db.ErrRoomNotFound): // an empty room has no first message
		slog.Error("failed to get first message", "error", err, "room_id", room.ID)
		roomWriteError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get room")
		return
	}
	roomWriteJSON(w, http.StatusOK, map[string]RoomSEO{"data": {
		Indexable:   indexable,
		Title:       room.DisplayName,
		Description: seo.RoomDescription(room.DisplayName, room.Description, task, room.MessageCount),
	}})
}
