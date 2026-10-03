// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"encoding/json"
	"log"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/config"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// Version is the API version string
const Version = "0.2.0"

// NewRouter creates and configures a new chi router with all middleware.
// The pool parameter is optional - if nil, /health/ready will return 503.
// hubMgr and registry are optional - if nil, room routes are not mounted.
// The embeddingService parameter is optional - if nil, post creation won't generate embeddings.
func NewRouter(pool *db.Pool, hubMgr *hub.HubManager, registry *hub.PresenceRegistry, embeddingService ...services.EmbeddingService) *chi.Mux {
	r := chi.NewRouter()

	// Middleware stack
	r.Use(requestIDMiddleware)
	r.Use(apimiddleware.ErrorEnvelope) // after request ID, outside Recoverer
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)

	// CORS configuration - MUST be early in the chain so error responses include CORS headers
	// Policy lives in cors_policy.go (ALLOWED_ORIGINS env var or defaults).
	r.Use(cors.Handler(corsOptions()))

	// Other middleware after CORS
	r.Use(apimiddleware.Logging)
	r.Use(apimiddleware.BodyLimit(requestBodyLimitBytes)) // FIX-028: 64KB request body limit
	r.Use(securityHeadersMiddleware)
	r.Use(jsonContentTypeMiddleware)

	// Aggregate API usage, measured at the boundary (see middleware.APIUsage
	// and db.AsyncAPIUsageRecorder). It sits AFTER routing has been arranged
	// so it can read the route TEMPLATE the request matched, and it records
	// asynchronously so measuring never slows down what it measures. Without
	// a database there is nowhere to record, and it becomes a pass-through.
	if pool != nil {
		r.Use(apimiddleware.APIUsage(db.NewAsyncAPIUsageRecorder(pool)))
	}

	// Rate limiting - load config from database with fallback to defaults
	rateLimitConfig := loadRateLimitConfig(pool)
	rateLimitStore := apimiddleware.NewInMemoryRateLimitStore()
	rateLimiter := apimiddleware.NewRateLimiter(rateLimitStore, rateLimitConfig)
	r.Use(rateLimiter.Middleware)

	// Custom 404 and 405 handlers for JSON responses
	r.NotFound(notFoundHandler)
	r.MethodNotAllowed(methodNotAllowedHandler)

	// Robots.txt — tell crawlers not to index the API
	r.Get("/robots.txt", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("User-agent: *\nDisallow: /\n"))
	})

	// Health endpoints
	r.Get("/health", healthHandler)
	r.Get("/health/live", healthLiveHandler)
	r.Get("/health/ready", healthReadyHandler(pool))

	// IPFS configuration (shared by health check and pinning service)
	ipfsAPIURL := os.Getenv("IPFS_API_URL")
	if ipfsAPIURL == "" {
		ipfsAPIURL = "http://localhost:5001"
	}

	// IPFS health check (per prd-v6-ipfs-expanded.json)
	// GET /v1/health/ipfs - check IPFS node connectivity (no auth, public monitoring endpoint)
	ipfsHealthSvc := services.NewKuboIPFSServiceWithConfig(ipfsAPIURL, services.IPFSConfig{
		Timeout:    5 * time.Second,
		MaxRetries: 0,
		RetryDelay: 0,
	})
	ipfsHealthAdapter := &ipfsHealthAdapter{ipfs: ipfsHealthSvc}
	ipfsHealthHandler := handlers.NewIPFSHealthHandler(ipfsHealthAdapter)
	r.Get("/v1/health/ipfs", ipfsHealthHandler.Check)

	// Operator endpoints. Everything under /admin is reporting Solvr does about
	// itself — traffic, growth, raw diagnostics — so every one of these routes
	// goes through the server-validated operator gate, which also marks the
	// answer as one caller's private, unstorable response. See
	// handlers/operator_analytics.go; the handlers keep their own key check as
	// the second lock.
	operatorOnly := handlers.RequireOperatorAccess
	adminHandler := handlers.NewAdminHandler(pool)
	r.With(operatorOnly).Post("/admin/query", adminHandler.ExecuteQuery)

	// Admin hard-delete and list deleted (Task 17)
	r.With(operatorOnly).Delete("/admin/users/{id}", adminHandler.HardDeleteUser)
	r.With(operatorOnly).Delete("/admin/agents/{id}", adminHandler.HardDeleteAgent)
	r.With(operatorOnly).Get("/admin/users/deleted", adminHandler.ListDeletedUsers)
	r.With(operatorOnly).Get("/admin/agents/deleted", adminHandler.ListDeletedAgents)

	// Admin manual translation trigger — wire the job if GROQ and DB are available
	if groqKey := os.Getenv("GROQ_API_KEY"); groqKey != "" && pool != nil {
		adminPostRepo := db.NewPostRepository(pool)
		translationSvc := services.NewTranslationService(groqKey)
		modSvc := services.NewContentModerationService(groqKey)
		adminTrigger := handlers.NewModerationTrigger(
			NewContentModerationAdapter(modSvc),
			adminPostRepo,
			slog.Default(),
		)
		translationJob := jobs.NewTranslationJob(adminPostRepo, adminPostRepo, translationSvc, adminTrigger,
			jobs.DefaultTranslationBatchSize, 0)
		adminHandler.SetTranslationJobRunner(translationJob)
	}
	r.With(operatorOnly).Post("/admin/jobs/translation/run", adminHandler.RunTranslationJob)

	// Wire Resend email client and broadcast endpoint if API key is available
	if resendKey := os.Getenv("RESEND_API_KEY"); resendKey != "" {
		fromEmail := os.Getenv("FROM_EMAIL")
		if fromEmail == "" {
			fromEmail = "noreply@solvr.dev"
		}
		resendClient := services.NewResendClient(resendKey, fromEmail)
		adminHandler.SetEmailSender(resendClient)
		slog.Info("Resend email client configured", "from", fromEmail)
	}

	// Wire email broadcast repos (needed even without Resend key for 503 response)
	if pool != nil {
		adminHandler.SetEmailBroadcastRepo(db.NewEmailBroadcastRepository(pool))
		adminHandler.SetUserEmailRepo(db.NewUserRepository(pool))
	}
	r.With(operatorOnly).Post("/admin/email/broadcast", adminHandler.BroadcastEmail)
	mountAbuseAdminRoutes(r, pool, operatorOnly, ipfsAPIURL)
	mountGrowthReportRoutes(r, pool, operatorOnly)
	mountGrowthPlanningRoutes(r, pool, operatorOnly)
	mountOpsRoutes(r, pool, operatorOnly)
	r.With(operatorOnly).Get("/admin/email/history", adminHandler.ListBroadcasts)

	// Admin search analytics endpoints
	if pool != nil {
		saRepo := db.NewSearchAnalyticsRepository(pool)
		saHandler := handlers.NewSearchAnalyticsHandler(saRepo)
		r.With(operatorOnly).Get("/admin/search-analytics/trending", saHandler.GetTrending)
		r.With(operatorOnly).Get("/admin/search-analytics/summary", saHandler.GetSummary)
	}

	// Admin activation analytics: how many rooms were created and activated,
	// milestone latency, and funnel conversion by origin, measured from the
	// connection funnel. Operator-only, like every /admin route.
	if pool != nil {
		activationRepo := db.NewActivationAnalyticsRepository(pool)
		activationHandler := handlers.NewActivationAnalyticsHandler(activationRepo)
		r.With(operatorOnly).Get("/admin/activation-analytics", activationHandler.GetReport)
	}

	// Admin cohort comparison: consistent 7-day and 28-day post-launch windows
	// anchored to one launch timestamp, so the redesign is judged over the same
	// cohorts rather than a moving all-time baseline. Operator-only.
	if pool != nil {
		cohortRepo := db.NewCohortComparisonRepository(pool)
		cohortHandler := handlers.NewCohortComparisonHandler(cohortRepo)
		r.With(operatorOnly).Get("/admin/cohort-comparison", cohortHandler.GetReport)
	}

	// Admin incident management
	if pool != nil {
		incidentRepo := db.NewIncidentRepository(pool)
		incidentAdminHandler := handlers.NewIncidentAdminHandler(incidentRepo)
		r.With(operatorOnly).Post("/admin/incidents", incidentAdminHandler.CreateIncident)
		r.With(operatorOnly).Patch("/admin/incidents/{id}", incidentAdminHandler.UpdateIncidentStatus)
		r.With(operatorOnly).Post("/admin/incidents/{id}/updates", incidentAdminHandler.AddIncidentUpdate)
	}

	// Discovery endpoints (SPEC.md Part 18.3)
	r.Get("/.well-known/ai-agent.json", wellKnownAIAgentHandler)
	r.Get("/v1/openapi.json", openAPIJSONHandler)
	r.Get("/v1/openapi.yaml", openAPIYAMLHandler)

	// Mount v1 API routes
	var embedSvc services.EmbeddingService
	if len(embeddingService) > 0 {
		embedSvc = embeddingService[0]
	}
	postModerator := mountV1Routes(r, pool, ipfsAPIURL, embedSvc)

	// Homepage routes (public proof of the product; see router_homepage.go)
	mountHomepageRoutes(r, pool)
	mountConnectRoutes(r, pool)
	mountFunnelRoutes(r, pool)

	// Room routes (extracted per D-13 to keep router.go under 900 lines)
	if pool != nil && hubMgr != nil {
		jwtSecret := os.Getenv("JWT_SECRET")
		if jwtSecret == "" {
			jwtSecret = "test-jwt-secret-32-chars-long!!"
		}
		agentRepo := db.NewAgentRepository(pool)
		apiKeyValidator := auth.NewAPIKeyValidator(agentRepo).WithOwnerCheck(db.NewUserRepository(pool))
		userAPIKeyRepo := db.NewUserAPIKeyRepository(pool)
		userAPIKeyValidator := auth.NewUserAPIKeyValidator(userAPIKeyRepo)
		accounts := db.NewUserRepository(pool)
		authMW := auth.UnifiedAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts)
		// Optional auth identifies callers without rejecting omitted credentials, so
		// RoomAccessGuard can enforce closed rooms. Presented invalid credentials are 401.
		optionalAuthMW := auth.OptionalAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts)
		mountRoomRoutes(r, pool, hubMgr, registry, authMW, optionalAuthMW, jwtSecret, postModerator)
	}

	return r
}

// mountV1Routes mounts all v1 API routes.
func mountV1Routes(r *chi.Mux, pool *db.Pool, ipfsAPIURL string, embeddingService services.EmbeddingService) handlers.PostModerator {
	limitPosts, limitContributions := createRateLimits(pool, loadRateLimitConfig(pool)) // per-author hourly create limits (W3)
	// Create repositories and handlers
	var agentRepo handlers.AgentRepositoryInterface
	var claimTokenRepo handlers.ClaimTokenRepositoryInterface
	var postsRepo handlers.PostsRepositoryInterface
	var searchRepo handlers.SearchRepositoryInterface
	var userRepo handlers.MeUserRepositoryInterface
	var notificationsRepo handlers.NotificationsRepositoryInterface
	var userAPIKeysRepo handlers.UserAPIKeyRepositoryInterface
	var bookmarksRepo handlers.BookmarksRepositoryInterface
	var viewsRepo handlers.ViewsRepositoryInterface
	var reportsRepo handlers.ReportsRepositoryInterface
	var pinsRepo handlers.PinRepositoryInterface
	// Durable Idempotency-Key store for the create routes (idx 73 step 4).
	var idempotencyStore apimiddleware.IdempotencyStore
	if pool != nil {
		idempotencyStore = db.NewIdempotencyRepository(pool)
	}
	if pool == nil {
		log.Println("WARNING: Database pool is nil. V1 API routes will not be mounted.")
		return nil
	}

	agentRepoConcrete := db.NewCanonicalReputationAgentRepository(pool) // idx 76: canonical reputation
	agentRepo = agentRepoConcrete
	claimTokenRepoConcrete := db.NewClaimTokenRepository(pool)
	claimTokenRepo = claimTokenRepoConcrete
	postsRepo = db.NewPostRepository(pool)
	searchRepo = db.NewSearchRepository(pool)
	userRepo = db.NewCanonicalReputationUserRepository(pool)
	userAPIKeysRepo = db.NewUserAPIKeyRepository(pool)
	bookmarksRepo = db.NewBookmarkRepository(pool)
	viewsRepo = db.NewViewsRepository(pool)
	reportsRepo = db.NewReportsRepository(pool)
	notificationsRepoConcrete := db.NewNotificationsRepository(pool)
	notificationsRepo = notificationsRepoConcrete
	pinsRepoConcrete := db.NewPinRepository(pool)
	pinsRepo = pinsRepoConcrete
	followsRepo := db.NewFollowsRepository(pool)
	storageRepo := db.NewStorageRepository(pool)
	referralRepo := db.NewReferralRepository(pool)
	roomRepo := db.NewRoomRepository(pool)

	agentsHandler := handlers.NewAgentsHandler(agentRepo, "").WithIdentityGate(db.NewBannedIdentityRepository(pool))
	agentsHandler.SetClaimTokenRepository(claimTokenRepo)
	agentsHandler.SetBaseURL("https://solvr.dev")
	// Room repo lets ClaimAgentWithToken give the claiming human owner membership of rooms an agent created
	// while unclaimed (family scope). The full room routes live in mountRoomRoutes.
	agentsHandler.SetRoomRepository(roomRepo)

	// Family-scoped room discovery handler for GET /v1/me/rooms. ListMyRooms only needs
	// the room repo; the other room routes are served by mountRoomRoutes.
	roomDiscoveryHandler := handlers.NewRoomHandler(roomRepo, nil, nil, nil, nil, nil)

	// Create posts handler
	postsHandler := handlers.NewPostsHandler(postsRepo)
	// Gate public publication of a private-room outcome to the room owner (task: room
	// outcome → Post). An ordinary author edit cannot push a private-room outcome public.
	postsHandler.SetRoomPrivacyChecker(db.NewRoomRepository(pool))
	// A published post can display the public rooms started from it (task: post seeds a
	// collaboration). Private rooms are excluded by the repository.
	postRelatedRoomsHandler := handlers.NewPostRelatedRoomsHandler(roomRepo, db.NewPostRepository(pool))
	if embeddingService != nil {
		postsHandler.SetEmbeddingService(embeddingService)
	}
	// Wire content moderation service if GROQ_API_KEY is configured
	if groqAPIKey := os.Getenv("GROQ_API_KEY"); groqAPIKey != "" {
		var modOpts []services.Option
		if groqModel := os.Getenv("GROQ_MODEL"); groqModel != "" {
			modOpts = append(modOpts, services.WithGroqModel(groqModel))
		}
		modSvc := services.NewContentModerationService(groqAPIKey, modOpts...)
		postsHandler.SetContentModerationService(wrapContentModerator(modSvc))
		if pr, ok := postsRepo.(*db.PostRepository); ok {
			postsHandler.SetPostStatusUpdater(pr)
		}
		// Moderation verdicts are recorded as system replies on the post, not legacy comments.
		moderationVerdicts := db.NewModerationReplyWriter(pool)
		postsHandler.SetCommentRepo(moderationVerdicts)
		notifSvc := NewModerationNotificationService(notificationsRepoConcrete.Create)
		postsHandler.SetNotificationService(notifSvc)

		// Wire inline translation trigger for immediate translation on language-only rejection.
		// Reuses the same Groq key and creates a ModerationTrigger for post-translation re-moderation.
		if pr, ok := postsRepo.(*db.PostRepository); ok {
			translationSvc := services.NewTranslationService(groqAPIKey)
			if translationModel := os.Getenv("TRANSLATION_MODEL"); translationModel != "" {
				translationSvc = services.NewTranslationService(groqAPIKey, services.WithTranslationModel(translationModel))
			}
			reModSvc := services.NewContentModerationService(groqAPIKey, modOpts...)
			reModTrigger := handlers.NewModerationTrigger(
				NewContentModerationAdapter(reModSvc),
				pr,
				slog.Default(),
			)
			reModTrigger.SetCommentRepo(moderationVerdicts)
			reModTrigger.SetNotificationService(notifSvc)
			translationTrigger := NewTranslationTriggerAdapter(translationSvc, pr, reModTrigger, slog.Default())
			postsHandler.SetTranslationTrigger(translationTrigger)
		}
	} else {
		slog.Warn("GROQ_API_KEY not set - content moderation disabled, posts created as pending_review without auto-moderation")
	}

	// Create search handler (per SPEC.md Part 5.5)
	// Wire embedding service for hybrid RRF search (full-text + vector similarity)
	if embeddingService != nil {
		if sr, ok := searchRepo.(*db.SearchRepository); ok {
			sr.SetEmbeddingService(embeddingService)
		}
	}
	searchHandler := handlers.NewSearchHandler(searchRepo)

	// BART-155: cosine-similarity bar for meta.confident_match + min_similarity default.
	searchConfidenceThreshold := config.SearchConfidenceThreshold()
	searchHandler.SetConfidenceThreshold(searchConfidenceThreshold)

	// Wire search analytics repository
	searchAnalyticsRepo := db.NewSearchAnalyticsRepository(pool)
	searchHandler.SetAnalyticsRepo(searchAnalyticsRepo)

	// Create user-related handlers (API-CRITICAL per PRD-v2)
	notificationsHandler := handlers.NewNotificationsHandler(notificationsRepo)
	userAPIKeysHandler := handlers.NewUserAPIKeysHandler(userAPIKeysRepo)
	bookmarksHandler := handlers.NewBookmarksHandler(bookmarksRepo)
	viewsHandler := handlers.NewViewsHandler(viewsRepo)
	reportsHandler := handlers.NewReportsHandler(reportsRepo)
	followsHandler := handlers.NewFollowsHandler(followsRepo)

	// Canonical Reply model (BART-585): one create/list/update/delete/vote family
	// serving every contribution — no approach/answer/response/comment choice.
	repliesHandler := handlers.NewRepliesHandler(db.NewReplyRepository(pool))
	if embeddingService != nil {
		repliesHandler.SetEmbeddingService(embeddingService)
	}

	// Create users handler (BE-003: User profile endpoints)
	// Type assertion to get the full interface needed by UsersHandler
	var usersUserRepo handlers.UsersUserRepositoryInterface
	var usersPostRepo handlers.UsersPostRepositoryInterface
	var usersListRepo handlers.UsersUserListRepositoryInterface
	if pool != nil {
		usersUserRepo = db.NewCanonicalReputationUserRepository(pool)
		usersPostRepo = db.NewPostRepository(pool)
		usersListRepo = db.NewCanonicalReputationUserRepository(pool)
	}
	usersHandler := handlers.NewUsersHandler(usersUserRepo, usersPostRepo)
	// Per prd-v4: Set agent repository for GET /v1/users/{id}/agents endpoint
	usersHandler.SetAgentRepository(agentRepo)
	// Per prd-v4: Set user list repository for GET /v1/users endpoint
	usersHandler.SetUserListRepository(usersListRepo)

	// Create IPFS pinning handler (uses ipfsAPIURL passed from NewRouter)
	ipfsService := services.NewKuboIPFSService(ipfsAPIURL)
	pinsHandler := handlers.NewPinsHandler(pinsRepo, ipfsService)
	pinsHandler.SetStorageRepo(storageRepo)
	pinsHandler.SetAgentFinderRepo(agentRepoConcrete)

	// Create checkpoints handler (reuses pin repo, same IPFS service)
	checkpointsHandler := handlers.NewCheckpointsHandler(pinsRepo, ipfsService)
	checkpointsHandler.SetStorageRepo(storageRepo)
	checkpointsHandler.SetAgentFinderRepo(agentRepoConcrete)
	checkpointsHandler.SetAgentRepo(agentRepoConcrete)

	// Create resurrection bundle handler
	resurrectionRepo := db.NewResurrectionRepository(pool)
	resurrectionHandler := handlers.NewResurrectionHandler(
		agentRepoConcrete,
		pinsRepoConcrete,
		resurrectionRepo,
		agentRepoConcrete,
	)
	resurrectionHandler.SetAgentRepo(agentRepoConcrete)

	// Create IPFS upload handler
	// Max upload size: configurable via env, defaults to 100MB
	maxUploadSize := int64(handlers.DefaultMaxUploadSize)
	if maxUploadSizeStr := os.Getenv("MAX_UPLOAD_SIZE_BYTES"); maxUploadSizeStr != "" {
		if parsed, err := strconv.ParseInt(maxUploadSizeStr, 10, 64); err == nil && parsed > 0 {
			maxUploadSize = parsed
		}
	}
	uploadHandler := handlers.NewUploadHandler(ipfsService, maxUploadSize)
	uploadHandler.SetPinRepo(pinsRepo)
	uploadHandler.SetStorageRepo(storageRepo)

	// Create blog handler
	blogHandler := handlers.NewBlogHandler(db.NewBlogPostRepository(pool))
	wireContentGate(pool, postsHandler, repliesHandler, blogHandler)
	wireAntiAbuseModeration(pool, moderationTargets{posts: postsHandler, blog: blogHandler, replies: repliesHandler,
		notify: notificationsRepoConcrete.Create})
	if groqAPIKey := os.Getenv("GROQ_API_KEY"); groqAPIKey != "" {
		modSvc := services.NewContentModerationService(groqAPIKey)
		blogHandler.SetContentModerationService(wrapContentModerator(modSvc))
	}

	// JWT secret for auth middleware - read from env or use test default
	jwtSecret := os.Getenv("JWT_SECRET")
	if jwtSecret == "" {
		jwtSecret = "test-jwt-secret-32-chars-long!!"
	}

	// A claim token is stored as a hash plus a copy sealed under a key derived from this secret,
	// so a repeat request for a claim link gets the same link back and no table holds the token.
	// Every API instance shares JWT_SECRET, so any of them can open it.
	claimTokenRepoConcrete.WithSealSecret(jwtSecret)

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
	var authUserRepo handlers.UserRepositoryForAuth
	var authMethodRepo handlers.AuthMethodRepository
	var authReferralRepo handlers.ReferralRepositoryForAuth
	if pool != nil {
		userRepoForOAuth := db.NewUserRepository(pool)
		authMethodRepoForOAuth := db.NewAuthMethodRepository(pool)
		oauthUserService := services.NewOAuthUserService(userRepoForOAuth, authMethodRepoForOAuth).WithIdentityGate(db.NewBannedIdentityRepository(pool))
		oauthUserAdapter := services.NewOAuthUserServiceAdapter(oauthUserService)
		oauthHandlers = handlers.NewOAuthHandlersWithUserService(oauthConfig, pool, nil, oauthUserAdapter).
			WithLoginCodes(db.NewOAuthLoginCodeRepository(pool)).WithIdentityGate(db.NewBannedIdentityRepository(pool))
		authUserRepo = db.NewUserRepository(pool)
		authMethodRepo = authMethodRepoForOAuth
		authReferralRepo = db.NewReferralRepository(pool)
	} else {
		// Fallback for testing when pool is nil
		oauthHandlers = handlers.NewOAuthHandlers(oauthConfig, pool, nil)
		authMethodRepo = nil   // Will be nil for testing
		authReferralRepo = nil // Will be nil for testing
	}

	// Create API key validator for agent authentication
	// The agentRepo implements auth.AgentDB interface with GetAgentByAPIKeyHash
	accounts := db.NewUserRepository(pool) // a JWT authenticates only while its account is live
	apiKeyValidator := auth.NewAPIKeyValidator(agentRepo).WithOwnerCheck(accounts)

	// Create user API key validator for human programmatic access
	// userAPIKeysRepo implements auth.UserAPIKeyDB interface when backed by db.UserAPIKeyRepository
	var userAPIKeyValidator *auth.UserAPIKeyValidator
	if userAPIKeyDB, ok := userAPIKeysRepo.(auth.UserAPIKeyDB); ok {
		userAPIKeyValidator = auth.NewUserAPIKeyValidator(userAPIKeyDB)
	}

	mcpHandler := handlers.NewMCPHandler(r) // dispatches tool calls through the whole router

	// v1 API routes
	r.Route("/v1", func(r chi.Router) {
		// Agent self-registration (no auth required), limited per client IP (idx 79)
		// Per AGENT-ONBOARDING requirement: POST /v1/agents/register
		r.With(registrationRateLimit()).Post("/agents/register", agentsHandler.RegisterAgent)

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
			r.Use(auth.JWTMiddleware(jwtSecret, accounts))
			r.Post("/agents/claim", agentsHandler.ClaimAgentWithToken)
		})

		// Public claim info endpoint (no auth required)
		// POST /v1/agents/claim/lookup - get claim token info for the confirmation page.
		// The token is in the body: a claim credential must not travel in a URL.
		r.Post("/agents/claim/lookup", agentsHandler.LookupClaim)

		// OAuth endpoints (API-CRITICAL requirement)
		// SECURITY: Wrapped with BlockAgentAPIKeys middleware to prevent agents from
		// registering as humans (see SPEC.md Part 21: Security)
		// Per SPEC.md Part 5.2: GitHub OAuth
		r.With(apimiddleware.BlockAgentAPIKeys).Get("/auth/github", oauthHandlers.GitHubRedirect)
		r.With(apimiddleware.BlockAgentAPIKeys).Get("/auth/github/callback", oauthHandlers.GitHubCallback)

		// Per SPEC.md Part 5.2: Google OAuth
		r.With(apimiddleware.BlockAgentAPIKeys).Get("/auth/google", oauthHandlers.GoogleRedirect)
		r.With(apimiddleware.BlockAgentAPIKeys).Get("/auth/google/callback", oauthHandlers.GoogleCallback)

		// One-time login code -> access token: the OAuth callbacks redirect with a code, never a JWT in a URL.
		r.With(apimiddleware.BlockAgentAPIKeys).Post("/auth/oauth/exchange", oauthHandlers.ExchangeLoginCode)

		// Email/password authentication (API-CRITICAL per PRD Task 48 & 49)
		// SECURITY: Wrapped with BlockAgentAPIKeys middleware to prevent agents from
		// registering as humans (see SPEC.md Part 21: Security)
		authHandler := handlers.NewAuthHandlers(oauthConfig, authUserRepo, authMethodRepo, authReferralRepo).WithIdentityGate(db.NewBannedIdentityRepository(pool))
		r.With(apimiddleware.BlockAgentAPIKeys).Post("/auth/register", authHandler.Register)
		r.With(apimiddleware.BlockAgentAPIKeys).Post("/auth/login", authHandler.Login)
		r.With(auth.JWTMiddleware(jwtSecret, accounts)).Post("/auth/claim-referral", authHandler.ClaimReferral) // OAuth referral attribution (JWT required)

		// Moltbook OAuth (API-CRITICAL per PRD-v2)
		// Per SPEC.md Part 5.2: POST /auth/moltbook for agent authentication via Moltbook
		moltbookConfig := &handlers.MoltbookConfig{
			MoltbookAPIURL: "https://api.moltbook.dev",
		}
		moltbookHandler := handlers.NewMoltbookHandler(moltbookConfig, nil)
		r.Post("/auth/moltbook", moltbookHandler.Authenticate)

		// Search endpoint (API-CRITICAL per SPEC.md Part 5.5)
		// GET /v1/search - search the knowledge base (public access per SPEC.md Part 5.6)
		// OptionalAuth admits omitted credentials; presented invalid credentials are 401.
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts))
			r.Get("/search", searchHandler.Search)
		})

		// MCP endpoint (MCP-005: HTTP transport for MCP)
		// POST /v1/mcp - MCP over HTTP; each tool call runs through this router with the caller's credential.
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

		// GET /v1/agents/{id}/checkpoints - list agent's checkpoints (public read, no auth required)
		r.Get("/agents/{id}/checkpoints", func(w http.ResponseWriter, req *http.Request) {
			agentID := chi.URLParam(req, "id")
			checkpointsHandler.ListCheckpoints(w, req, agentID)
		})

		// GET /v1/agents/{id}/resurrection-bundle - agent rehydration bundle (public read, no auth required)
		r.Get("/agents/{id}/resurrection-bundle", func(w http.ResponseWriter, req *http.Request) {
			agentID := chi.URLParam(req, "id")
			resurrectionHandler.GetBundle(w, req, agentID)
		})

		// Per prd-v4: GET /v1/users - list all users (no auth required)
		r.Get("/users", usersHandler.ListUsers)

		// User profile endpoint (BE-003)
		// GET /v1/users/{id} - get user profile (no auth required)
		r.Get("/users/{id}", usersHandler.GetUserProfile)

		// Per prd-v4: GET /v1/users/{id}/agents - list agents claimed by user (no auth required)
		r.Get("/users/{id}/agents", usersHandler.GetUserAgents)

		// GET /v1/users/{id}/contributions is retired (idx 73): GET /v1/replies lists a user's replies.

		// Per prd-v5: GET /v1/agents/{id}/badges and /v1/users/{id}/badges (no auth required)
		if pool != nil {
			badgeRepo := db.NewBadgeRepository(pool)
			// The owner lookups make an absent owner a 404, as on GET /v1/{agents,users}/{id}.
			badgesHandler := handlers.NewMeHandler(oauthConfig, db.NewCanonicalReputationUserRepository(pool), nil, nil, nil)
			badgesHandler.SetAgentFinderRepo(agentRepoConcrete)
			badgesHandler.SetBadgeRepo(badgeRepo)
			r.Get("/agents/{id}/badges", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				badgesHandler.GetAgentBadges(w, req, agentID)
			})
			r.Get("/users/{id}/badges", func(w http.ResponseWriter, req *http.Request) {
				userID := chi.URLParam(req, "id")
				badgesHandler.GetUserBadges(w, req, userID)
			})
		}

		// Posts endpoints (API-CRITICAL requirement)
		// Per SPEC.md Part 5.6: GET /v1/posts - list posts (no auth required, optional auth for user_vote)
		// OptionalAuth: omitted credentials are anonymous; presented invalid ones are 401.
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts))
			r.Get("/posts", postsHandler.List)
			// Per SPEC.md Part 5.6: GET /v1/posts/:id - single post (no auth required, optional auth for user_vote)
			r.Get("/posts/{id}", postsHandler.Get)
			// The post page's search verdict (task idx 80): 404 exactly when the post read is.
			r.Get("/posts/{id}/seo", postsHandler.GetSEO)
			// Canonical Reply model (BART-585): public reads of a post's replies and a single reply.
			r.Get("/posts/{id}/replies", repliesHandler.List)
			r.Get("/replies/{id}", repliesHandler.Get)
			// One author's replies across posts (idx 73 step 3: replaces the contribution listings).
			r.Get("/replies", repliesHandler.ListByAuthor)
			// A post's related public rooms (post→room half of the two-way link).
			r.Get("/posts/{id}/rooms", postRelatedRoomsHandler.GetRelatedRooms)
			// FE-013: POST /v1/posts/:id/view records a view, GET /v1/posts/:id/views reads the
			// count; both follow the post's visibility for the caller's family.
			r.Post("/posts/{id}/view", viewsHandler.RecordView)
			r.Get("/posts/{id}/views", viewsHandler.GetViewCount)
		})

		// Email unsubscribe — public endpoint, HMAC-signed token validates identity
		if pool != nil {
			unsubHandler := handlers.NewUnsubscribeHandler(db.NewUserRepository(pool), jwtSecret)
			r.Get("/email/unsubscribe", unsubHandler.Unsubscribe)
		}

		// Stats endpoints (for frontend dashboard)
		var statsRepo handlers.StatsRepositoryInterface
		if pool != nil {
			statsRepo = db.NewCanonicalStatsRepository(pool) // idx 76: contributions are replies
		}
		if statsRepo != nil {
			statsHandler := handlers.NewStatsHandler(statsRepo)
			r.Get("/stats", statsHandler.GetStats)
			r.Get("/stats/trending", statsHandler.GetTrending)
			// GET /v1/stats/problems|questions|ideas are retired (task idx 73 step 3,
			// legacy_read_retirement.go): their counts live in GET /v1/overview.
		}
		if pool != nil {
			saRepo := db.NewSearchAnalyticsRepository(pool)
			saHandler := handlers.NewSearchAnalyticsHandler(saRepo)
			r.Get("/stats/search", saHandler.GetPublicSearchStats)
		}

		// Status endpoint (public, no auth required)
		if pool != nil {
			checksRepo := db.NewServiceCheckRepository(pool)
			incidentRepo := db.NewIncidentRepository(pool)
			statusHandler := handlers.NewStatusHandler(checksRepo, incidentRepo)
			r.Get("/status", statusHandler.GetStatus)
		}

		// Sitemap endpoint (SEO-URGENT, no auth required)
		// GET /v1/sitemap/urls - returns all indexable content for sitemap generation
		if pool != nil {
			sitemapRepo := db.NewSitemapRepository(pool)
			sitemapHandler := handlers.NewSitemapHandler(sitemapRepo)
			r.Get("/sitemap/urls", sitemapHandler.GetSitemapURLs)
			r.Get("/sitemap/counts", sitemapHandler.GetSitemapCounts)
		}

		// Public data analytics endpoints (no auth required)
		// GET /v1/data/trending - top trending search queries
		// GET /v1/data/breakdown - agent/human/total search breakdown
		// GET /v1/data/categories - search counts by type_filter category
		if pool != nil {
			dataRepo := db.NewDataAnalyticsRepository(pool)
			dataHandler := handlers.NewDataHandler(dataRepo)
			r.Get("/data/trending", dataHandler.GetTrending)
			r.Get("/data/breakdown", dataHandler.GetBreakdown)
			r.Get("/data/categories", dataHandler.GetCategories)
		}

		// Leaderboard endpoints (PRD-v5)
		// GET /v1/leaderboard - global leaderboard (no auth required)
		// GET /v1/leaderboard/tags/{tag} - tag-specific leaderboard (no auth required)
		if pool != nil {
			// idx 76 (feature:leaderboards): earned reputation frozen at the cutover plus live votes.
			leaderboardRepo := db.NewCanonicalLeaderboardRepository(pool)
			leaderboardHandler := handlers.NewLeaderboardHandler(leaderboardRepo)
			r.Get("/leaderboard", leaderboardHandler.GetLeaderboard)
			r.Get("/leaderboard/tags/{tag}", leaderboardHandler.GetLeaderboardByTag)
		}

		// Blog endpoints (PRD-v5: public reads with optional auth for user_vote)
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts))
			r.Get("/blog", blogHandler.List)
		})
		r.Get("/blog/featured", blogHandler.GetFeatured)
		r.Get("/blog/tags", blogHandler.ListTags)
		r.Group(func(r chi.Router) {
			r.Use(auth.OptionalAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts))
			r.Get("/blog/{slug}", blogHandler.GetBySlug)
		})
		r.Post("/blog/{slug}/view", blogHandler.RecordView)

		// The legacy typed reads (GET /v1/{problems,questions,ideas}, their single-post reads, the
		// problem's approaches, approach history and export, the question's answers, the idea's
		// responses) and the comment lists are retired (idx 73 step 3, legacy_read_retirement.go):
		// GET /v1/posts, GET /v1/posts/{id} and GET /v1/posts/{id}/replies serve them.

		// The legacy WRITE routes (typed creates, approaches, answers, responses, progress
		// notes, comments and the status commands) are retired: 410 ENDPOINT_RETIRED naming
		// the canonical replacement, for every caller (task idx 52, legacy_write_retirement.go).
		mountRetiredLegacyWrites(r)
		// The retired legacy READ routes answer the same error (task idx 73 step 3,
		// legacy_read_retirement.go).
		mountRetiredLegacyReads(r)

		// Protected posts routes (require authentication)
		// Per FIX-003: Use UnifiedAuthMiddleware so JWT (humans), agent API keys, and user API keys all work
		r.Group(func(r chi.Router) {
			// Use unified auth middleware that accepts JWT, agent API keys, and user API keys
			r.Use(auth.UnifiedAuthMiddleware(jwtSecret, apiKeyValidator, userAPIKeyValidator, accounts))

			// Per SPEC.md Part 5.6: POST /v1/posts - create post (requires auth)
			r.With(apimiddleware.Idempotency(idempotencyStore, "post.create"), limitPosts).Post("/posts", postsHandler.Create)
			// Per SPEC.md Part 5.6: PATCH /v1/posts/:id - update post (requires auth)
			r.Patch("/posts/{id}", postsHandler.Update)
			// Per SPEC.md Part 5.6: DELETE /v1/posts/:id - delete post (requires auth)
			r.Delete("/posts/{id}", postsHandler.Delete)
			// Per SPEC.md Part 5.6: POST /v1/posts/:id/vote - vote on post (requires auth)
			r.Post("/posts/{id}/vote", postsHandler.Vote)
			// GET /v1/posts/:id/my-vote - get current user's vote on a post (requires auth)
			r.Get("/posts/{id}/my-vote", postsHandler.GetMyVote)

			// Canonical Reply model (BART-585): one write path for every contribution.
			r.With(apimiddleware.Idempotency(idempotencyStore, "reply.create"), limitContributions).Post("/posts/{id}/replies", repliesHandler.Create)
			r.Patch("/replies/{id}", repliesHandler.Update)
			r.Delete("/replies/{id}", repliesHandler.Delete)
			r.Post("/replies/{id}/vote", repliesHandler.Vote)

			// Blog write endpoints (PRD-v5: authenticated writes)
			r.With(limitPosts).Post("/blog", blogHandler.Create)
			r.Patch("/blog/{slug}", blogHandler.Update)
			r.Delete("/blog/{slug}", blogHandler.Delete)
			r.Post("/blog/{slug}/vote", blogHandler.Vote)

			// Per prd-v4: PATCH /v1/agents/{id} - update agent profile (requires auth)
			// Works with JWT (human owner) or API key (agent updating itself)
			r.Patch("/agents/{id}", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				agentsHandler.UpdateAgent(w, req, agentID)
			})

			// Per SPEC.md Part 5.6: POST /v1/agents/{id}/api-key - rotate agent API key.
			// Human owner only: the handler requires JWT/user-key claims and verifies
			// ownership, so an agent's own API key is rejected (rotation authority stays
			// with the human owner). Returns a fresh key once; the old key dies immediately.
			r.Post("/agents/{id}/api-key", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				agentsHandler.RegenerateAPIKey(w, req, agentID)
			})

			// SPEC.md Part 12.3: the agent's webhooks, managed by the agent or its owner.
			mountWebhookRoutes(r, pool)

			// PRD-v5 Task 22: DELETE /v1/agents/me - agent self-deletion
			// Requires API key auth (agents only, not humans with JWT)
			r.Delete("/agents/me", agentsHandler.DeleteMe)

			// KERI identity management: PATCH /v1/agents/me/identity
			// Agent-only endpoint for updating amcp_aid and keri_public_key
			r.Patch("/agents/me/identity", agentsHandler.UpdateIdentity)

			// Per FIX-005: GET /v1/me - current authenticated entity info
			// Works with both JWT (humans) and API key (agents)
			meHandler := handlers.NewMeHandler(oauthConfig, userRepo, agentRepo, authMethodRepo, pool)
			// Task idx 76 step 3: every briefing section reads canonical posts, replies, votes
			// and room outcomes.
			briefingRepo := db.NewCanonicalBriefingRepository(pool)
			platformRepo := db.NewCanonicalPlatformBriefingRepository(pool)
			briefingSvc := services.NewBriefingServiceWithDeps(services.BriefingDeps{
				InboxRepo:               notificationsRepoConcrete,
				OpenItemsRepo:           briefingRepo,
				SuggestedActionsRepo:    briefingRepo,
				OpportunitiesRepo:       briefingRepo,
				ReputationRepo:          briefingRepo,
				AgentRepo:               agentRepoConcrete,
				PlatformPulseRepo:       platformRepo,
				TrendingRepo:            platformRepo,
				HardcoreRepo:            platformRepo,
				RisingIdeasRepo:         platformRepo,
				VictoriesRepo:           platformRepo,
				RecommendationsRepo:     db.NewCanonicalRecommendationRepository(pool),
				InferredSpecialtiesRepo: db.NewCanonicalInferredSpecialtiesRepository(pool),
				CrystallizationsRepo:    briefingRepo,
				CheckpointFinder:        pinsRepoConcrete,
			})
			meHandler.SetBriefingService(briefingSvc)
			meHandler.SetAgentFinderRepo(agentRepoConcrete)
			meHandler.SetBadgeRepo(db.NewBadgeRepository(pool))
			r.Get("/me", meHandler.Me)
			r.Get("/me/auth-methods", meHandler.GetMyAuthMethods)

			// GET /v1/me/diff - delta-only polling for efficient agent check-ins
			diffRepo := db.NewBriefingDiffRepository(pool)
			meDiffHandler := handlers.NewMeDiffHandler(
				diffRepo,          // DiffNotificationsRepo
				briefingRepo,      // BriefingReputationRepo (reuse existing)
				diffRepo,          // DiffOpportunitiesRepo
				diffRepo,          // DiffBadgesRepo
				agentRepoConcrete, // DiffAgentUpdater (UpdateLastSeen)
				diffRepo,          // DiffTrendingRepo
			)
			r.Get("/me/diff", meDiffHandler.GetDiff)

			// GET /v1/agents/{id}/briefing - agent briefing for human owners or agent self
			r.Get("/agents/{id}/briefing", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				meHandler.GetAgentBriefing(w, req, agentID)
			})
			r.Delete("/me", meHandler.DeleteMe) // PRD-v5 Task 12: User self-deletion

			// Per prd-v6-ipfs-expanded Phase 2: GET /v1/me/storage - storage usage
			storageHandler := handlers.NewStorageHandler(storageRepo)
			storageHandler.SetAgentFinderRepo(agentRepoConcrete)
			r.Get("/me/storage", storageHandler.GetStorage)

			// GET /v1/agents/{id}/pins - agent pins for human owners or agent self
			r.Get("/agents/{id}/pins", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				pinsHandler.ListAgentPins(w, req, agentID)
			})

			// AMCP Checkpoint endpoints
			// POST /v1/agents/me/checkpoints - create checkpoint (agent API key only)
			r.Post("/agents/me/checkpoints", checkpointsHandler.Create)
			// GET /v1/agents/{id}/checkpoints and /resurrection-bundle are in the public group above

			// GET /v1/agents/{id}/storage - agent storage for human owners or agent self
			r.Get("/agents/{id}/storage", func(w http.ResponseWriter, req *http.Request) {
				agentID := chi.URLParam(req, "id")
				storageHandler.GetAgentStorage(w, req, agentID)
			})

			// Heartbeat endpoint — agent/user check-in with aggregated status
			heartbeatHandler := handlers.NewHeartbeatHandler(agentRepo, notificationsRepo, storageRepo)
			heartbeatHandler.SetCheckpointFinder(pinsRepoConcrete)
			if pr, ok := postsRepo.(handlers.HeartbeatPostRepo); ok {
				heartbeatHandler.SetPostRepo(pr)
			}
			r.Get("/heartbeat", heartbeatHandler.Heartbeat)

			// BE-003: User profile endpoints
			// PATCH /v1/me - update own profile
			r.Patch("/me", usersHandler.UpdateProfile)
			// GET /v1/me/posts - list own posts
			r.Get("/me/posts", usersHandler.GetMyPosts)
			// GET /v1/me/rooms - family-scoped room discovery (rooms owned by the caller's
			// human, INCLUDING private rooms) so agents can find sibling rooms.
			r.Get("/me/rooms", roomDiscoveryHandler.ListMyRooms)
			// GET /v1/me/contributions is retired with GET /v1/users/{id}/contributions.

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
			// DELETE /notifications/{id} - delete a single notification
			r.Delete("/notifications/{id}", notificationsHandler.Delete)
			// DELETE /notifications - bulk delete all read notifications
			r.Delete("/notifications", notificationsHandler.DeleteAllRead)

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

			// Referral endpoint (REF-04)
			// GET /v1/users/me/referral — returns user's referral code and count
			referralHandler := handlers.NewReferralHandler(referralRepo)
			r.Get("/users/me/referral", referralHandler.GetMyReferral)

			// Reports endpoints (FE-018)
			// POST /reports - create a new report (requires auth)
			r.Post("/reports", reportsHandler.Create)
			// GET /reports/check - check if user has reported content (requires auth)
			r.Get("/reports/check", reportsHandler.Check)

			// Follows endpoints (PRD-v5: social graph)
			// POST /follow - follow an entity (requires auth)
			r.Post("/follow", followsHandler.Follow)
			// DELETE /follow - unfollow an entity (requires auth)
			r.Delete("/follow", followsHandler.Unfollow)
			// GET /following - list entities the caller follows (requires auth)
			r.Get("/following", followsHandler.ListFollowing)
			// GET /followers - list entities following the caller (requires auth)
			r.Get("/followers", followsHandler.ListFollowers)

			// IPFS Pinning Service API endpoints (per prd-v6-ipfs-expanded.json)
			// Follows IPFS Pinning Service API spec for interoperability
			// POST /v1/pins - create a pin request (async IPFS pin)
			r.Post("/pins", pinsHandler.Create)
			// GET /v1/pins - list user's pins with filters
			r.Get("/pins", pinsHandler.List)
			// GET /v1/pins/:requestid - check pin status by request ID
			r.Get("/pins/{requestid}", pinsHandler.GetByRequestID)
			// DELETE /v1/pins/:requestid - unpin content (async IPFS unpin)
			r.Delete("/pins/{requestid}", pinsHandler.Delete)

			// IPFS content upload endpoint (per prd-v6-ipfs-expanded.json)
			// POST /v1/add - upload content to IPFS and return CID (does NOT auto-pin)
			r.Post("/add", uploadHandler.AddContent)
		})
	})
	return postsHandler.StartModeration // room publication hands approved outcomes to moderation
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

// ipfsHealthAdapter wraps KuboIPFSService to satisfy handlers.IPFSHealthChecker.
type ipfsHealthAdapter struct {
	ipfs *services.KuboIPFSService
}

func (a *ipfsHealthAdapter) NodeInfo(ctx context.Context) (*handlers.IPFSNodeInfo, error) {
	result, err := a.ipfs.NodeInfo(ctx)
	if err != nil {
		return nil, err
	}
	return &handlers.IPFSNodeInfo{
		PeerID:          result.PeerID,
		AgentVersion:    result.AgentVersion,
		ProtocolVersion: result.ProtocolVersion,
	}, nil
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
