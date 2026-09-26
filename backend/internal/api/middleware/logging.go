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
	"slices"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

// sensitiveParams lists URL query parameter names that contain secrets.
// Values of these parameters will be redacted in logs.
var sensitiveParams = []string{
	"api_key",
	"apikey",
	"token",
	"access_token",
	"ticket",
	"refresh_token",
	"secret",
	"password",
	"key",
}

// sensitivePathParams lists route parameter names whose VALUE in the URL path is a
// secret (the claim link GET /v1/claim/{token}). The value is redacted in logs, exactly as
// a sensitive query parameter is.
var sensitivePathParams = []string{
	"token",
	"claim_token",
	"ticket",
	"secret",
	"api_key",
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

// Flush implements http.Flusher by delegating to the underlying ResponseWriter.
// This is required for SSE streaming: the SSE handler checks w.(http.Flusher)
// and the logging middleware must not break that assertion.
func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
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

		// Build log entry with redacted path (removes sensitive path segments and
		// sensitive query params)
		logPath := redactPathSecrets(r, r.URL.Path)
		if r.URL.RawQuery != "" {
			logPath += RedactURLPath("?" + r.URL.RawQuery)
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

// redactPathSecrets replaces every path segment that the matched route bound to a
// sensitive parameter (see sensitivePathParams) with ***REDACTED***. The route is only
// known once routing has happened, so this runs after the handler; a request that matched
// no route has no bound parameters and its path is returned as is.
func redactPathSecrets(r *http.Request, path string) string {
	rctx := chi.RouteContext(r.Context())
	if rctx == nil {
		return path
	}
	var secrets []string
	for i, key := range rctx.URLParams.Keys {
		if i < len(rctx.URLParams.Values) && rctx.URLParams.Values[i] != "" && slices.Contains(sensitivePathParams, key) {
			secrets = append(secrets, rctx.URLParams.Values[i])
		}
	}
	if len(secrets) == 0 {
		return path
	}
	segments := strings.Split(path, "/")
	for i, segment := range segments {
		if slices.Contains(secrets, segment) {
			segments[i] = "***REDACTED***"
		}
	}
	return strings.Join(segments, "/")
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

// sensitiveBodyFields lists the fragments that mark a body field (JSON key, form field) as a
// secret. A field is sensitive when its lower-cased name, with "_", "-" and "." removed,
// contains one: new_password, room_token, Client-Secret and X.API.Key are all secrets, not just
// a field spelled exactly `password`. A field named exactly `key` is one too (see
// isSensitiveField).
var sensitiveBodyFields = []string{
	"password",
	"passwd",
	"apikey",
	"token",
	"secret",
	"credential",
	"ticket",
	"authorization",
	"privatekey",
	"logincode",
}

// secretTextPattern finds `name: value` and `name=value` pairs of sensitive names in text that
// is not a JSON object or array (malformed JSON, a truncated body, a form): the value may be
// double- or single-quoted, cut off before its closing quote, or bare. It keeps the name and
// replaces the value, so the log still shows which field the client sent. The boundary group
// stops a name from being matched from the middle of a longer word (monkey is not key).
var secretTextPattern = regexp.MustCompile(`(?i)((?:^|[^A-Za-z0-9_.\-])["']?(?:[A-Za-z0-9_.\-]*(?:password|passwd|token|secret|api[_-]?key|credential|ticket|authorization|private[_-]?key|login[_-]?code)[A-Za-z0-9_.\-]*|key)["']?\s*[:=]\s*)(?:"(?:[^"\\]|\\.)*"?|'(?:[^'\\]|\\.)*'?|[^&\s,}\]"']*)`)

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

// RedactRequestBody redacts secrets from a request body before it is logged. In a JSON object
// or array (however deeply nested) the value of every sensitive field is replaced with
// ***REDACTED***. Anything else, which is what a malformed or truncated body is, is scanned
// for sensitive `name: value` and `name=value` pairs instead, because the body of a request
// that failed validation is exactly the kind that arrives malformed.
func RedactRequestBody(body string) string {
	if body == "" {
		return body
	}

	var data any
	if err := json.Unmarshal([]byte(body), &data); err == nil {
		switch data.(type) {
		case map[string]any, []any:
			redactValue(data)
			if result, err := json.Marshal(data); err == nil {
				return string(result)
			}
		}
	}
	return secretTextPattern.ReplaceAllString(body, `${1}"***REDACTED***"`)
}

// redactValue redacts sensitive fields in place, recursing through objects and arrays.
func redactValue(value any) {
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			if isSensitiveField(key) {
				v[key] = "***REDACTED***"
				continue
			}
			redactValue(item)
		}
	case []any:
		for _, item := range v {
			redactValue(item)
		}
	}
}

// isSensitiveField reports whether a body field name marks a secret (case-insensitive, and
// blind to "_", "-" and "." between words).
func isSensitiveField(fieldName string) bool {
	name := strings.NewReplacer("_", "", "-", "", ".", "").Replace(strings.ToLower(fieldName))
	if name == "key" {
		return true
	}
	for _, sensitive := range sensitiveBodyFields {
		if strings.Contains(name, sensitive) {
			return true
		}
	}
	return false
}
