// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"
	"time"
)

// RegistrationRateLimitConfig holds configuration for registration rate limiting.
// Per AGENT-ONBOARDING requirement: Limit registrations per IP (e.g., 5/hour).
type RegistrationRateLimitConfig struct {
	// MaxPerIP is the maximum number of registrations per IP per window.
	MaxPerIP int

	// Window is the time window for rate limiting.
	Window time.Duration

	// LogPrefix is the prefix for log messages.
	LogPrefix string

	// SuspiciousThreshold is the count at which to log suspicious patterns.
	// When an IP exceeds this many attempts, it's logged as suspicious.
	SuspiciousThreshold int
}

// DefaultRegistrationsPerIPPerHour is how many agents one client IP may register in an hour.
// Owner decision 2026-10-03 (spec.json idx 79): 20, so an 8-agent room set up from one
// machine fits (idx 24). RATE_LIMIT_REGISTRATIONS_PER_IP_HOUR overrides it at startup.
const DefaultRegistrationsPerIPPerHour = 20

// DefaultRegistrationRateLimitConfig returns the default configuration:
// DefaultRegistrationsPerIPPerHour registrations per IP per hour.
func DefaultRegistrationRateLimitConfig() *RegistrationRateLimitConfig {
	return &RegistrationRateLimitConfig{
		MaxPerIP:            DefaultRegistrationsPerIPPerHour,
		Window:              time.Hour,
		LogPrefix:           "registration",
		SuspiciousThreshold: 10,
	}
}

// RegistrationRateLimiter implements IP-based rate limiting for registration endpoints.
type RegistrationRateLimiter struct {
	store  RateLimitStore
	config *RegistrationRateLimitConfig
}

// NewRegistrationRateLimiter creates a new RegistrationRateLimiter.
func NewRegistrationRateLimiter(store RateLimitStore, config *RegistrationRateLimitConfig) *RegistrationRateLimiter {
	if config == nil {
		config = DefaultRegistrationRateLimitConfig()
	}
	return &RegistrationRateLimiter{
		store:  store,
		config: config,
	}
}

// Middleware returns HTTP middleware that enforces IP-based registration rate limits.
func (rl *RegistrationRateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract client IP
		clientIP := ClientIP(r)
		if clientIP == "" {
			// If we can't determine IP, allow through but log it
			log.Printf("[%s] WARNING: could not determine client IP for request", rl.config.LogPrefix)
			next.ServeHTTP(w, r)
			return
		}

		// Generate rate limit key for this IP
		key := rl.generateKey(clientIP)

		// Increment and check the limit
		record, err := rl.store.IncrementAndGet(r.Context(), key, rl.config.Window)
		if err != nil {
			// On error, allow request through (fail open) but log it
			log.Printf("[%s] ERROR: rate limit store failed: %v", rl.config.LogPrefix, err)
			next.ServeHTTP(w, r)
			return
		}

		SetRateLimitHeaders(w.Header(), rl.config.MaxPerIP, rl.config.MaxPerIP-record.Count,
			record.WindowStart.Add(rl.config.Window), time.Now())

		// Log suspicious patterns (only if threshold is configured)
		if rl.config.SuspiciousThreshold > 0 && record.Count >= rl.config.SuspiciousThreshold {
			log.Printf("[%s] SUSPICIOUS: IP %s has made %d registration attempts in window (threshold: %d)",
				rl.config.LogPrefix, clientIP, record.Count, rl.config.SuspiciousThreshold)
		}

		// Check if rate limited (after incrementing, so count > limit means exceeded)
		if record.Count > rl.config.MaxPerIP {
			rl.writeRateLimitError(w, r, record, clientIP)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// generateKey creates the rate limit key for an IP address.
func (rl *RegistrationRateLimiter) generateKey(ip string) string {
	return "registration:ip:" + ip
}

// writeRateLimitError writes a 429 Too Many Requests response.
func (rl *RegistrationRateLimiter) writeRateLimitError(w http.ResponseWriter, r *http.Request, record *RateLimitRecord, clientIP string) {
	// Calculate reset time
	resetTime := record.WindowStart.Add(rl.config.Window)

	// Calculate Retry-After in seconds
	retryAfter := int(time.Until(resetTime).Seconds())
	if retryAfter < 1 {
		retryAfter = 1
	}

	// Log the rate limit event
	log.Printf("[%s] RATE_LIMITED: IP %s exceeded registration limit (%d/%d), retry after %ds",
		rl.config.LogPrefix, clientIP, record.Count, rl.config.MaxPerIP, retryAfter)

	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.WriteHeader(http.StatusTooManyRequests)

	response := map[string]interface{}{
		"error": map[string]interface{}{
			"code":    "RATE_LIMITED",
			"message": "too many registration attempts from this IP, please try again later",
			"details": map[string]interface{}{
				"retry_after_seconds": retryAfter,
				"limit":               rl.config.MaxPerIP,
				"window":              rl.config.Window.String(),
			},
		},
	}

	json.NewEncoder(w).Encode(response)
}
