// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// sensitiveParams lists URL query parameter names that contain secrets.
// Values of these parameters will be redacted in logs.
var sensitiveParams = []string{
	"api_key",
	"apikey",
	"token",
	"access_token",
	"refresh_token",
	"secret",
	"password",
	"key",
}

// solvrKeyPrefix is the prefix for Solvr API keys.
const solvrKeyPrefix = "solvr_"

// bearerPrefix is the prefix for Bearer tokens in Authorization headers.
const bearerPrefix = "Bearer "

// jwtRegex matches JWT tokens (three base64 segments separated by dots).
var jwtRegex = regexp.MustCompile(`^eyJ[A-Za-z0-9_-]+\.eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+$`)

// LogEntry represents a structured log entry for HTTP requests.
type LogEntry struct {
	Level      string  `json:"level"`
	Timestamp  string  `json:"timestamp"`
	Message    string  `json:"message"`
	RequestID  string  `json:"request_id,omitempty"`
	Method     string  `json:"method"`
	Path       string  `json:"path"`
	Status     int     `json:"status"`
	DurationMS float64 `json:"duration_ms"`
	RemoteAddr string  `json:"remote_addr,omitempty"`
}

// responseWriter wraps http.ResponseWriter to capture the status code.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
}

func (rw *responseWriter) WriteHeader(code int) {
	if !rw.wroteHeader {
		rw.status = code
		rw.wroteHeader = true
		rw.ResponseWriter.WriteHeader(code)
	}
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	if !rw.wroteHeader {
		rw.WriteHeader(http.StatusOK)
	}
	return rw.ResponseWriter.Write(b)
}

// Logging returns middleware that logs HTTP requests in JSON format.
// Log entries include: method, path, status code, and duration.
// SECURITY: API keys, tokens, and other sensitive data are automatically redacted.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status
		wrapped := &responseWriter{
			ResponseWriter: w,
			status:         http.StatusOK,
		}

		// Process request
		next.ServeHTTP(wrapped, r)

		// Calculate duration
		duration := time.Since(start)

		// Build log entry with redacted path (removes sensitive query params)
		logPath := r.URL.Path
		if r.URL.RawQuery != "" {
			logPath = RedactURLPath(r.URL.Path + "?" + r.URL.RawQuery)
		}

		entry := LogEntry{
			Level:      logLevel(wrapped.status),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			Message:    "Request completed",
			Method:     r.Method,
			Path:       logPath,
			Status:     wrapped.status,
			DurationMS: float64(duration.Nanoseconds()) / 1e6,
		}

		// Add optional fields (all redacted for security)
		if requestID := r.Header.Get("X-Request-ID"); requestID != "" {
			entry.RequestID = requestID
		}
		if r.RemoteAddr != "" {
			entry.RemoteAddr = r.RemoteAddr
		}

		// Output JSON log
		logJSON, err := json.Marshal(entry)
		if err != nil {
			log.Printf("failed to marshal log entry: %v", err)
			return
		}
		log.Println(string(logJSON))
	})
}

// logLevel returns the appropriate log level based on status code.
func logLevel(status int) string {
	switch {
	case status >= 500:
		return "error"
	case status >= 400:
		return "warn"
	default:
		return "info"
	}
}

// RedactSensitiveData redacts sensitive data from a string value.
// It handles:
// - Solvr API keys (solvr_xxx) -> solvr_***REDACTED***
// - Bearer tokens -> Bearer ***REDACTED***
// - JWT tokens -> ***REDACTED***
func RedactSensitiveData(value string) string {
	if value == "" {
		return value
	}

	// Check for Bearer prefix
	if strings.HasPrefix(value, bearerPrefix) {
		return bearerPrefix + "***REDACTED***"
	}

	// Check for Solvr API key prefix
	if strings.HasPrefix(value, solvrKeyPrefix) {
		return solvrKeyPrefix + "***REDACTED***"
	}

	// Check for JWT token pattern
	if jwtRegex.MatchString(value) {
		return "***REDACTED***"
	}

	return value
}

// RedactURLPath redacts sensitive query parameters from a URL path.
// Parameters like api_key, token, access_token will have their values redacted.
func RedactURLPath(path string) string {
	// Parse the URL
	u, err := url.Parse(path)
	if err != nil {
		return path
	}

	// If no query string, return as is
	if u.RawQuery == "" {
		return path
	}

	// Parse query parameters
	query := u.Query()
	modified := false

	// Check each sensitive param
	for _, param := range sensitiveParams {
		if query.Has(param) {
			query.Set(param, "***REDACTED***")
			modified = true
		}
	}

	// If nothing was modified, return original
	if !modified {
		return path
	}

	// Rebuild the URL
	u.RawQuery = query.Encode()
	return u.String()
}
