package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// mountOpsRoutes mounts the operations report (spec.json idx 79): the reliability gates
// computed from what the service recorded. Like every /admin route it sits behind the
// operator gate and the handler checks the operator key again. Kept out of router.go,
// which is over the size limit.
func mountOpsRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler) {
	if pool == nil {
		return
	}
	h := handlers.NewOpsSLOHandler(db.NewOpsSLORepository(pool))
	r.With(operatorOnly).Get("/admin/ops/slo", h.GetSLO)
}
