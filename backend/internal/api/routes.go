package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// MountAPIRoutes mounts additional API routes to the router
// Note: /v1 route group already exists in router.go, so we add routes directly
func MountAPIRoutes(r *chi.Mux, pool *db.Pool, jwtSecret string) {
	if pool == nil {
		return
	}

	// Initialize search repository
	searchRepo := db.NewSearchRepository(pool)

	// Search endpoint
	r.Get("/v1/search", func(w http.ResponseWriter, req *http.Request) {
		query := req.URL.Query().Get("q")
		if query == "" {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"code":    "BAD_REQUEST",
					"message": "query parameter 'q' is required",
				},
			})
			return
		}

		// Parse pagination
		page := 1
		perPage := 20
		if p := req.URL.Query().Get("page"); p != "" {
			if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
				page = parsed
			}
		}
		if pp := req.URL.Query().Get("per_page"); pp != "" {
			if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 50 {
				perPage = parsed
			}
		}

		opts := models.SearchOptions{
			Page:    page,
			PerPage: perPage,
			Type:    req.URL.Query().Get("type"),
			Status:  req.URL.Query().Get("status"),
		}
		if tags := req.URL.Query().Get("tags"); tags != "" {
			opts.Tags = []string{tags}
		}

		results, total, err := searchRepo.Search(req.Context(), query, opts)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": map[string]string{
					"code":    "INTERNAL_ERROR",
					"message": "search failed",
				},
			})
			return
		}

		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": results,
			"meta": map[string]interface{}{
				"query":    query,
				"total":    total,
				"page":     page,
				"per_page": perPage,
				"has_more": total > page*perPage,
			},
		})
	})

	// Placeholder for agents
	r.Get("/v1/agents/{id}", func(w http.ResponseWriter, req *http.Request) {
		agentID := chi.URLParam(req, "id")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"message": "Agent endpoint - coming soon",
			"id":      agentID,
		})
	})

	// Placeholder for posts
	r.Get("/v1/posts", func(w http.ResponseWriter, req *http.Request) {
		json.NewEncoder(w).Encode(map[string]interface{}{
			"data": []interface{}{},
			"meta": map[string]interface{}{
				"message": "Posts endpoint - coming soon",
			},
		})
	})
}
