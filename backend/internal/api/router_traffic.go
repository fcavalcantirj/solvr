package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountTrafficRoutes registers who the traffic came from (SPEC.md 27.6).
//
//	POST /v1/analytics/pageview     the site's web server reports a page it served,
//	                                consent-free; public, it carries no credential
//	GET  /admin/traffic/summary     who a window's traffic came from (operator-only)
//	POST /admin/traffic/reclassify  the behavioural rules over a named window, dry run
//	                                first (operator-only)
//
// Kept out of router.go, which is already near the file-size limit.
func mountTrafficRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	r.Post("/v1/analytics/pageview", handlers.NewPageViewHandler(db.NewPageViewRepository(pool)).Record)

	h := handlers.NewTrafficReportHandler(db.NewTrafficReportRepository(pool), db.NewTrafficRules(pool))
	r.With(operatorOnly).Get("/admin/traffic/summary", h.GetSummary)
	r.With(operatorOnly).Post("/admin/traffic/reclassify", h.Reclassify)
}
