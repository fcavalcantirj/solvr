// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/hub"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/requestid"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// Version is the API version string
const Version = "0.2.0"

// NewRouter creates and configures a new chi router with all middleware.
// The pool parameter is optional - if nil, /health/ready will return 503.
// hubMgr and registry are optional - if nil, room routes are not mounted.
// The embeddingService parameter is optional - if nil, post creation won't generate embeddings.
func NewRouter(pool *db.Pool, hubMgr *hub.HubManager, registry *hub.PresenceRegistry, embeddingService ...services.EmbeddingService) *chi.Mux {
	clientIPMiddleware, err := apimiddleware.NewClientIPMiddleware(os.Getenv("TRUSTED_PROXY_CIDRS"))
	if err != nil {
		// Refuse startup rather than silently trusting an invalid boundary.
		panic(err)
	}
	r := chi.NewRouter()

	// Middleware stack
	r.Use(requestIDMiddleware)
	r.Use(apimiddleware.ErrorEnvelope) // after request ID, outside Recoverer
	r.Use(clientIPMiddleware)          // Normalize the verified client once, before logging and limits.
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

	// The two Resend senders (SPEC.md 16.6): broadcasts from FROM_EMAIL (replies to
	// BROADCAST_REPLY_TO), notices only from FROM_EMAIL_TRANSACTIONAL. A missing transactional
	// address leaves no notice sender, so notices are refused (fail closed) and this is logged at start.
	broadcastSender, noticeSender, emailGap := emailSendersFromEnv(os.Getenv)
	if broadcastSender != nil {
		adminHandler.SetEmailSender(broadcastSender)
		replyToSet := false
		if s, ok := broadcastSender.(interface{ ReplyTo() string }); ok {
			replyToSet = s.ReplyTo() != ""
		}
		slog.Info("Resend email client configured", "transactional_sender", noticeSender != nil,
			"broadcast_reply_to", replyToSet)
	}
	if noticeSender != nil {
		adminHandler.SetNoticeSender(noticeSender)
	}
	if emailGap != "" && broadcastSender != nil {
		slog.Error("transactional email is refused until it is configured", "missing", emailGap)
	}

	// Wire email broadcast repos (needed even without Resend key for 503 response)
	if pool != nil {
		adminHandler.SetEmailBroadcastRepo(db.NewEmailBroadcastRepository(pool))
		adminHandler.SetUserEmailRepo(db.NewUserRepository(pool))
		delivery := db.NewEmailDeliveryRepository(pool)
		adminHandler.SetEmailMessageRecorder(delivery)
		adminHandler.SetNoticeRecipientRepo(delivery)
	}
	r.With(operatorOnly).Post("/admin/email/broadcast", adminHandler.BroadcastEmail)
	r.With(operatorOnly).Post("/admin/email/notice", adminHandler.SendNotice)
	mountAbuseAdminRoutes(r, pool, operatorOnly, ipfsAPIURL)
	mountGrowthReportRoutes(r, pool, operatorOnly)
	mountSEOReportRoutes(r, pool, operatorOnly)
	mountGrowthPlanningRoutes(r, pool, operatorOnly)
	mountFeaturedRoomsAdminRoutes(r, pool, operatorOnly)
	mountOpsRoutes(r, pool, operatorOnly)
	mountTrafficRoutes(r, pool, operatorOnly)
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

// requestIDMiddleware adds a unique request ID to each request
// requestIDMiddleware resolves the request's one id (SPEC.md 17.3): a safe client X-Request-ID,
// or a new one. It answers it in the response header, and puts it in the request's header and
// context, so the Logging and APIUsage middlewares, the db layer's log lines and the error
// envelope all name the same id.
func requestIDMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestID := requestid.Resolve(r.Header.Get("X-Request-ID"))
		r.Header.Set("X-Request-ID", requestID)
		w.Header().Set("X-Request-ID", requestID)
		next.ServeHTTP(w, r.WithContext(requestid.With(r.Context(), requestID)))
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
