// Package response provides HTTP response helpers for the Solvr API.
// Response format follows SPEC.md Part 5.3.
package response

import (
	"encoding/json"
	"log/slog"
	"net/http"
)

// SuccessResponse wraps successful responses per SPEC.md Part 5.3.
type SuccessResponse struct {
	Data interface{} `json:"data"`
	Meta *Meta       `json:"meta,omitempty"`
}

// Meta contains pagination and timing metadata.
type Meta struct {
	Total     int    `json:"total,omitempty"`
	Page      int    `json:"page,omitempty"`
	PerPage   int    `json:"per_page,omitempty"`
	HasMore   bool   `json:"has_more,omitempty"`
	Timestamp string `json:"timestamp,omitempty"`
}

// ErrorResponse wraps error responses per SPEC.md Part 5.3.
type ErrorResponse struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail contains error information.
type ErrorDetail struct {
	Code    string      `json:"code"`
	Message string      `json:"message"`
	Details interface{} `json:"details,omitempty"`
}

// WriteJSON writes a successful JSON response with data envelope.
// Format: {"data": {...}}
func WriteJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := SuccessResponse{
		Data: data,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		// Log but can't really recover at this point
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"failed to encode response"}}`, http.StatusInternalServerError)
	}
}

// WriteJSONWithMeta writes a successful JSON response with data and meta.
// Format: {"data": [...], "meta": {...}}
func WriteJSONWithMeta(w http.ResponseWriter, status int, data interface{}, meta Meta) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := SuccessResponse{
		Data: data,
		Meta: &meta,
	}

	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, `{"error":{"code":"INTERNAL_ERROR","message":"failed to encode response"}}`, http.StatusInternalServerError)
	}
}

// WriteError writes an error JSON response.
// Format: {"error": {"code": "...", "message": "..."}}
func WriteError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// WriteErrorWithDetails writes an error JSON response with additional details.
// Format: {"error": {"code": "...", "message": "...", "details": {...}}}
func WriteErrorWithDetails(w http.ResponseWriter, status int, code, message string, details interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	response := ErrorResponse{
		Error: ErrorDetail{
			Code:    code,
			Message: message,
			Details: details,
		},
	}

	json.NewEncoder(w).Encode(response)
}

// WriteCreated writes a 201 Created response with data.
func WriteCreated(w http.ResponseWriter, data interface{}) {
	WriteJSON(w, http.StatusCreated, data)
}

// WriteNoContent writes a 204 No Content response.
func WriteNoContent(w http.ResponseWriter) {
	w.WriteHeader(http.StatusNoContent)
}

// Common error codes per SPEC.md Part 5.4.
const (
	ErrCodeUnauthorized     = "UNAUTHORIZED"
	ErrCodeForbidden        = "FORBIDDEN"
	ErrCodeNotFound         = "NOT_FOUND"
	ErrCodeValidation       = "VALIDATION_ERROR"
	ErrCodeRateLimited      = "RATE_LIMITED"
	ErrCodeDuplicateContent = "DUPLICATE_CONTENT"
	ErrCodeContentTooShort  = "CONTENT_TOO_SHORT"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeMethodNotAllowed = "METHOD_NOT_ALLOWED"
)

// Convenience error writers for common cases.

// WriteUnauthorized writes a 401 Unauthorized response.
func WriteUnauthorized(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusUnauthorized, ErrCodeUnauthorized, message)
}

// WriteForbidden writes a 403 Forbidden response.
func WriteForbidden(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusForbidden, ErrCodeForbidden, message)
}

// WriteNotFound writes a 404 Not Found response.
func WriteNotFound(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusNotFound, ErrCodeNotFound, message)
}

// WriteValidationError writes a 400 Bad Request response for validation errors.
func WriteValidationError(w http.ResponseWriter, message string, details interface{}) {
	WriteErrorWithDetails(w, http.StatusBadRequest, ErrCodeValidation, message, details)
}

// WriteRateLimited writes a 429 Too Many Requests response.
func WriteRateLimited(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusTooManyRequests, ErrCodeRateLimited, message)
}

// WriteInternalError writes a 500 Internal Server Error response.
func WriteInternalError(w http.ResponseWriter, message string) {
	WriteError(w, http.StatusInternalServerError, ErrCodeInternalError, message)
}

// LogContext provides additional context for error logging.
// Use this to pass request-specific information to error logs.
type LogContext struct {
	Operation string            // The operation that failed (e.g., "FindByID", "Create")
	Resource  string            // The resource being operated on (e.g., "post", "agent")
	RequestID string            // The request ID for correlation
	Extra     map[string]string // Additional key-value pairs for context
}

// WriteInternalErrorWithLog writes a 500 response AND logs the actual error with context.
// This ensures internal errors are visible in logs for debugging while still returning
// a safe generic message to clients.
//
// Per FIX-011: Handlers should use this instead of WriteInternalError when they have
// an error to log. The logged error includes:
// - The error message (to client)
// - The actual error (to logs)
// - Operation context (to logs)
// - Request ID for correlation (to logs)
// - Any extra context (to logs)
//
// Example usage:
//
//	ctx := response.LogContext{
//	    Operation: "FindByID",
//	    Resource:  "post",
//	    RequestID: r.Header.Get("X-Request-ID"),
//	    Extra:     map[string]string{"postID": postID},
//	}
//	response.WriteInternalErrorWithLog(w, "failed to get post", err, ctx, logger)
func WriteInternalErrorWithLog(w http.ResponseWriter, message string, err error, ctx LogContext, logger *slog.Logger) {
	// Log the error with full context
	if logger != nil {
		attrs := []any{
			"error", err.Error(),
		}

		// Add optional context fields
		if ctx.Operation != "" {
			attrs = append(attrs, "operation", ctx.Operation)
		}
		if ctx.Resource != "" {
			attrs = append(attrs, "resource", ctx.Resource)
		}
		if ctx.RequestID != "" {
			attrs = append(attrs, "request_id", ctx.RequestID)
		}

		// Add extra context
		for k, v := range ctx.Extra {
			attrs = append(attrs, k, v)
		}

		logger.Error(message, attrs...)
	}

	// Write the HTTP response (generic message to client)
	WriteInternalError(w, message)
}
