package api

import (
	"net/http"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
	"github.com/go-chi/chi/v5"
)

// mountAbuseAdminRoutes mounts the operator anti-abuse endpoints (bans, IPFS unpin and gc). Like every /admin
// route they sit behind the operator gate, and the handlers check the admin key again.
func mountAbuseAdminRoutes(r chi.Router, pool *db.Pool, operatorOnly func(http.Handler) http.Handler, ipfsAPIURL string) {
	if pool == nil {
		return
	}
	bans := handlers.NewAdminBansHandler(db.NewAccountBanRepository(pool))
	r.With(operatorOnly).Post("/admin/bans", bans.Ban)

	// IPFS unpin and garbage collection (W5): unpin retries like pinning; repo/gc can take
	// minutes on a large repo and is never retried.
	unpin := services.NewKuboIPFSServiceWithConfig(ipfsAPIURL, services.IPFSConfig{Timeout: 30 * time.Second, MaxRetries: 1, RetryDelay: time.Second})
	gc := services.NewKuboIPFSServiceWithConfig(ipfsAPIURL, services.IPFSConfig{Timeout: 10 * time.Minute})
	ipfsAdmin := handlers.NewAdminIPFSHandler(unpin, gc, db.NewIPFSReferenceRepository(pool))
	r.With(operatorOnly).Post("/admin/ipfs/unpin", ipfsAdmin.Unpin)
	r.With(operatorOnly).Post("/admin/ipfs/gc", ipfsAdmin.GC)
}
