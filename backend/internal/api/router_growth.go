package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountGrowthPlanningRoutes mounts lane G2's operator growth reports (spec.json idx 86-91):
// participant counts, stage gates, the acquisition model and the acquisition loop. Lane G1's
// share-attribution report is mounted beside it by mountGrowthReportRoutes. They are reporting
// Solvr does about itself, so like every /admin route they sit behind the operator gate, and the
// handler checks the operator key again. Kept out of router.go, which is over the size limit.
func mountGrowthPlanningRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	h := handlers.NewGrowthReportsHandler(handlers.GrowthReaders{
		Participants: db.NewParticipantActivityRepository(pool),
		Stages:       db.NewGrowthStageRepository(pool),
		Model:        db.NewAcquisitionModelRepository(pool),
	})
	r.With(operatorOnly).Get("/admin/growth/participants", h.GetParticipants)
	r.With(operatorOnly).Get("/admin/growth/stages", h.GetStages)
	r.With(operatorOnly).Get("/admin/growth/model", h.GetModel)
}
