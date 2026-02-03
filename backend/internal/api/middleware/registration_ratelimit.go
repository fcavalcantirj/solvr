// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
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

// DefaultRegistrationRateLimitConfig returns the default configuration.
// Per requirement: 5 registrations per IP per hour.
func DefaultRegistrationRateLimitConfig() *RegistrationRateLimitConfig {
	return &RegistrationRateLimitConfig{
		MaxPerIP:            5,
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
		clientIP := ExtractClientIP(r)
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

// ExtractClientIP extracts the client IP address from the request.
// It checks headers commonly set by proxies/load balancers.
// Order of precedence: X-Forwarded-For (first IP), X-Real-IP, RemoteAddr
func ExtractClientIP(r *http.Request) string {
	// Check X-Forwarded-For header (commonly set by proxies)
	xff := r.Header.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs: client, proxy1, proxy2...
		// The first one is the original client
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			ip := strings.TrimSpace(ips[0])
			if ip != "" {
				return ip
			}
		}
	}

	// Check X-Real-IP header (set by nginx)
	xrip := r.Header.Get("X-Real-IP")
	if xrip != "" {
		return strings.TrimSpace(xrip)
	}

	// Fall back to RemoteAddr
	return extractIPFromAddr(r.RemoteAddr)
}

// extractIPFromAddr extracts the IP from an address string like "ip:port" or "[ipv6]:port".
func extractIPFromAddr(addr string) string {
	if addr == "" {
		return ""
	}

	// Handle IPv6 addresses in brackets
	if strings.HasPrefix(addr, "[") {
		// Format: [ipv6]:port
		host, _, err := net.SplitHostPort(addr)
		if err != nil {
			// Try without port
			if idx := strings.Index(addr, "]:"); idx != -1 {
				return addr[1:idx]
			}
			return strings.Trim(addr, "[]")
		}
		return host
	}

	// Handle IPv4 or IPv6 without brackets
	host, _, err := net.SplitHostPort(addr)
	if err != nil {
		// No port, return as-is
		return addr
	}
	return host
}
