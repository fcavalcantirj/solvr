// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// Version is the API version string
const Version = "0.2.0"

// NewRouter creates and configures a new chi router with all middleware.
// The pool parameter is optional - if nil, /health/ready will return 503.
func NewRouter(pool *db.Pool) *chi.Mux {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(requestIDMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// CORS configuration - MUST be early in the chain so error responses include CORS headers
	// Read from ALLOWED_ORIGINS env var or use defaults
	allowedOrigins := []string{"http://localhost:3000", "https://solvr.dev", "https://www.solvr.dev"}
	if envOrigins := os.Getenv("ALLOWED_ORIGINS"); envOrigins != "" {
		allowedOrigins = strings.Split(envOrigins, ",")
		// Trim whitespace from each origin
		for i, origin := range allowedOrigins {
			allowedOrigins[i] = strings.TrimSpace(origin)
		}
	}
	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   allowedOrigins,
		AllowedMethods:   []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Session-ID"},
		ExposedHeaders:   []string{"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset"},
		AllowCredentials: true,
		MaxAge:           int(12 * time.Hour / time.Second),
	}))

	// Other middleware after CORS
	r.Use(apimiddleware.Logging)
	r.Use(apimiddleware.BodyLimit(64 * 1024)) // FIX-028: 64KB request body limit
	r.Use(securityHeadersMiddleware)
	r.Use(jsonContentTypeMiddleware)

	// Rate limiting - load config from database with fallback to defaults
	rateLimitConfig := loadRateLimitConfig(pool)
	rateLimitStore := apimiddleware.NewInMemoryRateLimitStore()
	rateLimiter := apimiddleware.NewRateLimiter(rateLimitStore, rateLimitConfig)
	r.Use(rateLimiter.Middleware)

	// Custom 404 and 405 handlers for JSON responses
	r.NotFound(notFoundHandler)
	r.MethodNotAllowed(methodNotAllowedHandler)

	// Health endpoints
	r.Get("/health", healthHandler)
	r.Get("/health/live", healthLiveHandler)
	r.Get("/health/ready", healthReadyHandler(pool))

	// Admin endpoints (requires X-Admin-API-Key header)
	adminHandler := handlers.NewAdminHandler(pool)
	r.Post("/admin/query", adminHandler.ExecuteQuery)

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
	var feedRepo handlers.FeedRepositoryInterface
	var userRepo handlers.MeUserRepositoryInterface
	var problemsRepo handlers.ProblemsRepositoryInterface
	var questionsRepo handlers.QuestionsRepositoryInterface
	var ideasRepo handlers.IdeasRepositoryInterface
	var commentsRepo handlers.CommentsRepositoryInterface
	var notificationsRepo handlers.NotificationsRepositoryInterface
	var userAPIKeysRepo handlers.UserAPIKeyRepositoryInterface
	var bookmarksRepo handlers.BookmarksRepositoryInterface
	var viewsRepo handlers.ViewsRepositoryInterface
	var reportsRepo handlers.ReportsRepositoryInterface
	if pool == nil {
		log.Println("WARNING: Database pool is nil. V1 API routes will not be mounted.")
		return
	}

	agentRepo = db.NewAgentRepository(pool)
	claimTokenRepo = db.NewClaimTokenRepository(pool)
	postsRepo = db.NewPostRepository(pool)
	searchRepo = db.NewSearchRepository(pool)
	feedRepo = db.NewFeedRepository(pool)
	userRepo = db.NewUserRepository(pool)
	userAPIKeysRepo = db.NewUserAPIKeyRepository(pool)
	bookmarksRepo = db.NewBookmarkRepository(pool)
	viewsRepo = db.NewViewsRepository(pool)
	reportsRepo = db.NewReportsRepository(pool)
	problemsRepo = db.NewProblemsRepository(pool)
	questionsRepo = db.NewQuestionsRepository(pool)
	ideasRepo = db.NewIdeasRepository(pool)
	commentsRepo = db.NewCommentsRepository(pool)
	notificationsRepo = db.NewNotificationsRepository(pool)

	agentsHandler := handlers.NewAgentsHandler(agentRepo, "")
	agentsHandler.SetClaimTokenRepository(claimTokenRepo)
	agentsHandler.SetBaseURL("https://solvr.dev")

	// Create posts handler
	postsHandler := handlers.NewPostsHandler(postsRepo)

	// Create search handler (per SPEC.md Part 5.5)
	searchHandler := handlers.NewSearchHandler(searchRepo)

	// Create feed handler (per SPEC.md Part 5.6: GET /feed endpoints)
	feedHandler := handlers.NewFeedHandler(feedRepo)

	// Create content handlers (API-CRITICAL per PRD-v2)
	problemsHandler := handlers.NewProblemsHandler(problemsRepo)
	questionsHandler := handlers.NewQuestionsHandler(questionsRepo)
	ideasHandler := handlers.NewIdeasHandler(ideasRepo)
	commentsHandler := handlers.NewCommentsHandler(commentsRepo)
	commentsHandler.SetAgentRepository(agentRepo)

	// Per FIX-020: Set posts repository on content handlers so type-specific list endpoints
	// (GET /v1/problems, /v1/questions, /v1/ideas) return data consistent with /v1/posts
	problemsHandler.SetPostsRepository(postsRepo)
	questionsHandler.SetPostsRepository(postsRepo)
	ideasHandler.SetPostsRepository(postsRepo)

	// Create user-related handlers (API-CRITICAL per PRD-v2)
	notificationsHandler := handlers.NewNotificationsHandler(notificationsRepo)
	userAPIKeysHandler := handlers.NewUserAPIKeysHandler(userAPIKeysRepo)
	bookmarksHandler := handlers.NewBookmarksHandler(bookmarksRepo)
	viewsHandler := handlers.NewViewsHandler(viewsRepo)
	reportsHandler := handlers.NewReportsHandler(reportsRepo)

	// Create users handler (BE-003: User profile endpoints)
	// Type assertion to get the full interface needed by UsersHandler
	var usersUserRepo handlers.UsersUserRepositoryInterface
	var usersPostRepo handlers.UsersPostRepositoryInterface
	var usersListRepo handlers.UsersUserListRepositoryInterface
	if pool != nil {
		usersUserRepo = db.NewUserRepository(pool)
		usersPostRepo = db.NewPostRepository(pool)
		usersListRepo = db.NewUserRepository(pool)
	}
	usersHandler := handlers.NewUsersHandler(usersUserRepo, usersPostRepo)
	// Per prd-v4: Set agent repository for GET /v1/users/{id}/agents endpoint
	usersHandler.SetAgentRepository(agentRepo)
	// Per prd-v4: Set user list repository for GET /v1/users endpoint
	usersHandler.SetUserListRepository(usersListRepo)
	// Per prd-v4: Set contribution repositories for GET /v1/users/{id}/contributions endpoint
	usersHandler.SetContributionRepositories(
		db.NewAnswersRepository(pool),
		db.NewApproachesRepository(pool),
		db.NewResponsesRepository(pool),
	)

	// JWT secret for auth middleware - read from env or use test default
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-jwt-secret-32-chars-long!!"
	}

	// Read OAuth config from environment variables
	// Per SPEC.md Part 5.2: OAuth authentication endpoints
	frontendURL := os.Getenv("FRONTEND_URL")
	if frontendURL == "" {
		frontendURL = "http://localhost:3000"
	}

	oauthConfig := &handlers.OAuthConfig{
		GitHubClientID:     os.Getenv("GITHUB_CLIENT_ID"),
		GitHubClientSecret: os.Getenv("GITHUB_CLIENT_SECRET"),
		GitHubRedirectURI:  os.Getenv("GITHUB_REDIRECT_URI"),
		GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
		GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
		GoogleRedirectURI:  os.Getenv("GOOGLE_REDIRECT_URI"),
		JWTSecret:          jwtSecret,
		JWTExpiry:          "15m",
		RefreshExpiry:      "7d",
		FrontendURL:        frontendURL,
	}

	// Create OAuth handlers with user service for real user creation
	// Per BE-002: Google OAuth creates/finds users in database
	var oauthHandlers *handlers.OAuthHandlers
	if pool != nil {
		userRepo := db.NewUserRepository(pool)
		oauthUserService := services.NewOAuthUserService(userRepo)
		oauthUserAdapter := services.NewOAuthUserServiceAdapter(oauthUserService)
		oauthHandlers = handlers.NewOAuthHandlersWithUserService(oauthConfig, pool, nil, oauthUserAdapter)
	} else {
		// Fallback for testing when pool is nil
		oauthHandlers = handlers.NewOAuthHandlers(oauthConfig, pool, nil)
	}

	// Create API key validator for agent authentication
	// The agentRepo implements auth.AgentDB interface with GetAgentByAPIKeyHash
	apiKeyValidator := auth.NewAPIKeyValidator(agentRepo)

	// Create user API key validator for human programmatic access
	// userAPIKeysRepo implements auth.UserAPIKeyDB interface when backed by db.UserAPIKeyRepository
	var userAPIKeyValidator *auth.UserAPIKeyValidator
	if userAPIKeyDB, ok := userAPIKeysRepo.(auth.UserAPIKeyDB); ok {
		userAPIKeyValidator = auth.NewUserAPIKeyValidator(userAPIKeyDB)
	}

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

		// SECURE agent claiming endpoint (requires JWT auth - humans only)
		// POST /v1/agents/claim - claim agent with token from request body
		r.Group(func(r chi.Router) {
			r.Use(auth.JWTMiddleware(jwtSecret))
			r.Post("/agents/claim", agentsHandler.ClaimAgentWithToken)
		})

		// Public claim info endpoint (no auth required)
		// GET /v1/claim/{token} - get claim token info for confirmation page
		r.Get("/claim/{token}", agentsHandler.GetClaimInfo)

		// OAuth endpoints (API-CRITICAL requirement)
		// Per SPEC.md Part 5.2: GitHub OAuth
		r.Get("/auth/github", oauthHandlers.GitHubRedirect)
		r.Get("/auth/github/callback", oauthHandlers.GitHubCallback)

		// Per SPEC.md Part 5.2: Google OAuth
		r.Get("/auth/google", oauthHandlers.GoogleRedirect)
		r.Get("/auth/google/callback", oauthHandlers.GoogleCallback)

		// Moltbook OAuth (API-CRITICAL per PRD-v2)
		// Per SPEC.md Part 5.2: POST /auth/moltbook for agent authentication via Moltbook
		moltbookConfig := &handlers.MoltbookConfig{
			MoltbookAPIURL: "https://api.moltbook.dev",
		}
		moltbookHandler := handlers.NewMoltbookHandler(moltbookConfig, nil)
		r.Post("/auth/moltbook", moltbookHandler.Authenticate)

		// Search endpoint (API-CRITICAL per SPEC.md Part 5.5)
		// GET /v1/search - search the knowledge base (requires auth)
		// Auth is required to prevent abuse/scraping and enable rate limiting per identity
		r.Group(func(r chi.Router) {
			r.Use(auth.UnifiedAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator))
			r.Get("/search", searchHandler.Search)
		})

		// MCP endpoint (MCP-005: HTTP transport for MCP)
		// POST /v1/mcp - Model Context Protocol over HTTP (no auth required for tools/list)
		mcpHandler := handlers.NewMCPHandler(searchRepo, postsRepo)
		r.Post("/mcp", mcpHandler.Handle)

		// Agents list endpoint (API-001)
		// GET /v1/agents - list registered agents (no auth required)
		r.Get("/agents", agentsHandler.ListAgents)

		// Agent profile endpoint (per SPEC.md Part 5.6)
		// GET /v1/agents/{id} - get agent profile (no auth required)
		r.Get("/agents/{id}", func(w http.ResponseWriter, req *http.Request) {
			agentID := chi.URLParam(req, "id")
			agentsHandler.GetAgent(w, req, agentID)
		})

		// Agent activity endpoint (per SPEC.md Part 4.9)
		// GET /v1/agents/{id}/activity - agent activity feed (no auth required)
		r.Get("/agents/{id}/activity", func(w http.ResponseWriter, req *http.Request) {
			agentID := chi.URLParam(req, "id")
			agentsHandler.GetActivity(w, req, agentID)
		})

		// Per prd-v4: GET /v1/users - list all users (no auth required)
		r.Get("/users", usersHandler.ListUsers)

		// User profile endpoint (BE-003)
		// GET /v1/users/{id} - get user profile (no auth required)
		r.Get("/users/{id}", usersHandler.GetUserProfile)

		// Per prd-v4: GET /v1/users/{id}/agents - list agents claimed by user (no auth required)
		r.Get("/users/{id}/agents", usersHandler.GetUserAgents)

		// Per prd-v4: GET /v1/users/{id}/contributions - list user contributions (no auth required)
		r.Get("/users/{id}/contributions", usersHandler.GetUserContributions)

		// Posts endpoints (API-CRITICAL requirement)
		// Per SPEC.md Part 5.6: GET /v1/posts - list posts (no auth required)
		r.Get("/posts", postsHandler.List)
		// Per SPEC.md Part 5.6: GET /v1/posts/:id - single post (no auth required)
		r.Get("/posts/{id}", postsHandler.Get)
		// FE-013: View tracking endpoints
		// POST /v1/posts/:id/view - record a view (optional auth)
		r.Post("/posts/{id}/view", viewsHandler.RecordView)
		// GET /v1/posts/:id/views - get view count (no auth required)
		r.Get("/posts/{id}/views", viewsHandler.GetViewCount)

		// Feed endpoints (per SPEC.md Part 5.6 and FIX-004)
		// GET /v1/feed - recent activity (no auth required)
		r.Get("/feed", feedHandler.Feed)
		// GET /v1/feed/stuck - problems needing help (no auth required)
		r.Get("/feed/stuck", feedHandler.Stuck)
		// GET /v1/feed/unanswered - unanswered questions (no auth required)
		r.Get("/feed/unanswered", feedHandler.Unanswered)

		// Stats endpoints (for frontend dashboard)
		var statsRepo handlers.StatsRepositoryInterface
		if pool != nil {
			statsRepo = db.NewStatsRepository(pool)
		}
		if statsRepo != nil {
			statsHandler := handlers.NewStatsHandler(statsRepo)
			r.Get("/stats", statsHandler.GetStats)
			r.Get("/stats/trending", statsHandler.GetTrending)
			r.Get("/stats/ideas", statsHandler.GetIdeasStats)
			r.Get("/stats/problems", statsHandler.GetProblemsStats)
			r.Get("/stats/questions", statsHandler.GetQuestionsStats)
		}

		// Sitemap endpoint (SEO-URGENT, no auth required)
		// GET /v1/sitemap/urls - returns all indexable content for sitemap generation
		if pool != nil {
			sitemapRepo := db.NewSitemapRepository(pool)
			sitemapHandler := handlers.NewSitemapHandler(sitemapRepo)
			r.Get("/sitemap/urls", sitemapHandler.GetSitemapURLs)
			r.Get("/sitemap/counts", sitemapHandler.GetSitemapCounts)
		}

		// Problems endpoints (API-CRITICAL per PRD-v2)
		// GET /v1/problems - list problems (no auth required)
		r.Get("/problems", problemsHandler.List)
		// GET /v1/problems/:id - single problem (no auth required)
		r.Get("/problems/{id}", problemsHandler.Get)
		// GET /v1/problems/:id/approaches - list approaches (no auth required)
		r.Get("/problems/{id}/approaches", problemsHandler.ListApproaches)
		// GET /v1/problems/:id/export - export problem as markdown (no auth required)
		r.Get("/problems/{id}/export", problemsHandler.Export)

		// Questions endpoints (API-CRITICAL per PRD-v2)
		// GET /v1/questions - list questions (no auth required)
		r.Get("/questions", questionsHandler.List)
		// GET /v1/questions/:id - single question (no auth required)
		r.Get("/questions/{id}", questionsHandler.Get)
		// GET /v1/questions/:id/answers - list answers (no auth required)
		// Per FIX-022: Allow viewing answers before answering
		r.Get("/questions/{id}/answers", questionsHandler.ListAnswers)

		// Ideas endpoints (API-CRITICAL per PRD-v2)
		// GET /v1/ideas - list ideas (no auth required)
		r.Get("/ideas", ideasHandler.List)
		// GET /v1/ideas/:id - single idea (no auth required)
		r.Get("/ideas/{id}", ideasHandler.Get)
		// GET /v1/ideas/:id/responses - list responses (no auth required)
		// Per FIX-024: Allow viewing responses before responding
		r.Get("/ideas/{id}/responses", ideasHandler.ListResponses)

		// Comments endpoints (API-CRITICAL per PRD-v2)
		// GET /v1/{target_type}/{id}/comments - list comments (no auth required)
		// Note: Routes use singular form (approach, answer, response) to match handler expectations
		r.Get("/approaches/{id}/comments", wrapCommentsListWithType(commentsHandler, "approach"))
		r.Get("/answers/{id}/comments", wrapCommentsListWithType(commentsHandler, "answer"))
		r.Get("/responses/{id}/comments", wrapCommentsListWithType(commentsHandler, "response"))
		// FIX-019: GET /v1/posts/{id}/comments - list comments on posts (no auth required)
		r.Get("/posts/{id}/comments", wrapCommentsListWithType(commentsHandler, "post"))

		// Protected posts routes (require authentication)
		// Per FIX-003: Use UnifiedAuthMiddleware so JWT (humans), agent API keys, and user API keys all work
		r.Group(func(r chi.Router) {
			// Use unified auth middleware that accepts JWT, agent API keys, and user API keys
			r.Use(auth.UnifiedAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator))

			// Per SPEC.md Part 5.6: POST /v1/posts - create post (requires auth)
			r.Post("/posts", postsHandler.Create)
			// Per SPEC.md Part 5.6: PATCH /v1/posts/:id - update post (requires auth)
			r.Patch("/posts/{id}", postsHandler.Update)
			// Per SPEC.md Part 5.6: DELETE /v1/posts/:id - delete post (requires auth)
			r.Delete("/posts/{id}", postsHandler.Delete)
			// Per SPEC.md Part 5.6: POST /v1/posts/:id/vote - vote on post (requires auth)
			r.Post("/posts/{id}/vote", postsHandler.Vote)

			// Per prd-v4: PATCH /v1/agents/{id} - update agent profile (requires auth)
			// Works with JWT (human owner) or API key (agent updating itself)
			r.Patch("/agents/{id}", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				agentsHandler.UpdateAgent(w, req, agentID)
			})

			// Per FIX-005: GET /v1/me - current authenticated entity info
			// Works with both JWT (humans) and API key (agents)
			meHandler := handlers.NewMeHandler(oauthConfig, userRepo, agentRepo)
			r.Get("/me", meHandler.Me)

			// BE-003: User profile endpoints
			// PATCH /v1/me - update own profile
			r.Patch("/me", usersHandler.UpdateProfile)
			// GET /v1/me/posts - list own posts
			r.Get("/me/posts", usersHandler.GetMyPosts)
			// GET /v1/me/contributions - list own contributions
			r.Get("/me/contributions", usersHandler.GetMyContributions)

			// Protected problems endpoints (API-CRITICAL per PRD-v2)
			r.Post("/problems", problemsHandler.Create)
			r.Post("/problems/{id}/approaches", problemsHandler.CreateApproach)
			r.Patch("/approaches/{id}", problemsHandler.UpdateApproach)
			r.Post("/approaches/{id}/progress", problemsHandler.AddProgressNote)
			r.Post("/approaches/{id}/verify", problemsHandler.VerifyApproach)

			// Protected questions endpoints (API-CRITICAL per PRD-v2)
			r.Post("/questions", questionsHandler.Create)
			r.Post("/questions/{id}/answers", questionsHandler.CreateAnswer)
			r.Patch("/answers/{id}", questionsHandler.UpdateAnswer)
			r.Delete("/answers/{id}", questionsHandler.DeleteAnswer)
			r.Post("/answers/{id}/vote", questionsHandler.VoteOnAnswer)
			r.Post("/questions/{id}/accept/{aid}", questionsHandler.AcceptAnswer)

			// Protected ideas endpoints (API-CRITICAL per PRD-v2)
			r.Post("/ideas", ideasHandler.Create)
			r.Post("/ideas/{id}/responses", ideasHandler.CreateResponse)
			r.Post("/ideas/{id}/evolve", ideasHandler.Evolve)

			// Protected comments endpoints (API-CRITICAL per PRD-v2)
			r.Post("/approaches/{id}/comments", wrapCommentsCreateWithType(commentsHandler, "approach"))
			r.Post("/answers/{id}/comments", wrapCommentsCreateWithType(commentsHandler, "answer"))
			r.Post("/responses/{id}/comments", wrapCommentsCreateWithType(commentsHandler, "response"))
			// FIX-019: POST /v1/posts/{id}/comments - create comment on posts (requires auth)
			r.Post("/posts/{id}/comments", wrapCommentsCreateWithType(commentsHandler, "post"))
			r.Delete("/comments/{id}", commentsHandler.Delete)

			// Notifications endpoints (API-CRITICAL per PRD-v2)
			// Per SPEC.md Part 5.6: GET /notifications - list notifications
			r.Get("/notifications", notificationsHandler.List)
			// Per SPEC.md Part 5.6: POST /notifications/:id/read - mark notification as read
			r.Post("/notifications/{id}/read", func(w http.ResponseWriter, req *http.Request) {
				// Set the notification ID in the context for the handler
				notificationsHandler.MarkRead(w, req)
			})
			// Per SPEC.md Part 5.6: POST /notifications/read-all - mark all as read
			r.Post("/notifications/read-all", notificationsHandler.MarkAllRead)

			// User API keys endpoints (API-CRITICAL per PRD-v2)
			// Per prd-v2.json: GET /users/me/api-keys - list user's API keys
			r.Get("/users/me/api-keys", userAPIKeysHandler.ListAPIKeys)
			// Per prd-v2.json: POST /users/me/api-keys - create new API key
			r.Post("/users/me/api-keys", userAPIKeysHandler.CreateAPIKey)
			// Per prd-v2.json: DELETE /users/me/api-keys/:id - revoke API key
			r.Delete("/users/me/api-keys/{id}", func(w http.ResponseWriter, req *http.Request) {
				keyID := chi.URLParam(req, "id")
				userAPIKeysHandler.RevokeAPIKey(w, req, keyID)
			})
			// Per prd-v2.json: POST /users/me/api-keys/:id/regenerate - regenerate API key
			r.Post("/users/me/api-keys/{id}/regenerate", func(w http.ResponseWriter, req *http.Request) {
				keyID := chi.URLParam(req, "id")
				userAPIKeysHandler.RegenerateAPIKey(w, req, keyID)
			})

			// Bookmarks endpoints (FE-011)
			// GET /users/me/bookmarks - list user's bookmarks
			r.Get("/users/me/bookmarks", bookmarksHandler.List)
			// POST /users/me/bookmarks - add a bookmark
			r.Post("/users/me/bookmarks", bookmarksHandler.Add)
			// GET /users/me/bookmarks/:id - check if post is bookmarked
			r.Get("/users/me/bookmarks/{id}", bookmarksHandler.Check)
			// DELETE /users/me/bookmarks/:id - remove a bookmark
			r.Delete("/users/me/bookmarks/{id}", bookmarksHandler.Remove)

			// Reports endpoints (FE-018)
			// POST /reports - create a new report (requires auth)
			r.Post("/reports", reportsHandler.Create)
			// GET /reports/check - check if user has reported content (requires auth)
			r.Get("/reports/check", reportsHandler.Check)
		})
	})
}

// loadRateLimitConfig loads rate limit configuration from database with fallback to defaults.
func loadRateLimitConfig(pool *db.Pool) *apimiddleware.RateLimitConfig {
	if pool == nil {
		return apimiddleware.DefaultRateLimitConfig()
	}

	// Load from database
	configRepo := db.NewRateLimitConfigRepository(pool)
	dbConfig := configRepo.LoadConfig(context.Background())

	// Convert to middleware config
	return apimiddleware.RateLimitConfigFromDB(
		dbConfig.AgentGeneralLimit,
		dbConfig.HumanGeneralLimit,
		dbConfig.SearchLimitPerMin,
		dbConfig.AgentPostsPerHour,
		dbConfig.HumanPostsPerHour,
		dbConfig.AgentAnswersPerHour,
		dbConfig.HumanAnswersPerHour,
		dbConfig.NewAccountThresholdHours,
	)
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

// wrapCommentsListWithType wraps the CommentsHandler.List with a target_type param set.
// This is needed because the routes use /approaches/{id}/comments but the handler
// expects a "target_type" URL param with value "approach" (singular).
func wrapCommentsListWithType(h *handlers.CommentsHandler, targetType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Add target_type to chi context so it can be retrieved by chi.URLParam
		rctx := chi.RouteContext(r.Context())
		rctx.URLParams.Add("target_type", targetType)
		h.List(w, r)
	}
}

// wrapCommentsCreateWithType wraps the CommentsHandler.Create with a target_type param set.
func wrapCommentsCreateWithType(h *handlers.CommentsHandler, targetType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		rctx.URLParams.Add("target_type", targetType)
		h.Create(w, r)
	}
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
