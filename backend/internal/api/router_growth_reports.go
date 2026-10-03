package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountGrowthReportRoutes mounts lane G1's operator reports (idx 88/92). Operator-only,
// like every /admin route. Kept out of router.go, which is already over the size limit.
func mountGrowthReportRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	shareHandler := handlers.NewShareAttributionHandler(db.NewShareAttributionRepository(pool))
	r.With(operatorOnly).Get("/admin/share-attribution", shareHandler.GetReport)

	returnHandler := handlers.NewReturnUsageHandler(db.NewReturnUsageRepository(pool))
	r.With(operatorOnly).Get("/admin/return-usage", returnHandler.GetReport)
}
