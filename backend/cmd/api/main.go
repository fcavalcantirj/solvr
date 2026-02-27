// Package main is the entry point for the Solvr API server.
package main

import (
	"context"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api"
	"github.com/fcavalcantirj/solvr/internal/config"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// translationModTrigger implements jobs.PostModerationTrigger for use by the translation job.
// After a post is translated, it fires a single moderation call to determine if the
// translated content should be approved or rejected.
type translationModTrigger struct {
	modSvc   *services.ContentModerationService
	postRepo *db.PostRepository
}

func (t *translationModTrigger) TriggerAsync(postID, title, description string, tags []string, postType, authorType, authorID string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		result, err := t.modSvc.ModerateContent(ctx, services.ModerationInput{
			Title:       title,
			Description: description,
			Tags:        tags,
		})
		if err != nil {
			log.Printf("translation job: post-translation moderation failed for %s: %v", postID, err)
			return
		}

		status := models.PostStatusRejected
		if result.Approved {
			status = models.PostStatusOpen
		}

		if updateErr := t.postRepo.UpdateStatus(ctx, postID, status); updateErr != nil {
			log.Printf("translation job: failed to update post %s status after moderation: %v", postID, updateErr)
		} else {
			log.Printf("translation job: post %s moderation result approved=%v (language: %s)", postID, result.Approved, result.LanguageDetected)
		}
	}()
}

func main() {
	// Load configuration
	cfg, err := config.Load()
	if err != nil {
		log.Printf("Warning: Configuration incomplete: %v", err)
		// Continue without full config for development
	}

	// Get port from environment or default to 8080
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	// Initialize database pool (optional - server can run without it)
	var pool *db.Pool
	if cfg != nil && cfg.DatabaseURL != "" {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		pool, err = db.NewPool(ctx, cfg.DatabaseURL)
		cancel()
		if err != nil {
			log.Printf("Warning: Database connection failed: %v", err)
			log.Println("Server will start without database (health/ready will return 503)")
		} else {
			log.Println("Database connection established")
			defer pool.Close()
		}
	}

	// Initialize embedding service based on configuration
	var embeddingService services.EmbeddingService
	if cfg != nil {
		provider := cfg.EmbeddingProvider
		if provider == "" {
			provider = "voyage"
		}
		switch provider {
		case "ollama":
			log.Fatal("FATAL: EMBEDDING_PROVIDER=ollama is incompatible with the current database schema (vector(1024)). Ollama nomic-embed-text produces 768-dim vectors. Use EMBEDDING_PROVIDER=voyage or update migration 000044 to vector(768).")
		default:
			if cfg.VoyageAPIKey != "" {
				embeddingService = services.NewVoyageEmbeddingService(cfg.VoyageAPIKey)
				log.Println("Embedding service: voyage")
			} else {
				log.Println("Embedding service: disabled (no VOYAGE_API_KEY)")
			}
		}
	}

	// Create router with database pool and embedding service
	router := api.NewRouter(pool, embeddingService)

	// Note: API routes are now mounted directly in api.NewRouter() via mountV1Routes()
	// The previous call to api.MountAPIRoutes() was removed per FIX-001 because
	// it added placeholder routes that overrode the real handlers.
	// All routes are now consolidated in router.go.

	// Log startup configuration (FIX-014)
	// This provides visibility into what config the server started with
	logger := slog.Default()
	dbConnected := pool != nil
	config.LogStartupConfig(logger, cfg, dbConnected)

	// Start background cleanup job if database is available
	// Per prd-v2.json: "Cron/scheduled job to delete expired tokens, Run every hour"
	var cleanupCancel context.CancelFunc
	if pool != nil {
		var cleanupCtx context.Context
		cleanupCtx, cleanupCancel = context.WithCancel(context.Background())
		tokenRepo := db.NewClaimTokenRepository(pool)
		cleanupJob := jobs.NewCleanupJob(tokenRepo)
		go cleanupJob.RunScheduled(cleanupCtx, jobs.DefaultCleanupInterval)
		log.Println("Cleanup job started (runs every hour)")
	}

	// Start crystallization cron job if database and IPFS are available
	// Per prd-v6: "Create cron job to scan for crystallization candidates daily"
	var crystallizationCancel context.CancelFunc
	if pool != nil {
		ipfsURL := os.Getenv("IPFS_API_URL")
		if ipfsURL == "" {
			ipfsURL = "http://localhost:5001"
		}
		postRepo := db.NewPostRepository(pool)
		approachRepo := db.NewApproachesRepository(pool)
		ipfsSvc := services.NewKuboIPFSService(ipfsURL)
		crystallizationSvc := services.NewCrystallizationService(
			postRepo, postRepo, approachRepo, ipfsSvc, ipfsSvc,
		)
		crystallizationJob := jobs.NewCrystallizationJob(
			postRepo, crystallizationSvc, jobs.DefaultCrystallizationStabilityPeriod,
		)
		var crystallizationCtx context.Context
		crystallizationCtx, crystallizationCancel = context.WithCancel(context.Background())
		go crystallizationJob.RunScheduled(crystallizationCtx, jobs.DefaultCrystallizationInterval)
		log.Println("Crystallization job started (runs every 24 hours)")
	}

	// Start stale content cleanup job if database is available
	// Per prd-v5: abandon stale approaches (30d), warn before abandonment (23d), mark dormant posts (60d)
	var staleContentCancel context.CancelFunc
	if pool != nil {
		notifRepo := db.NewNotificationsRepository(pool)
		staleContentRepo := db.NewStaleContentRepository(pool, notifRepo)
		staleContentJob := jobs.NewStaleContentJob(staleContentRepo, staleContentRepo, staleContentRepo)
		var staleContentCtx context.Context
		staleContentCtx, staleContentCancel = context.WithCancel(context.Background())
		go staleContentJob.RunScheduled(staleContentCtx, jobs.DefaultStaleContentInterval)
		log.Println("Stale content cleanup job started (runs every 24 hours)")
	}

	// Start auto-translation job if database and Groq API key are available.
	// Runs twice daily (every 12 hours) to translate non-English draft posts.
	var translationCancel context.CancelFunc
	if pool != nil && os.Getenv("GROQ_API_KEY") != "" {
		translationSvc := services.NewTranslationService(os.Getenv("GROQ_API_KEY"))
		if model := os.Getenv("TRANSLATION_MODEL"); model != "" {
			translationSvc = services.NewTranslationService(os.Getenv("GROQ_API_KEY"), services.WithTranslationModel(model))
		}
		translationPostRepo := db.NewPostRepository(pool)
		translationModSvc := services.NewContentModerationService(os.Getenv("GROQ_API_KEY"))
		trigger := &translationModTrigger{
			modSvc:   translationModSvc,
			postRepo: translationPostRepo,
		}

		batchSize := jobs.DefaultTranslationBatchSize
		if v := os.Getenv("TRANSLATION_BATCH_SIZE"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n > 0 {
				batchSize = n
			}
		}
		delayMs := jobs.DefaultTranslationDelayMs
		if v := os.Getenv("TRANSLATION_DELAY_MS"); v != "" {
			if n, err := strconv.Atoi(v); err == nil && n >= 0 {
				delayMs = n
			}
		}

		translationJob := jobs.NewTranslationJob(translationPostRepo, translationPostRepo, translationSvc, trigger, batchSize, delayMs)
		var translationCtx context.Context
		translationCtx, translationCancel = context.WithCancel(context.Background())
		go translationJob.RunScheduled(translationCtx, jobs.DefaultTranslationInterval)
		log.Println("Translation job started (runs every 12 hours)")
	}

	// Start health check monitoring job if database is available
	var healthCheckCancel context.CancelFunc
	if pool != nil {
		ipfsURL := os.Getenv("IPFS_API_URL")
		if ipfsURL == "" {
			ipfsURL = "http://localhost:5001"
		}
		checksRepo := db.NewServiceCheckRepository(pool)
		ipfsChecker := services.NewKuboIPFSService(ipfsURL)
		healthSvc := services.NewHealthCheckerService(pool, ipfsChecker)
		healthCheckJob := jobs.NewHealthCheckJob(healthSvc, checksRepo)
		var healthCheckCtx context.Context
		healthCheckCtx, healthCheckCancel = context.WithCancel(context.Background())
		go healthCheckJob.RunScheduled(healthCheckCtx, jobs.DefaultHealthCheckInterval)
		log.Println("Health check monitoring job started (runs every 5 minutes)")
	}

	// Create server
	server := &http.Server{
		Addr:         ":" + port,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Start server in goroutine
	go func() {
		log.Printf("Starting Solvr API server on port %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Stop background jobs if running
	if cleanupCancel != nil {
		cleanupCancel()
	}
	if crystallizationCancel != nil {
		crystallizationCancel()
	}
	if staleContentCancel != nil {
		staleContentCancel()
	}
	if translationCancel != nil {
		translationCancel()
	}
	if healthCheckCancel != nil {
		healthCheckCancel()
	}

	// Create shutdown context with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Attempt graceful shutdown
	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server shutdown failed: %v", err)
	}

	log.Println("Server stopped")
}
