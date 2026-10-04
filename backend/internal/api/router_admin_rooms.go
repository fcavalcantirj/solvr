package api

import (
	"net/http"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/go-chi/chi/v5"
)

// mountFeaturedRoomsAdminRoutes mounts the operator's curation of the homepage's featured rooms
// (SPEC Part 26, "Featured rooms"). Like every /admin route they sit behind the operator gate, and
// the handler checks the admin key again. A change announces itself to every API instance.
func mountFeaturedRoomsAdminRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	h := handlers.NewAdminFeaturedRoomsHandler(db.NewFeaturedRoomRepository(pool), pool.OverviewChanged)
	r.With(operatorOnly).Put("/admin/rooms/{slug}/featured", h.Feature)
	r.With(operatorOnly).Delete("/admin/rooms/{slug}/featured", h.Unfeature)
	r.With(operatorOnly).Get("/admin/rooms/featured", h.List)
}
