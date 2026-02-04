package db

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"testing"
)

// TestLogQueryError verifies that LogQueryError produces structured JSON logs.
func TestLogQueryError(t *testing.T) {
	// Create a buffer to capture log output
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	// Replace the default logger temporarily
	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	// Create a test error
	testErr := errors.New("connection refused")

	// Call the function under test
	LogQueryError(context.Background(), "FindByID", "posts", testErr)

	// Parse the log output
	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	// Verify required fields
	if logEntry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", logEntry["level"])
	}

	if logEntry["msg"] != "db query failed" {
		t.Errorf("expected msg 'db query failed', got %v", logEntry["msg"])
	}

	if logEntry["op"] != "FindByID" {
		t.Errorf("expected op 'FindByID', got %v", logEntry["op"])
	}

	if logEntry["table"] != "posts" {
		t.Errorf("expected table 'posts', got %v", logEntry["table"])
	}

	if logEntry["error"] != "connection refused" {
		t.Errorf("expected error 'connection refused', got %v", logEntry["error"])
	}
}

// TestLogQueryErrorWithContext verifies context values are included in logs.
func TestLogQueryErrorWithContext(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	testErr := errors.New("timeout")

	// Create context with request ID (commonly added by middleware)
	ctx := context.WithValue(context.Background(), requestIDKey, "req-12345")

	LogQueryError(ctx, "Create", "agents", testErr)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["request_id"] != "req-12345" {
		t.Errorf("expected request_id 'req-12345', got %v", logEntry["request_id"])
	}
}

// TestLogConnectionError verifies that LogConnectionError logs connection errors.
func TestLogConnectionError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	testErr := errors.New("connection pool exhausted")

	LogConnectionError(context.Background(), testErr)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["level"] != "ERROR" {
		t.Errorf("expected level ERROR, got %v", logEntry["level"])
	}

	if logEntry["msg"] != "db connection error" {
		t.Errorf("expected msg 'db connection error', got %v", logEntry["msg"])
	}

	if logEntry["error"] != "connection pool exhausted" {
		t.Errorf("expected error 'connection pool exhausted', got %v", logEntry["error"])
	}
}

// TestLogTransactionError verifies that LogTransactionError logs transaction errors.
func TestLogTransactionError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	testErr := errors.New("deadlock detected")

	LogTransactionError(context.Background(), "Vote", testErr)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["msg"] != "db transaction failed" {
		t.Errorf("expected msg 'db transaction failed', got %v", logEntry["msg"])
	}

	if logEntry["op"] != "Vote" {
		t.Errorf("expected op 'Vote', got %v", logEntry["op"])
	}
}

// TestLogDuplicateKeyError verifies that duplicate key errors are logged at info level.
func TestLogDuplicateKeyError(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	// Duplicate key errors are expected behavior, should be logged as info
	LogDuplicateKeyError(context.Background(), "Create", "users", "username")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	// Duplicate key is not an error, it's expected behavior
	if logEntry["level"] != "INFO" {
		t.Errorf("expected level INFO for duplicate key, got %v", logEntry["level"])
	}

	if logEntry["msg"] != "duplicate key constraint" {
		t.Errorf("expected msg 'duplicate key constraint', got %v", logEntry["msg"])
	}

	if logEntry["constraint"] != "username" {
		t.Errorf("expected constraint 'username', got %v", logEntry["constraint"])
	}
}

// TestLogNotFoundAsDebug verifies that not found errors are logged at debug level.
func TestLogNotFoundAsDebug(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	// Not found is expected behavior when querying non-existent resources
	LogNotFound(context.Background(), "FindByID", "posts", "abc-123")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	// Not found should be debug level - it's normal operation
	if logEntry["level"] != "DEBUG" {
		t.Errorf("expected level DEBUG for not found, got %v", logEntry["level"])
	}

	if logEntry["id"] != "abc-123" {
		t.Errorf("expected id 'abc-123', got %v", logEntry["id"])
	}
}
