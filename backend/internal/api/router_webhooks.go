package api

import (
	"net/http"
	"os"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/services"
	"github.com/go-chi/chi/v5"
)

// webhookSealSecret is the server secret webhook signing secrets are sealed under: the JWT
// secret the router authenticates with (the claim tokens are sealed under it too), each use
// under its own derived key. The fallback matches the router's, for tests.
func webhookSealSecret() string {
	if secret := os.Getenv("JWT_SECRET"); secret != "" {
		return secret
	}
	return "test-jwt-secret-32-chars-long!!"
}

// newWebhookRepository is the webhook store every server shares: the routes write it, the
// delivery job reads it.
func newWebhookRepository(pool *db.Pool) *db.WebhookRepository {
	return db.NewWebhookRepository(pool).WithSealSecret(webhookSealSecret())
}

// mountWebhookRoutes serves an agent's webhooks (SPEC.md Part 12.3) to the agent itself or
// the human who owns it. r must authenticate its callers.
func mountWebhookRoutes(r chi.Router, pool *db.Pool) {
	h := handlers.NewWebhooksHandler(newWebhookRepository(pool))
	r.Post("/agents/{id}/webhooks", func(w http.ResponseWriter, req *http.Request) {
		h.CreateWebhook(w, req, chi.URLParam(req, "id"))
	})
	r.Get("/agents/{id}/webhooks", func(w http.ResponseWriter, req *http.Request) {
		h.ListWebhooks(w, req, chi.URLParam(req, "id"))
	})
	r.Get("/agents/{id}/webhooks/{wh_id}", func(w http.ResponseWriter, req *http.Request) {
		h.GetWebhook(w, req, chi.URLParam(req, "id"), chi.URLParam(req, "wh_id"))
	})
	r.Patch("/agents/{id}/webhooks/{wh_id}", func(w http.ResponseWriter, req *http.Request) {
		h.UpdateWebhook(w, req, chi.URLParam(req, "id"), chi.URLParam(req, "wh_id"))
	})
	r.Delete("/agents/{id}/webhooks/{wh_id}", func(w http.ResponseWriter, req *http.Request) {
		h.DeleteWebhook(w, req, chi.URLParam(req, "id"), chi.URLParam(req, "wh_id"))
	})
}

// NewWebhookDeliveryJob is the job that sends the queued webhook deliveries, over the store
// the routes write, with client (services.NewWebhookHTTPClient in production).
func NewWebhookDeliveryJob(pool *db.Pool, client *http.Client) *jobs.WebhookDeliveryJob {
	repo := newWebhookRepository(pool)
	return jobs.NewWebhookDeliveryJob(repo, services.NewWebhookDeliveryService(repo, client), jobs.DefaultWebhookDeliveryBatchSize)
}
