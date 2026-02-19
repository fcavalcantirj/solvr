package db

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
)

// TestLogSearchCompleted_HybridRRF verifies that hybrid search logs with method=hybrid_rrf.
func TestLogSearchCompleted_HybridRRF(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	LogSearchCompleted(context.Background(), "golang race condition", 42, 15, "hybrid_rrf")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["level"] != "INFO" {
		t.Errorf("expected level INFO, got %v", logEntry["level"])
	}

	if logEntry["msg"] != "search completed" {
		t.Errorf("expected msg 'search completed', got %v", logEntry["msg"])
	}

	if logEntry["query"] != "golang race condition" {
		t.Errorf("expected query 'golang race condition', got %v", logEntry["query"])
	}

	// duration_ms should be present and numeric
	durationMs, ok := logEntry["duration_ms"].(float64)
	if !ok {
		t.Fatalf("expected duration_ms to be a number, got %T", logEntry["duration_ms"])
	}
	if durationMs != 42 {
		t.Errorf("expected duration_ms 42, got %v", durationMs)
	}

	resultsCount, ok := logEntry["results_count"].(float64)
	if !ok {
		t.Fatalf("expected results_count to be a number, got %T", logEntry["results_count"])
	}
	if resultsCount != 15 {
		t.Errorf("expected results_count 15, got %v", resultsCount)
	}

	if logEntry["method"] != "hybrid_rrf" {
		t.Errorf("expected method 'hybrid_rrf', got %v", logEntry["method"])
	}
}

// TestLogSearchCompleted_FulltextOnly verifies that fulltext search logs with method=fulltext_only.
func TestLogSearchCompleted_FulltextOnly(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	LogSearchCompleted(context.Background(), "async bug", 10, 3, "fulltext_only")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["method"] != "fulltext_only" {
		t.Errorf("expected method 'fulltext_only', got %v", logEntry["method"])
	}

	if logEntry["query"] != "async bug" {
		t.Errorf("expected query 'async bug', got %v", logEntry["query"])
	}
}

// TestLogSearchCompleted_WithRequestID verifies request_id is included from context.
func TestLogSearchCompleted_WithRequestID(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	ctx := context.WithValue(context.Background(), requestIDKey, "req-search-001")
	LogSearchCompleted(ctx, "test query", 5, 0, "fulltext_only")

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["request_id"] != "req-search-001" {
		t.Errorf("expected request_id 'req-search-001', got %v", logEntry["request_id"])
	}
}

// TestLogSearchEmbeddingGenerated verifies embedding generation is logged at DEBUG level.
func TestLogSearchEmbeddingGenerated(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	LogSearchEmbeddingGenerated(context.Background(), 125)

	var logEntry map[string]any
	if err := json.Unmarshal(buf.Bytes(), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v", err)
	}

	if logEntry["level"] != "DEBUG" {
		t.Errorf("expected level DEBUG, got %v", logEntry["level"])
	}

	if logEntry["msg"] != "query embedding generated" {
		t.Errorf("expected msg 'query embedding generated', got %v", logEntry["msg"])
	}

	// duration_ms in milliseconds (not seconds) - consistent with project convention
	durationMs, ok := logEntry["duration_ms"].(float64)
	if !ok {
		t.Fatalf("expected duration_ms to be a number, got %T", logEntry["duration_ms"])
	}
	if durationMs != 125 {
		t.Errorf("expected duration_ms 125, got %v", durationMs)
	}
}

// TestLogSearchEmbeddingFailed verifies embedding failure is logged as warning with error.
func TestLogSearchEmbeddingFailed(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug}))

	oldLogger := slog.Default()
	slog.SetDefault(logger)
	defer slog.SetDefault(oldLogger)

	LogSearchEmbeddingFailed(context.Background(), "api timeout")

	// Parse multi-line output - take the last log entry (there might be multiple)
	lines := strings.Split(strings.TrimSpace(buf.String()), "\n")
	lastLine := lines[len(lines)-1]

	var logEntry map[string]any
	if err := json.Unmarshal([]byte(lastLine), &logEntry); err != nil {
		t.Fatalf("failed to parse log output as JSON: %v\nraw: %s", err, lastLine)
	}

	if logEntry["level"] != "WARN" {
		t.Errorf("expected level WARN, got %v", logEntry["level"])
	}

	if logEntry["msg"] != "search embedding generation failed, falling back to full-text" {
		t.Errorf("expected msg about embedding failure fallback, got %v", logEntry["msg"])
	}

	if logEntry["error"] != "api timeout" {
		t.Errorf("expected error 'api timeout', got %v", logEntry["error"])
	}
}
