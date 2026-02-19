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

// LogSearchCompleted logs a completed search with method and latency.
// method is "hybrid_rrf" when vector+fulltext fusion is used, or "fulltext_only" for keyword-only.
// duration_ms in milliseconds (not seconds) - consistent with project convention.
func LogSearchCompleted(ctx context.Context, query string, durationMs int64, resultsCount int, method string) {
	attrs := []any{
		"query", query,
		"duration_ms", durationMs,
		"results_count", resultsCount,
		"method", method,
	}

	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Info("search completed", attrs...)
}

// LogSearchEmbeddingGenerated logs query embedding generation timing at DEBUG level.
// duration_ms in milliseconds (not seconds) - consistent with project convention.
func LogSearchEmbeddingGenerated(ctx context.Context, durationMs int64) {
	attrs := []any{
		"duration_ms", durationMs,
	}

	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Debug("query embedding generated", attrs...)
}

// LogSearchEmbeddingFailed logs when embedding generation fails and search falls back to full-text.
// Logged at WARN level since the search still works but with reduced quality.
func LogSearchEmbeddingFailed(ctx context.Context, errMsg string) {
	attrs := []any{
		"error", errMsg,
	}

	if reqID := ctx.Value(requestIDKey); reqID != nil {
		attrs = append(attrs, "request_id", reqID)
	}

	slog.Warn("search embedding generation failed, falling back to full-text", attrs...)
}
