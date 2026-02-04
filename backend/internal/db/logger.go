// Package db provides database access for Solvr.
package db

import (
	"context"
	"log/slog"
)

// contextKey is the type for context keys in this package.
type contextKey string

// requestIDKey is the context key for request ID.
// This should match the key used by the request logging middleware.
const requestIDKey contextKey = "request_id"

// LogQueryError logs a database query error with structured context.
// Per FIX-013: Add DB query error logging with table, operation, and error details.
// Pattern: slog.Error("db query failed", "op", "FindByID", "table", "posts", "error", err)
func LogQueryError(ctx context.Context, op, table string, err error) {
	attrs := []any{
		"op", op,
		"table", table,
		"error", err.Error(),
	}

	// Include request ID if present in context
	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Error("db query failed", attrs...)
}

// LogConnectionError logs a database connection error.
// Per FIX-013: Log connection errors separately from query errors.
func LogConnectionError(ctx context.Context, err error) {
	attrs := []any{
		"error", err.Error(),
	}

	// Include request ID if present in context
	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Error("db connection error", attrs...)
}

// LogTransactionError logs a database transaction error.
// Includes operation name for context on what was being attempted.
func LogTransactionError(ctx context.Context, op string, err error) {
	attrs := []any{
		"op", op,
		"error", err.Error(),
	}

	// Include request ID if present in context
	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Error("db transaction failed", attrs...)
}

// LogDuplicateKeyError logs a duplicate key constraint violation.
// These are expected errors (e.g., registering existing username) so logged at INFO level.
// This is business logic - not a system error.
func LogDuplicateKeyError(ctx context.Context, op, table, constraint string) {
	attrs := []any{
		"op", op,
		"table", table,
		"constraint", constraint,
	}

	// Include request ID if present in context
	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Info("duplicate key constraint", attrs...)
}

// LogNotFound logs when a resource is not found.
// Not found is expected behavior (querying non-existent resources) so logged at DEBUG level.
func LogNotFound(ctx context.Context, op, table, id string) {
	attrs := []any{
		"op", op,
		"table", table,
		"id", id,
	}

	// Include request ID if present in context
	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Debug("resource not found", attrs...)
}
