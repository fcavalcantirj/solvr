package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// TestErrorEnvelopeContract pins the public error envelope at the router
// boundary: every error carries a stable code, a message, and the same
// request_id the response advertises in X-Request-ID.
func TestErrorEnvelopeContract(t *testing.T) {
	router := NewRouter(nil, nil, nil)

	cases := []struct {
		name     string
		method   string
		path     string
		clientID string
		status   int
		code     string
	}{
		{"unknown route generated id", http.MethodGet, "/v1/definitely-not-a-route", "", http.StatusNotFound, "NOT_FOUND"},
		{"unknown route client id", http.MethodGet, "/v1/definitely-not-a-route", "client-supplied-7", http.StatusNotFound, "NOT_FOUND"},
		{"method not allowed", http.MethodDelete, "/health", "", http.StatusMethodNotAllowed, "METHOD_NOT_ALLOWED"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			if tc.clientID != "" {
				req.Header.Set("X-Request-ID", tc.clientID)
			}
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)

			if w.Code != tc.status {
				t.Fatalf("status = %d, want %d", w.Code, tc.status)
			}
			headerID := w.Header().Get("X-Request-ID")
			if headerID == "" {
				t.Fatal("X-Request-ID header missing")
			}
			if tc.clientID != "" && headerID != tc.clientID {
				t.Errorf("X-Request-ID = %q, want client value %q", headerID, tc.clientID)
			}

			var body struct {
				Error struct {
					Code      string `json:"code"`
					Message   string `json:"message"`
					RequestID string `json:"request_id"`
				} `json:"error"`
			}
			if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
				t.Fatalf("error body not JSON: %v (%q)", err, w.Body.String())
			}
			if body.Error.Code != tc.code || body.Error.Message == "" {
				t.Errorf("error = %+v, want code %s with a message", body.Error, tc.code)
			}
			if body.Error.RequestID != headerID {
				t.Errorf("error.request_id = %q, want X-Request-ID %q", body.Error.RequestID, headerID)
			}
		})
	}
}
