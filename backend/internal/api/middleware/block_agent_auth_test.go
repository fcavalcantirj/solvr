package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestBlockAgentAPIKeys_NoAuthHeader(t *testing.T) {
	// Arrange
	handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil)
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
	if rec.Body.String() != "success" {
		t.Errorf("Expected 'success', got %s", rec.Body.String())
	}
}

func TestBlockAgentAPIKeys_EmptyAuthHeader(t *testing.T) {
	// Arrange
	handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil)
	req.Header.Set("Authorization", "")
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestBlockAgentAPIKeys_BearerTokenNotAgent(t *testing.T) {
	// Arrange
	handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("success"))
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil)
	req.Header.Set("Authorization", "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9...")
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

func TestBlockAgentAPIKeys_AgentAPIKeyBlocked(t *testing.T) {
	// Arrange
	handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called when agent API key is present")
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil)
	req.Header.Set("Authorization", "Bearer solvr_test_key_123")
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", rec.Code)
	}

	// Check response body contains helpful error message
	body := rec.Body.String()
	if body == "" {
		t.Error("Expected error message in response body")
	}
}

func TestBlockAgentAPIKeys_AgentAPIKeyVariations(t *testing.T) {
	testCases := []struct {
		name   string
		header string
		expect int
	}{
		{
			name:   "solvr_ prefix lowercase",
			header: "Bearer solvr_abc123",
			expect: http.StatusForbidden,
		},
		{
			name:   "solvr_ prefix with special chars",
			header: "Bearer solvr_abc-123_xyz",
			expect: http.StatusForbidden,
		},
		{
			name:   "SOLVR_ prefix uppercase",
			header: "Bearer SOLVR_abc123",
			expect: http.StatusForbidden,
		},
		{
			name:   "Mixed case solvr_",
			header: "Bearer SoLvR_abc123",
			expect: http.StatusForbidden,
		},
		{
			name:   "Non-agent bearer token",
			header: "Bearer jwt_token_here",
			expect: http.StatusOK,
		},
		{
			name:   "No Bearer prefix",
			header: "solvr_abc123",
			expect: http.StatusOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			}))

			req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil)
			req.Header.Set("Authorization", tc.header)
			rec := httptest.NewRecorder()

			// Act
			handler.ServeHTTP(rec, req)

			// Assert
			if rec.Code != tc.expect {
				t.Errorf("Expected status %d, got %d", tc.expect, rec.Code)
			}
		})
	}
}

func TestBlockAgentAPIKeys_BasicAuthNotBlocked(t *testing.T) {
	// Arrange
	handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/register", nil)
	req.Header.Set("Authorization", "Basic dXNlcjpwYXNz") // Basic auth should not be blocked
	rec := httptest.NewRecorder()

	// Act
	handler.ServeHTTP(rec, req)

	// Assert
	if rec.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", rec.Code)
	}
}

// A blocked agent key gets the public error envelope, {"error": {"code", "message", ...}},
// so clients read one error shape on every route. The body must not be a flat object.
func TestBlockAgentAPIKeys_ForbiddenUsesTheErrorEnvelope(t *testing.T) {
	handler := BlockAgentAPIKeys(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("Handler should not be called when agent API key is present")
	}))

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/login", nil)
	req.Header.Set("Authorization", "Bearer solvr_test_key_123")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("Expected status 403, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %q", ct)
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("Expected a JSON body, got %q: %v", rec.Body.String(), err)
	}
	if len(body) != 1 {
		t.Errorf("Expected only the error envelope at the top level, got %s", rec.Body.String())
	}
	var envelope struct {
		Code    string `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal(body["error"], &envelope); err != nil {
		t.Fatalf("Expected an error object, got %s: %v", rec.Body.String(), err)
	}
	if envelope.Code != "FORBIDDEN" {
		t.Errorf("Expected error.code FORBIDDEN, got %q", envelope.Code)
	}
	if !strings.Contains(envelope.Message, "POST /v1/agents/register") {
		t.Errorf("Expected error.message to point agents at POST /v1/agents/register, got %q", envelope.Message)
	}
}
