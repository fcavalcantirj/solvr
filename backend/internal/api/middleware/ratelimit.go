// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// RateLimitConfig holds configuration for rate limiting per SPEC.md Part 5.6.
type RateLimitConfig struct {
	// General request limits
	AgentGeneralLimit int           // 120 requests/minute for agents
	HumanGeneralLimit int           // 60 requests/minute for humans
	GeneralWindow     time.Duration // Window for general limits

	// Search limits
	SearchLimitPerMin int // 60 searches/minute for agents

	// Post creation limits
	AgentPostsPerHour int // 10 posts/hour for agents
	HumanPostsPerHour int // 5 posts/hour for humans
	PostsWindow       time.Duration

	// Answer limits
	AgentAnswersPerHour int // 30 answers/hour for agents
	HumanAnswersPerHour int // 20 answers/hour for humans
	AnswersWindow       time.Duration

	// New account restrictions
	NewAccountThreshold time.Duration // 24 hours - accounts younger get 50% limits

	// API Key rate limiting (per key instead of per user)
	APIKeyDefaultLimit int            // Default limit for API keys (uses human limit if 0)
	APIKeyTierLimits   map[string]int // Tier-specific limits (e.g., "premium": 180)
}

// DefaultRateLimitConfig returns the default rate limit configuration per SPEC.md Part 5.6.
// These are fallback values if database config is unavailable.
func DefaultRateLimitConfig() *RateLimitConfig {
	return &RateLimitConfig{
		AgentGeneralLimit: 60,  // Tighter for launch (was 120)
		HumanGeneralLimit: 30,  // Tighter for launch (was 60)
		GeneralWindow:     time.Minute,

		SearchLimitPerMin: 30,  // Tighter for launch (was 60)

		AgentPostsPerHour: 5,   // Tighter for launch (was 10)
		HumanPostsPerHour: 3,   // Tighter for launch (was 5)
		PostsWindow:       time.Hour,

		AgentAnswersPerHour: 15,  // Tighter for launch (was 30)
		HumanAnswersPerHour: 10,  // Tighter for launch (was 20)
		AnswersWindow:       time.Hour,

		NewAccountThreshold: 24 * time.Hour,
	}
}

// RateLimitConfigFromDB creates a RateLimitConfig from database values.
// Use this to load dynamic config from the rate_limit_config table.
func RateLimitConfigFromDB(
	agentGeneral, humanGeneral, searchPerMin,
	agentPosts, humanPosts, agentAnswers, humanAnswers,
	newAccountHours int,
) *RateLimitConfig {
	return &RateLimitConfig{
		AgentGeneralLimit: agentGeneral,
		HumanGeneralLimit: humanGeneral,
		GeneralWindow:     time.Minute,

		SearchLimitPerMin: searchPerMin,

		AgentPostsPerHour: agentPosts,
		HumanPostsPerHour: humanPosts,
		PostsWindow:       time.Hour,

		AgentAnswersPerHour: agentAnswers,
		HumanAnswersPerHour: humanAnswers,
		AnswersWindow:       time.Hour,

		NewAccountThreshold: time.Duration(newAccountHours) * time.Hour,
	}
}

// RateLimitRecord represents a rate limit record from the database.
type RateLimitRecord struct {
	Key         string
	Count       int
	WindowStart time.Time
}

// RateLimitStore defines the interface for rate limit storage.
type RateLimitStore interface {
	// GetRecord retrieves a rate limit record by key.
	GetRecord(ctx context.Context, key string) (*RateLimitRecord, error)
	// IncrementAndGet increments the count and returns the updated record.
	// If the window has expired, it starts a new window.
	IncrementAndGet(ctx context.Context, key string, window time.Duration) (*RateLimitRecord, error)
}

// RateLimiter implements rate limiting middleware.
type RateLimiter struct {
	store  RateLimitStore
	config *RateLimitConfig
}

// NewRateLimiter creates a new RateLimiter with the given store and config.
func NewRateLimiter(store RateLimitStore, config *RateLimitConfig) *RateLimiter {
	return &RateLimiter{
		store:  store,
		config: config,
	}
}

// Middleware returns HTTP middleware that enforces rate limits.
func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Get identity from context (including API key info)
		identity := rl.getIdentityInfo(r)

		// If no identity, allow through (auth middleware will handle)
		if identity.Identifier == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Detect operation type
		operation := DetectOperation(r)

		// Get the applicable limit and window
		limit, window := rl.getLimitAndWindowWithAPIKey(identity, operation)

		// Generate the rate limit key (uses API key ID if present)
		key := GenerateRateLimitKeyWithAPIKey(identity.IsAgent, identity.Identifier, identity.APIKeyID, operation)

		// Increment and check the limit
		record, err := rl.store.IncrementAndGet(r.Context(), key, window)
		if err != nil {
			// On error, allow request through (fail open)
			next.ServeHTTP(w, r)
			return
		}

		// Calculate reset time
		resetTime := record.WindowStart.Add(window)

		// Set rate limit headers
		w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limit))
		remaining := limit - record.Count
		if remaining < 0 {
			remaining = 0
		}
		w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		w.Header().Set("X-RateLimit-Reset", strconv.FormatInt(resetTime.Unix(), 10))

		// Check if rate limited
		if record.Count > limit {
			rl.writeRateLimitError(w, resetTime)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// IdentityInfo holds identity information for rate limiting.
type IdentityInfo struct {
	IsAgent    bool
	Identifier string    // User ID or Agent ID
	CreatedAt  time.Time // For new account restrictions
	APIKeyID   string    // API Key ID (for per-key rate limiting)
	APIKeyTier string    // API Key tier (for tiered rate limits)
}

// getIdentity extracts identity information from the request context.
// Returns (isAgent, identifier, createdAt).
// Deprecated: Use getIdentityInfo instead.
func (rl *RateLimiter) getIdentity(r *http.Request) (bool, string, time.Time) {
	// Check for agent first
	agent := auth.AgentFromContext(r.Context())
	if agent != nil {
		return true, agent.ID, agent.CreatedAt
	}

	// Check for human (JWT claims)
	claims := auth.ClaimsFromContext(r.Context())
	if claims != nil {
		return false, claims.UserID, time.Time{} // CreatedAt not available in claims
	}

	return false, "", time.Time{}
}

// getIdentityInfo extracts full identity information including API key info.
func (rl *RateLimiter) getIdentityInfo(r *http.Request) IdentityInfo {
	ctx := r.Context()

	// Check for agent first
	agent := auth.AgentFromContext(ctx)
	if agent != nil {
		return IdentityInfo{
			IsAgent:    true,
			Identifier: agent.ID,
			CreatedAt:  agent.CreatedAt,
		}
	}

	// Check for human (JWT claims)
	claims := auth.ClaimsFromContext(ctx)
	if claims != nil {
		return IdentityInfo{
			IsAgent:    false,
			Identifier: claims.UserID,
			APIKeyID:   auth.APIKeyIDFromContext(ctx),
			APIKeyTier: auth.APIKeyTierFromContext(ctx),
		}
	}

	return IdentityInfo{}
}

// getLimitAndWindow returns the rate limit and window for the given operation.
func (rl *RateLimiter) getLimitAndWindow(isAgent bool, operation string, createdAt time.Time) (int, time.Duration) {
	var limit int
	var window time.Duration

	switch operation {
	case "search":
		limit = rl.config.SearchLimitPerMin
		window = time.Minute
	case "posts":
		if isAgent {
			limit = rl.config.AgentPostsPerHour
		} else {
			limit = rl.config.HumanPostsPerHour
		}
		window = rl.config.PostsWindow
	case "answers":
		if isAgent {
			limit = rl.config.AgentAnswersPerHour
		} else {
			limit = rl.config.HumanAnswersPerHour
		}
		window = rl.config.AnswersWindow
	default: // "general"
		if isAgent {
			limit = rl.config.AgentGeneralLimit
		} else {
			limit = rl.config.HumanGeneralLimit
		}
		window = rl.config.GeneralWindow
	}

	// Apply new account restriction (50% limit for accounts < 24h old)
	if !createdAt.IsZero() && time.Since(createdAt) < rl.config.NewAccountThreshold {
		limit = limit / 2
	}

	return limit, window
}

// getLimitAndWindowWithAPIKey returns the rate limit and window, considering API key tiers.
func (rl *RateLimiter) getLimitAndWindowWithAPIKey(identity IdentityInfo, operation string) (int, time.Duration) {
	// For agents, use standard logic (no API key tiers for agents)
	if identity.IsAgent {
		return rl.getLimitAndWindow(true, operation, identity.CreatedAt)
	}

	// For humans with API key, check for tier-specific limits
	if identity.APIKeyID != "" && identity.APIKeyTier != "" && rl.config.APIKeyTierLimits != nil {
		if tierLimit, ok := rl.config.APIKeyTierLimits[identity.APIKeyTier]; ok {
			// Use tier-specific limit with standard window
			return tierLimit, rl.config.GeneralWindow
		}
	}

	// Use API key default limit if configured
	if identity.APIKeyID != "" && rl.config.APIKeyDefaultLimit > 0 {
		return rl.config.APIKeyDefaultLimit, rl.config.GeneralWindow
	}

	// Fall back to standard human limits
	return rl.getLimitAndWindow(false, operation, identity.CreatedAt)
}

// writeRateLimitError writes a 429 Too Many Requests response.
func (rl *RateLimiter) writeRateLimitError(w http.ResponseWriter, resetTime time.Time) {
	// Calculate Retry-After in seconds
	retryAfter := int(time.Until(resetTime).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "RATE_LIMITED",
			"message": "too many requests, please slow down",
		},
	}

	json.NewEncoder(w).Encode(response)
}

// GenerateRateLimitKey generates a unique key for rate limiting.
// Format: "{type}:{identifier}:{operation}"
// Example: "agent:my-agent:general" or "human:user-uuid:search"
func GenerateRateLimitKey(isAgent bool, identifier, operation string) string {
	entityType := "human"
	if isAgent {
		entityType = "agent"
	}
	return fmt.Sprintf("%s:%s:%s", entityType, identifier, operation)
}

// GenerateRateLimitKeyWithAPIKey generates a rate limit key, using API key ID if present.
// When an API key ID is provided, the key is based on the API key (per-key rate limiting).
// Otherwise, falls back to user/agent-based rate limiting.
// Format with API key: "apikey:{apiKeyID}:{operation}"
// Format without: "{type}:{identifier}:{operation}"
func GenerateRateLimitKeyWithAPIKey(isAgent bool, identifier, apiKeyID, operation string) string {
	// If API key ID is present, use per-key rate limiting
	if apiKeyID != "" {
		return fmt.Sprintf("apikey:%s:%s", apiKeyID, operation)
	}
	// Fall back to entity-based rate limiting
	return GenerateRateLimitKey(isAgent, identifier, operation)
}

// DetectOperation determines the operation type from the request.
// Returns: "general", "search", "posts", or "answers"
func DetectOperation(r *http.Request) string {
	path := r.URL.Path

	// Search detection
	if strings.HasPrefix(path, "/v1/search") {
		return "search"
	}

	// Post creation detection (POST to posts, problems, questions, ideas)
	if r.Method == http.MethodPost {
		// Creating a new post
		if path == "/v1/posts" ||
			path == "/v1/problems" ||
			path == "/v1/questions" ||
			path == "/v1/ideas" {
			return "posts"
		}

		// Creating an answer
		if strings.Contains(path, "/answers") {
			return "answers"
		}
	}

	return "general"
}
