// Package db provides database operations for the Solvr API.
package db

import (
	"context"
	"log/slog"
	"time"
)

// RateLimitConfigRepository provides access to rate limit configuration.
type RateLimitConfigRepository struct {
	pool *Pool
}

// NewRateLimitConfigRepository creates a new RateLimitConfigRepository.
func NewRateLimitConfigRepository(pool *Pool) *RateLimitConfigRepository {
	return &RateLimitConfigRepository{pool: pool}
}

// RateLimitConfigValues holds all rate limit configuration values.
type RateLimitConfigValues struct {
	AgentGeneralLimit       int
	HumanGeneralLimit       int
	SearchLimitPerMin       int
	AgentPostsPerHour       int
	HumanPostsPerHour       int
	AgentAnswersPerHour     int
	HumanAnswersPerHour     int
	NewAccountThresholdHours int
}

// DefaultRateLimitConfigValues returns hardcoded fallback values.
func DefaultRateLimitConfigValues() *RateLimitConfigValues {
	return &RateLimitConfigValues{
		AgentGeneralLimit:       60,
		HumanGeneralLimit:       30,
		SearchLimitPerMin:       30,
		AgentPostsPerHour:       5,
		HumanPostsPerHour:       3,
		AgentAnswersPerHour:     15,
		HumanAnswersPerHour:     10,
		NewAccountThresholdHours: 24,
	}
}

// LoadConfig loads rate limit configuration from the database.
// Falls back to defaults if database is unavailable or values are missing.
func (r *RateLimitConfigRepository) LoadConfig(ctx context.Context) *RateLimitConfigValues {
	config := DefaultRateLimitConfigValues()

	if r.pool == nil {
		slog.Warn("Rate limit config: database not available, using defaults")
		return config
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	rows, err := r.pool.Query(ctx, "SELECT key, value FROM rate_limit_config")
	if err != nil {
		slog.Warn("Rate limit config: failed to load from database, using defaults", "error", err)
		return config
	}
	defer rows.Close()

	for rows.Next() {
		var key string
		var value int
		if err := rows.Scan(&key, &value); err != nil {
			continue
		}

		switch key {
		case "agent_general_limit":
			config.AgentGeneralLimit = value
		case "human_general_limit":
			config.HumanGeneralLimit = value
		case "search_limit_per_min":
			config.SearchLimitPerMin = value
		case "agent_posts_per_hour":
			config.AgentPostsPerHour = value
		case "human_posts_per_hour":
			config.HumanPostsPerHour = value
		case "agent_answers_per_hour":
			config.AgentAnswersPerHour = value
		case "human_answers_per_hour":
			config.HumanAnswersPerHour = value
		case "new_account_threshold_hours":
			config.NewAccountThresholdHours = value
		}
	}

	slog.Info("Rate limit config loaded from database",
		"agent_general", config.AgentGeneralLimit,
		"human_general", config.HumanGeneralLimit,
		"search", config.SearchLimitPerMin,
		"agent_posts", config.AgentPostsPerHour,
		"human_posts", config.HumanPostsPerHour,
	)

	return config
}

// GetValue gets a single config value by key.
func (r *RateLimitConfigRepository) GetValue(ctx context.Context, key string, defaultValue int) int {
	if r.pool == nil {
		return defaultValue
	}

	var value int
	err := r.pool.QueryRow(ctx, "SELECT value FROM rate_limit_config WHERE key = $1", key).Scan(&value)
	if err != nil {
		return defaultValue
	}
	return value
}

// SetValue updates a config value (for admin use).
func (r *RateLimitConfigRepository) SetValue(ctx context.Context, key string, value int) error {
	_, err := r.pool.Exec(ctx,
		"UPDATE rate_limit_config SET value = $1, updated_at = NOW() WHERE key = $2",
		value, key)
	return err
}
