// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

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

		// Build log entry
		entry := LogEntry{
			Level:      logLevel(wrapped.status),
			Timestamp:  time.Now().UTC().Format(time.RFC3339),
			Message:    "Request completed",
			Method:     r.Method,
			Path:       r.URL.Path,
			Status:     wrapped.status,
			DurationMS: float64(duration.Nanoseconds()) / 1e6,
		}

		// Add optional fields
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
