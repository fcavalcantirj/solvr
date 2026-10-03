package api

import (
	"os"
	"strings"
	"time"

	"github.com/go-chi/cors"
)

// The cross-origin, request-size and timeout policy of the public API. One definition feeds
// both the server and the OpenAPI contract (openapi_conventions.go), so the published
// contract cannot drift from what the server enforces.

// Server timeouts (cmd/api/main.go). There is deliberately no write timeout: room streams
// are long-lived. ServerReadTimeout protects against slow-header attacks and the body
// limit against slow-body ones.
const (
	ServerReadTimeout = 15 * time.Second
	ServerIdleTimeout = 60 * time.Second
)

// requestBodyLimitBytes caps every non-multipart request body (FIX-028).
const requestBodyLimitBytes = 64 * 1024

// corsMaxAge is how long a browser may cache a preflight answer.
const corsMaxAge = 12 * time.Hour

var (
	defaultAllowedOrigins = []string{"http://localhost:3000", "https://solvr.dev", "https://www.solvr.dev"}

	corsAllowedMethods = []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"}

	// Request headers a browser page may send: credentials, body type, correlation, the
	// create-retry key, the edit precondition and the stream resume cursor.
	corsAllowedHeaders = []string{
		"Accept", "Authorization", "Content-Type", "X-Request-ID", "X-Session-ID",
		"Idempotency-Key", "If-Match", "Last-Event-ID",
	}

	// Response headers a browser page may read: correlation, rate-limit state, the edit
	// validator, the replay marker and the retry delay.
	corsExposedHeaders = []string{
		"X-Request-ID", "X-RateLimit-Limit", "X-RateLimit-Remaining", "X-RateLimit-Reset",
		"RateLimit-Limit", "RateLimit-Remaining", "RateLimit-Reset",
		"ETag", "Idempotent-Replayed", "Retry-After",
	}
)

// allowedOrigins returns the ALLOWED_ORIGINS env list (comma separated) or the defaults.
func allowedOrigins() []string {
	envOrigins := os.Getenv("ALLOWED_ORIGINS")
	if envOrigins == "" {
		return defaultAllowedOrigins
	}
	origins := strings.Split(envOrigins, ",")
	for i, origin := range origins {
		origins[i] = strings.TrimSpace(origin)
	}
	return origins
}

func corsOptions() cors.Options {
	return cors.Options{
		AllowedOrigins:   allowedOrigins(),
		AllowedMethods:   corsAllowedMethods,
		AllowedHeaders:   corsAllowedHeaders,
		ExposedHeaders:   corsExposedHeaders,
		AllowCredentials: true,
		MaxAge:           int(corsMaxAge / time.Second),
	}
}
