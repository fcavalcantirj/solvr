package api

import (
	"context"
	"net/http"

	apimiddleware "github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/db"
)

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

// createRateLimits returns the per-author hourly create limiters for posts and for
// contributions (anti-abuse W3), counting the authoritative tables. They are mounted on the
// create routes themselves, after authentication, inside any idempotency middleware (a
// replayed 201 never counts). Without a database they pass everything through.
func createRateLimits(pool *db.Pool, cfg *apimiddleware.RateLimitConfig) (posts, contributions func(http.Handler) http.Handler) {
	var counter apimiddleware.CreateCounter
	if pool != nil {
		counter = db.NewCreateCountRepository(pool)
	}
	return apimiddleware.CreateRateLimit(counter, cfg, apimiddleware.CreateOpPosts),
		apimiddleware.CreateRateLimit(counter, cfg, apimiddleware.CreateOpContributions)
}
