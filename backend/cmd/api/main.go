// Package main is the entry point for the Solvr API server.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api"
	"github.com/fcavalcantirj/solvr/internal/config"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
)

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

	// Create router with database pool
	router := api.NewRouter(pool)

	// Mount API routes if database and config are available
	if pool != nil && cfg != nil {
		jwtSecret := cfg.JWTSecret
		if jwtSecret == "" {
			jwtSecret = "dev-secret-change-in-production" // fallback for dev
		}
		api.MountAPIRoutes(router, pool, jwtSecret)
		log.Println("API routes mounted")
	}

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

	// Stop the cleanup job if running
	if cleanupCancel != nil {
		cleanupCancel()
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
