package middleware

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestBodyLimit_AcceptsSmallPayload tests that small payloads are accepted.
func TestBodyLimit_AcceptsSmallPayload(t *testing.T) {
	// Create a test handler that reads the body
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error reading body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	})

	// Wrap with BodyLimit middleware (64KB limit)
	wrapped := BodyLimit(64 * 1024)(handler)

	// Small payload (1KB)
	smallPayload := bytes.Repeat([]byte("a"), 1024)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(smallPayload))
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for small payload, got %d", rr.Code)
	}
	if rr.Body.String() != string(smallPayload) {
		t.Errorf("expected body to be echoed back")
	}
}

// TestBodyLimit_RejectsLargePayload tests that oversized payloads return 413.
// This is the core test for FIX-028.
func TestBodyLimit_RejectsLargePayload(t *testing.T) {
	// Create a test handler that should NOT be called
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for oversized payload")
		w.WriteHeader(http.StatusOK)
	})

	// Wrap with BodyLimit middleware (64KB limit)
	wrapped := BodyLimit(64 * 1024)(handler)

	// Large payload (100KB)
	largePayload := bytes.Repeat([]byte("a"), 100*1024)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(largePayload))
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	// Should return 413 Payload Too Large
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("FIX-028: expected status 413 for large payload, got %d", rr.Code)
	}

	// Verify response is JSON with correct error code
	if rr.Header().Get("Content-Type") != "application/json" {
		t.Errorf("expected Content-Type application/json, got %s", rr.Header().Get("Content-Type"))
	}
}

// TestBodyLimit_AllowsGetRequests tests that GET requests bypass body limit check.
func TestBodyLimit_AllowsGetRequests(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	wrapped := BodyLimit(64 * 1024)(handler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for GET request, got %d", rr.Code)
	}
}

// TestBodyLimit_RejectsExactLimit tests payload at exactly the limit.
func TestBodyLimit_AcceptsExactLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("unexpected error reading body: %v", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// 1KB limit
	limit := int64(1024)
	wrapped := BodyLimit(limit)(handler)

	// Exactly at limit
	exactPayload := bytes.Repeat([]byte("a"), int(limit))
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(exactPayload))
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected status 200 for exact limit payload, got %d", rr.Code)
	}
}

// TestBodyLimit_RejectsOneByteOverLimit tests payload at exactly one byte over limit.
func TestBodyLimit_RejectsOneByteOverLimit(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for over-limit payload")
		w.WriteHeader(http.StatusOK)
	})

	// 1KB limit
	limit := int64(1024)
	wrapped := BodyLimit(limit)(handler)

	// One byte over limit
	overPayload := bytes.Repeat([]byte("a"), int(limit)+1)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader(overPayload))
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413 for over-limit payload, got %d", rr.Code)
	}
}

// TestBodyLimit_ChecksContentLength tests that Content-Length header is checked.
func TestBodyLimit_ChecksContentLength(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for over-limit Content-Length")
		w.WriteHeader(http.StatusOK)
	})

	// 64KB limit
	limit := int64(64 * 1024)
	wrapped := BodyLimit(limit)(handler)

	// Create request with large Content-Length header (but small actual body)
	req := httptest.NewRequest(http.MethodPost, "/test", bytes.NewReader([]byte("small")))
	req.ContentLength = 100 * 1024 // Claim 100KB
	rr := httptest.NewRecorder()

	wrapped.ServeHTTP(rr, req)

	// Should be rejected based on Content-Length header alone
	if rr.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("expected status 413 for large Content-Length, got %d", rr.Code)
	}
}
