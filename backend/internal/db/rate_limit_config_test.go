package db_test

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// TestRateLimitConfig_MeBriefing verifies that the me_briefing config loads
// with a default value of 30 req/min.
// /me is designed to be called every 4 hours per HEARTBEAT.md schedule;
// 30 req/min is 1800x headroom over actual usage.
func TestRateLimitConfig_MeBriefing(t *testing.T) {
	t.Run("default value is 30 req/min", func(t *testing.T) {
		defaults := db.DefaultRateLimitConfigValues()
		if defaults.MeBriefingLimitPerMin != 30 {
			t.Errorf("MeBriefingLimitPerMin default = %d, want 30", defaults.MeBriefingLimitPerMin)
		}
	})

	t.Run("loads from database when available", func(t *testing.T) {
		url := getTestDatabaseURL(t)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pool, err := db.NewPool(ctx, url)
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		defer pool.Close()

		repo := db.NewRateLimitConfigRepository(pool)

		// Insert me_briefing config into rate_limit_config table
		_, _ = pool.Exec(ctx, `DELETE FROM rate_limit_config WHERE key = 'me_briefing_limit_per_min'`)
		_, err = pool.Exec(ctx,
			`INSERT INTO rate_limit_config (key, value, description) VALUES ($1, $2, $3)`,
			"me_briefing_limit_per_min", 45,
			"Agent /me briefing requests per minute",
		)
		if err != nil {
			t.Fatalf("Failed to insert me_briefing config: %v", err)
		}
		defer func() {
			_, _ = pool.Exec(ctx, `DELETE FROM rate_limit_config WHERE key = 'me_briefing_limit_per_min'`)
		}()

		config := repo.LoadConfig(ctx)
		if config.MeBriefingLimitPerMin != 45 {
			t.Errorf("MeBriefingLimitPerMin = %d, want 45 (loaded from DB)", config.MeBriefingLimitPerMin)
		}
	})

	t.Run("falls back to default when not in database", func(t *testing.T) {
		url := getTestDatabaseURL(t)
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		pool, err := db.NewPool(ctx, url)
		if err != nil {
			t.Fatalf("NewPool() error = %v", err)
		}
		defer pool.Close()

		repo := db.NewRateLimitConfigRepository(pool)

		// Ensure the key doesn't exist
		_, _ = pool.Exec(ctx, `DELETE FROM rate_limit_config WHERE key = 'me_briefing_limit_per_min'`)

		config := repo.LoadConfig(ctx)
		if config.MeBriefingLimitPerMin != 30 {
			t.Errorf("MeBriefingLimitPerMin = %d, want 30 (default fallback)", config.MeBriefingLimitPerMin)
		}
	})

	t.Run("nil pool returns default", func(t *testing.T) {
		repo := db.NewRateLimitConfigRepository(nil)
		config := repo.LoadConfig(context.Background())
		if config.MeBriefingLimitPerMin != 30 {
			t.Errorf("MeBriefingLimitPerMin = %d, want 30 (nil pool default)", config.MeBriefingLimitPerMin)
		}
	})
}
