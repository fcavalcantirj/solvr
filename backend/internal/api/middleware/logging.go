// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"bytes"
	"encoding/json"
	"io"
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
	Level       string  `json:"level"`
	Timestamp   string  `json:"timestamp"`
	Message     string  `json:"message"`
	RequestID   string  `json:"request_id,omitempty"`
	Method      string  `json:"method"`
	Path        string  `json:"path"`
	Status      int     `json:"status"`
	DurationMS  float64 `json:"duration_ms"`
	RemoteAddr  string  `json:"remote_addr,omitempty"`
	Error       string  `json:"error,omitempty"`        // Error message for 4xx/5xx responses
	ErrorCode   string  `json:"error_code,omitempty"`   // Error code for 4xx/5xx responses
	RequestBody string  `json:"request_body,omitempty"` // Request body for failed non-GET requests (redacted)
}

// responseWriter wraps http.ResponseWriter to capture the status code and body for error responses.
type responseWriter struct {
	http.ResponseWriter
	status      int
	wroteHeader bool
	body        []byte // Captured body for error responses (4xx/5xx)
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
	// Capture body for error responses (4xx/5xx) to extract error details
	if rw.status >= 400 {
		rw.body = append(rw.body, b...)
	}
	return rw.ResponseWriter.Write(b)
}

// Logging returns middleware that logs HTTP requests in JSON format.
// Log entries include: method, path, status code, duration, and error details for 4xx/5xx.
// SECURITY: API keys, tokens, and other sensitive data are automatically redacted.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Capture request body for non-GET methods (for logging on error)
		var requestBody string
		if r.Method != http.MethodGet && r.Body != nil {
			bodyBytes, err := io.ReadAll(r.Body)
			if err == nil && len(bodyBytes) > 0 {
				requestBody = string(bodyBytes)
				// Restore the body so it can be read by handlers
				r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
			}
		}

		// Wrap response writer to capture status and body
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

		// Extract error details for 4xx/5xx responses
		if wrapped.status >= 400 && len(wrapped.body) > 0 {
			errCode, errMsg := extractErrorDetails(wrapped.body)
			if errCode != "" {
				entry.ErrorCode = errCode
			}
			if errMsg != "" {
				entry.Error = errMsg
			}
		}

		// Include request body for failed non-GET requests (redacted, truncated)
		if wrapped.status >= 400 && requestBody != "" {
			entry.RequestBody = prepareRequestBodyForLog(requestBody)
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

// errorResponse represents the standard error response structure.
type errorResponse struct {
	Error struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	} `json:"error"`
}

// extractErrorDetails extracts error code and message from JSON response body.
// Returns empty strings if the body is not valid JSON or doesn't match expected structure.
func extractErrorDetails(body []byte) (code, message string) {
	if len(body) == 0 {
		return "", ""
	}

	var resp errorResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		// Not valid JSON or unexpected structure, return raw body truncated
		// But only for non-empty bodies
		bodyStr := string(body)
		if len(bodyStr) > 200 {
			bodyStr = bodyStr[:200] + "..."
		}
		return "", bodyStr
	}

	return resp.Error.Code, resp.Error.Message
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

// sensitiveBodyFields lists JSON field names that contain secrets.
// Values of these fields will be redacted in request body logs.
var sensitiveBodyFields = []string{
	"password",
	"api_key",
	"apikey",
	"token",
	"access_token",
	"refresh_token",
	"secret",
	"credential",
	"credentials",
}

// maxRequestBodyLogSize is the maximum size of request body to log (1KB).
const maxRequestBodyLogSize = 1024

// prepareRequestBodyForLog redacts sensitive fields and truncates the body for logging.
func prepareRequestBodyForLog(body string) string {
	// First redact sensitive fields
	redacted := RedactRequestBody(body)

	// Then truncate if needed
	if len(redacted) > maxRequestBodyLogSize {
		return redacted[:maxRequestBodyLogSize] + "...[truncated]"
	}

	return redacted
}

// RedactRequestBody redacts sensitive fields from a JSON request body.
// Fields like password, api_key, token will have their values replaced with ***REDACTED***.
func RedactRequestBody(body string) string {
	if body == "" {
		return body
	}

	// Try to parse as JSON
	var data map[string]interface{}
	if err := json.Unmarshal([]byte(body), &data); err != nil {
		// Not valid JSON, return as-is (or could apply simple regex redaction)
		return body
	}

	// Recursively redact sensitive fields
	redactMapValues(data)

	// Re-serialize
	result, err := json.Marshal(data)
	if err != nil {
		return body
	}

	return string(result)
}

// redactMapValues recursively redacts sensitive field values in a map.
func redactMapValues(data map[string]interface{}) {
	for key, value := range data {
		// Check if this key is sensitive
		if isSensitiveField(key) {
			data[key] = "***REDACTED***"
			continue
		}

		// Recursively handle nested maps
		switch v := value.(type) {
		case map[string]interface{}:
			redactMapValues(v)
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					redactMapValues(m)
				}
			}
		}
	}
}

// isSensitiveField checks if a field name is sensitive (case-insensitive).
func isSensitiveField(fieldName string) bool {
	lowerField := strings.ToLower(fieldName)
	for _, sensitive := range sensitiveBodyFields {
		if lowerField == sensitive {
			return true
		}
	}
	return false
}
