package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// serveEnveloped runs h behind ErrorEnvelope with a fixed X-Request-ID
// already on the response (the router's requestIDMiddleware sets it first).
func serveEnveloped(t *testing.T, method string, h http.HandlerFunc) *httptest.ResponseRecorder {
	t.Helper()
	withID := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Request-ID", "req-123")
		ErrorEnvelope(h).ServeHTTP(w, r)
	})
	rec := httptest.NewRecorder()
	withID.ServeHTTP(rec, httptest.NewRequest(method, "/v1/x", nil))
	return rec
}

func decodeErrorObject(t *testing.T, rec *httptest.ResponseRecorder) map[string]interface{} {
	t.Helper()
	var body map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("body is not JSON: %v (%q)", err, rec.Body.String())
	}
	errObj, ok := body["error"].(map[string]interface{})
	if !ok {
		t.Fatalf("body has no error object: %q", rec.Body.String())
	}
	return errObj
}

func TestErrorEnvelope_AddsRequestIDToJSONError(t *testing.T) {
	rec := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		w.Write([]byte(`{"error":{"code":"NOT_FOUND","message":"post not found","details":{"n":1000000}}}`))
	})
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404", rec.Code)
	}
	errObj := decodeErrorObject(t, rec)
	if errObj["request_id"] != "req-123" {
		t.Errorf("request_id = %v, want req-123", errObj["request_id"])
	}
	if errObj["code"] != "NOT_FOUND" || errObj["message"] != "post not found" {
		t.Errorf("code/message changed: %v", errObj)
	}
	// Numbers inside details must survive byte-for-byte (no float re-encoding).
	if !strings.Contains(rec.Body.String(), `"n":1000000`) {
		t.Errorf("details number re-encoded: %s", rec.Body.String())
	}
}

func TestErrorEnvelope_KeepsHandlerRequestID(t *testing.T) {
	rec := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":{"code":"VALIDATION_ERROR","message":"bad","request_id":"own"}}`))
	})
	if got := decodeErrorObject(t, rec)["request_id"]; got != "own" {
		t.Errorf("request_id = %v, want handler's own value", got)
	}
}

func TestErrorEnvelope_AddsRetryAfterSecondsOn429(t *testing.T) {
	rec := serveEnveloped(t, http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "42")
		w.WriteHeader(http.StatusTooManyRequests)
		w.Write([]byte(`{"error":{"code":"RATE_LIMITED","message":"slow down"}}`))
	})
	errObj := decodeErrorObject(t, rec)
	if errObj["retry_after_seconds"] != float64(42) {
		t.Errorf("retry_after_seconds = %v, want 42", errObj["retry_after_seconds"])
	}
	if rec.Header().Get("Retry-After") != "42" {
		t.Errorf("Retry-After header lost")
	}
}

func TestErrorEnvelope_SynthesizesEnvelopeForEmptyBody(t *testing.T) {
	// chi's Recoverer writes a bare 500 with no body after a panic.
	rec := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	})
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	errObj := decodeErrorObject(t, rec)
	if errObj["code"] != "INTERNAL_ERROR" || errObj["message"] != "Internal Server Error" || errObj["request_id"] != "req-123" {
		t.Errorf("synthesized envelope = %v", errObj)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
}

func TestErrorEnvelope_SynthesizesEnvelopeForPlainText(t *testing.T) {
	rec := serveEnveloped(t, http.MethodPost, func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "http: request body too large", http.StatusRequestEntityTooLarge)
	})
	errObj := decodeErrorObject(t, rec)
	if errObj["code"] != "PAYLOAD_TOO_LARGE" || errObj["message"] != "http: request body too large" {
		t.Errorf("synthesized envelope = %v", errObj)
	}
}

func TestErrorEnvelope_LeavesSuccessAndHTMLUntouched(t *testing.T) {
	ok := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"data":{"id":"1"}}`))
	})
	if ok.Body.String() != `{"data":{"id":"1"}}` {
		t.Errorf("success body changed: %q", ok.Body.String())
	}

	html := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("<p>bad link</p>"))
	})
	if html.Body.String() != "<p>bad link</p>" {
		t.Errorf("html error body changed: %q", html.Body.String())
	}
}

func TestErrorEnvelope_LeavesNonObjectErrorShapeUntouched(t *testing.T) {
	rec := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error":"MISSING_PARAMS","message":"x"}`))
	})
	if rec.Body.String() != `{"error":"MISSING_PARAMS","message":"x"}` {
		t.Errorf("non-object error body changed: %q", rec.Body.String())
	}
}

func TestErrorEnvelope_StreamsSuccessWithFlush(t *testing.T) {
	var flushed bool
	rec := serveEnveloped(t, http.MethodGet, func(w http.ResponseWriter, r *http.Request) {
		f, ok := w.(http.Flusher)
		if !ok {
			t.Fatal("ErrorEnvelope writer must implement http.Flusher for SSE")
		}
		w.Write([]byte("data: 1\n\n"))
		f.Flush()
		flushed = true
	})
	if !flushed || !rec.Flushed || rec.Body.String() != "data: 1\n\n" {
		t.Errorf("stream not passed through: flushed=%v recFlushed=%v body=%q", flushed, rec.Flushed, rec.Body.String())
	}
}

func TestErrorEnvelope_HeadErrorHasNoBody(t *testing.T) {
	rec := serveEnveloped(t, http.MethodHead, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	if rec.Code != http.StatusNotFound || rec.Body.Len() != 0 {
		t.Errorf("HEAD: status=%d body=%q", rec.Code, rec.Body.String())
	}
}
