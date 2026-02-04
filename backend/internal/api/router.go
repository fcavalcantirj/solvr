// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
)

// Version is the API version string
const Version = "0.1.0"

// NewRouter creates and configures a new chi router with all middleware.
// The pool parameter is optional - if nil, /health/ready will return 503.
func NewRouter(pool *db.Pool) *chi.Mux {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(requestIDMiddleware)
	r.Use(apimiddleware.Logging)
	r.Use(securityHeadersMiddleware)
	r.Use(jsonContentTypeMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// CORS configuration
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"http://localhost:3000", "https://solvr.dev", "https://www.solvr.dev"},
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           int(12 * time.Hour / time.Second),
	}))

	// Custom 404 and 405 handlers for JSON responses
	r.NotFound(notFoundHandler)
	r.MethodNotAllowed(methodNotAllowedHandler)

	// Health endpoints
	r.Get("/health", healthHandler)
	r.Get("/health/live", healthLiveHandler)
	r.Get("/health/ready", healthReadyHandler(pool))

	// Discovery endpoints (SPEC.md Part 18.3)
	r.Get("/.well-known/ai-agent.json", wellKnownAIAgentHandler)
	r.Get("/v1/openapi.json", openAPIJSONHandler)
	r.Get("/v1/openapi.yaml", openAPIYAMLHandler)

	// Mount v1 API routes
	mountV1Routes(r, pool)

	return r
}

// mountV1Routes mounts all v1 API routes.
func mountV1Routes(r *chi.Mux, pool *db.Pool) {
	// Create repositories and handlers
	var agentRepo handlers.AgentRepositoryInterface
	var claimTokenRepo handlers.ClaimTokenRepositoryInterface
	var postsRepo handlers.PostsRepositoryInterface
	var searchRepo handlers.SearchRepositoryInterface
	if pool != nil {
		agentRepo = db.NewAgentRepository(pool)
		claimTokenRepo = db.NewClaimTokenRepository(pool)
		postsRepo = db.NewPostRepository(pool)
		searchRepo = db.NewSearchRepository(pool)
	} else {
		// Use in-memory repository for testing when no DB is available
		agentRepo = NewInMemoryAgentRepository()
		claimTokenRepo = NewInMemoryClaimTokenRepository()
		postsRepo = NewInMemoryPostRepository()
		searchRepo = NewInMemorySearchRepository()
	}

	agentsHandler := handlers.NewAgentsHandler(agentRepo, "")
	agentsHandler.SetClaimTokenRepository(claimTokenRepo)
	agentsHandler.SetBaseURL("https://solvr.dev")

	// Create posts handler
	postsHandler := handlers.NewPostsHandler(postsRepo)

	// Create search handler (per SPEC.md Part 5.5)
	searchHandler := handlers.NewSearchHandler(searchRepo)

	// JWT secret for auth middleware
	jwtSecret := "test-jwt-secret"

	// Create OAuth handlers for GitHub and Google OAuth
	// Per SPEC.md Part 5.2: OAuth authentication endpoints
	oauthConfig := &handlers.OAuthConfig{
		// Config values can be empty for testing - actual values come from env vars in production
		GitHubClientID:     "",
		GitHubClientSecret: "",
		GitHubRedirectURI:  "",
		GoogleClientID:     "",
		GoogleClientSecret: "",
		GoogleRedirectURI:  "",
		JWTSecret:          jwtSecret,
		JWTExpiry:          "15m",
		RefreshExpiry:      "7d",
		FrontendURL:        "http://localhost:3000",
	}
	oauthHandlers := handlers.NewOAuthHandlers(oauthConfig, pool, nil)

	// Create API key validator for agent authentication
	// The agentRepo implements auth.AgentDB interface with GetAgentByAPIKeyHash
	apiKeyValidator := auth.NewAPIKeyValidator(agentRepo)

	// v1 API routes
	r.Route("/v1", func(r chi.Router) {
		// Agent self-registration (no auth required)
		// Per AGENT-ONBOARDING requirement: POST /v1/agents/register
		r.Post("/agents/register", agentsHandler.RegisterAgent)

		// Agent claim endpoints (API-CRITICAL requirement)
		// POST /v1/agents/me/claim - agent generates claim URL (requires API key auth)
		// Per FIX-002: Add API key auth middleware
		r.Group(func(r chi.Router) {
			r.Use(auth.APIKeyMiddleware(apiKeyValidator))
			r.Post("/agents/me/claim", agentsHandler.GenerateClaim)
		})

		// Claim token endpoints (API-CRITICAL requirement)
		// GET /v1/claim/{token} - get claim info (no auth required)
		r.Get("/claim/{token}", func(w http.ResponseWriter, req *http.Request) {
			token := chi.URLParam(req, "token")
			agentsHandler.GetClaimInfo(w, req, token)
		})

		// POST /v1/claim/{token} - confirm claim (requires JWT auth)
		r.Post("/claim/{token}", func(w http.ResponseWriter, req *http.Request) {
			token := chi.URLParam(req, "token")
			agentsHandler.ConfirmClaim(w, req, token)
		})

		// OAuth endpoints (API-CRITICAL requirement)
		// Per SPEC.md Part 5.2: GitHub OAuth
		r.Get("/auth/github", oauthHandlers.GitHubRedirect)
		r.Get("/auth/github/callback", oauthHandlers.GitHubCallback)

		// Per SPEC.md Part 5.2: Google OAuth
		r.Get("/auth/google", oauthHandlers.GoogleRedirect)
		r.Get("/auth/google/callback", oauthHandlers.GoogleCallback)

		// Search endpoint (API-CRITICAL per SPEC.md Part 5.5)
		// GET /v1/search - search the knowledge base (no auth required)
		r.Get("/search", searchHandler.Search)

		// Agent profile endpoint (per SPEC.md Part 5.6)
		// GET /v1/agents/{id} - get agent profile (no auth required)
		r.Get("/agents/{id}", func(w http.ResponseWriter, req *http.Request) {
			agentID := chi.URLParam(req, "id")
			agentsHandler.GetAgent(w, req, agentID)
		})

		// Posts endpoints (API-CRITICAL requirement)
		// Per SPEC.md Part 5.6: GET /v1/posts - list posts (no auth required)
		r.Get("/posts", postsHandler.List)
		// Per SPEC.md Part 5.6: GET /v1/posts/:id - single post (no auth required)
		r.Get("/posts/{id}", postsHandler.Get)

		// Protected posts routes (require authentication)
		r.Group(func(r chi.Router) {
			// Use JWT middleware for protected routes
			r.Use(auth.JWTMiddleware(jwtSecret))

			// Per SPEC.md Part 5.6: POST /v1/posts - create post (requires auth)
			r.Post("/posts", postsHandler.Create)
			// Per SPEC.md Part 5.6: PATCH /v1/posts/:id - update post (requires auth)
			r.Patch("/posts/{id}", postsHandler.Update)
			// Per SPEC.md Part 5.6: DELETE /v1/posts/:id - delete post (requires auth)
			r.Delete("/posts/{id}", postsHandler.Delete)
			// Per SPEC.md Part 5.6: POST /v1/posts/:id/vote - vote on post (requires auth)
			r.Post("/posts/{id}/vote", postsHandler.Vote)
		})
	})
}

// requestIDMiddleware adds a unique request ID to each request
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := r.Header.Get("X-Request-ID")
		if requestID == "" {
			requestID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r)
	})
}

// securityHeadersMiddleware adds security headers to all responses
func securityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
		next.ServeHTTP(w, r)
	})
}

// jsonContentTypeMiddleware sets Content-Type to application/json
func jsonContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		next.ServeHTTP(w, r)
	})
}

// HealthResponse is the response structure for health endpoints
type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
	Database  string `json:"database,omitempty"`
}

// ErrorResponse is the standard error response structure
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error details
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// healthHandler handles GET /health
func healthHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status:    "ok",
		Version:   Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	}
	writeJSON(w, http.StatusOK, response)
}

// healthLiveHandler handles GET /health/live
func healthLiveHandler(w http.ResponseWriter, r *http.Request) {
	response := HealthResponse{
		Status: "alive",
	}
	writeJSON(w, http.StatusOK, response)
}

// healthReadyHandler handles GET /health/ready
func healthReadyHandler(pool *db.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if pool == nil {
			writeError(w, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "database not configured")
			return
		}

		// Ping the database
		if err := pool.Ping(r.Context()); err != nil {
			writeError(w, http.StatusServiceUnavailable, "DATABASE_UNAVAILABLE", "database ping failed")
			return
		}

		response := HealthResponse{
			Status:   "ready",
			Database: "ok",
		}
		writeJSON(w, http.StatusOK, response)
	}
}

// notFoundHandler handles 404 responses
func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusNotFound, "NOT_FOUND", "resource not found")
}

// methodNotAllowedHandler handles 405 responses
func methodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	writeError(w, http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED", "method not allowed")
}

// writeJSON writes a JSON response with the given status code
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		// If encoding fails, we can't really recover gracefully
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"failed to encode response"}}`, http.StatusInternalServerError)
	}
}

// writeError writes a JSON error response
func writeError(w http.ResponseWriter, status int, code, message string) {
	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(response)
}
