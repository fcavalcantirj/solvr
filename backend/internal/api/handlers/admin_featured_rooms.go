package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// FeaturedRoomStore curates the homepage's featured pool (db.FeaturedRoomRepository).
type FeaturedRoomStore interface {
	Feature(ctx context.Context, slug string, askSeq, outcomeSeq *int) (*db.FeaturedRoom, error)
	Unfeature(ctx context.Context, slug string) (bool, error)
	ListAll(ctx context.Context) ([]db.FeaturedRoom, error)
}

// AdminFeaturedRoomsHandler serves the operator's curation of the featured
// rooms (SPEC Part 26, "Featured rooms"): PUT and DELETE
// /admin/rooms/{slug}/featured and GET /admin/rooms/featured.
type AdminFeaturedRoomsHandler struct {
	store FeaturedRoomStore
	// overviewChanged announces a change to every API instance
	// (db.Pool.OverviewChanged); nil drops this instance's snapshot only.
	overviewChanged func(ctx context.Context) error
	// now reads the clock "shown today" is computed at; nil means time.Now.
	now func() time.Time
}

// NewAdminFeaturedRoomsHandler wires the handler to the pool.
func NewAdminFeaturedRoomsHandler(store FeaturedRoomStore, overviewChanged func(ctx context.Context) error) *AdminFeaturedRoomsHandler {
	return &AdminFeaturedRoomsHandler{store: store, overviewChanged: overviewChanged}
}

// featuredRoomRequest is the optional body of PUT /admin/rooms/{slug}/featured.
type featuredRoomRequest struct {
	AskSeq     *int `json:"ask_seq"`
	OutcomeSeq *int `json:"outcome_seq"`
}

// featuredRoomView is one room of the pool as the operator sees it.
type featuredRoomView struct {
	Slug       string    `json:"slug"`
	FeaturedAt time.Time `json:"featured_at"`
	AskSeq     *int      `json:"ask_seq"`
	OutcomeSeq *int      `json:"outcome_seq"`
	Public     bool      `json:"public"`
	ShownToday bool      `json:"shown_today"`
}

func viewOfFeatured(r db.FeaturedRoom, shown bool) featuredRoomView {
	return featuredRoomView{Slug: r.Slug, FeaturedAt: r.FeaturedAt, AskSeq: r.AskSeq,
		OutcomeSeq: r.OutcomeSeq, Public: r.Public, ShownToday: shown}
}

// Feature handles PUT /admin/rooms/{slug}/featured. The body may name the
// messages the card quotes; without it the API picks them.
func (h *AdminFeaturedRoomsHandler) Feature(w http.ResponseWriter, r *http.Request) {
	if !(&AdminHandler{}).checkAdminAuth(w, r) {
		return
	}
	var body featuredRoomRequest
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil && !errors.Is(err, io.EOF) {
		writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "invalid JSON body")
		return
	}
	for _, seq := range []*int{body.AskSeq, body.OutcomeSeq} {
		if seq != nil && *seq < 1 {
			writeAdminError(w, http.StatusBadRequest, "VALIDATION_ERROR", "ask_seq and outcome_seq are message numbers, 1 or more")
			return
		}
	}

	slug := chi.URLParam(r, "slug")
	room, err := h.store.Feature(r.Context(), slug, body.AskSeq, body.OutcomeSeq)
	switch {
	case errors.Is(err, db.ErrRoomNotFound):
		writeAdminError(w, http.StatusNotFound, "NOT_FOUND", "no public room with that slug")
		return
	case err != nil:
		slog.Error("feature room failed", "error", err, "slug", slug)
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "feature room failed")
		return
	}
	h.changed(r.Context())
	slog.Info("room featured", "slug", slug)
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{"data": viewOfFeatured(*room, false)})
}

// Unfeature handles DELETE /admin/rooms/{slug}/featured.
func (h *AdminFeaturedRoomsHandler) Unfeature(w http.ResponseWriter, r *http.Request) {
	if !(&AdminHandler{}).checkAdminAuth(w, r) {
		return
	}
	slug := chi.URLParam(r, "slug")
	removed, err := h.store.Unfeature(r.Context(), slug)
	if err != nil {
		slog.Error("unfeature room failed", "error", err, "slug", slug)
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unfeature room failed")
		return
	}
	if removed {
		h.changed(r.Context())
		slog.Info("room unfeatured", "slug", slug)
	}
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{"data": map[string]interface{}{"slug": slug, "removed": removed}})
}

// List handles GET /admin/rooms/featured: the whole pool, oldest feature
// first, marking the rooms the homepage shows today.
func (h *AdminFeaturedRoomsHandler) List(w http.ResponseWriter, r *http.Request) {
	if !(&AdminHandler{}).checkAdminAuth(w, r) {
		return
	}
	all, err := h.store.ListAll(r.Context())
	if err != nil {
		slog.Error("list featured rooms failed", "error", err)
		writeAdminError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "list featured rooms failed")
		return
	}

	public := make([]db.FeaturedRoom, 0, len(all))
	for _, room := range all {
		if room.Public {
			public = append(public, room)
		}
	}
	now := time.Now()
	if h.now != nil {
		now = h.now()
	}
	shown := map[string]bool{}
	for _, room := range selectFeaturedRooms(public, ExampleRoomSlug(), now) {
		shown[room.Slug] = true
	}

	views := make([]featuredRoomView, 0, len(all))
	for _, room := range all {
		views = append(views, viewOfFeatured(room, shown[room.Slug]))
	}
	writeAdminJSON(w, http.StatusOK, map[string]interface{}{"data": views})
}

// changed drops the cached overview after a committed change to the pool.
func (h *AdminFeaturedRoomsHandler) changed(ctx context.Context) {
	if h.overviewChanged == nil {
		InvalidateOverviewCache()
		return
	}
	if err := h.overviewChanged(ctx); err != nil {
		slog.Warn("overview change not announced to other instances", "error", err)
	}
}
