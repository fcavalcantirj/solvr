package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountSEOReportRoutes mounts lane C's operator SEO baseline (task idx 85). Operator-only,
// like every /admin route. Kept out of router.go, which is already over the size limit.
func mountSEOReportRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	h := handlers.NewSEOBaselineHandler(db.NewSEOBaselineRepository(pool))
	r.With(operatorOnly).Get("/admin/seo/baseline", h.GetBaseline)
}
