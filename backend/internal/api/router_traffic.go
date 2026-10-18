package api

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountTrafficRoutes registers who the traffic came from (SPEC.md 27.6).
//
//	POST /v1/analytics/pageview     the site's web server reports a page it served,
//	                                consent-free; with WEB_REPORT_SECRET set, only a report
//	                                carrying it in X-Solvr-Web-Report is taken (F6)
//	GET  /admin/traffic/summary     who a window's traffic came from (operator-only)
//	POST /admin/traffic/reclassify  the behavioural rules over a named window, dry run
//	                                first (operator-only)
//
// Kept out of router.go, which is already near the file-size limit.
func mountTrafficRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	webReportSecret := os.Getenv("WEB_REPORT_SECRET")
	if webReportSecret == "" {
		slog.Info("page view reports are taken from anyone: WEB_REPORT_SECRET is not set")
	} else {
		slog.Info("page view reports are taken only with the web server's secret")
	}
	r.Post("/v1/analytics/pageview", handlers.NewPageViewHandler(db.NewPageViewRepository(pool), webReportSecret).Record)

	h := handlers.NewTrafficReportHandler(db.NewTrafficReportRepository(pool), db.NewTrafficRules(pool))
	r.With(operatorOnly).Get("/admin/traffic/summary", h.GetSummary)
	r.With(operatorOnly).Post("/admin/traffic/reclassify", h.Reclassify)
}
