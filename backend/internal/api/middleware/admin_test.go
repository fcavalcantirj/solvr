// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// TestAdminMiddleware_Success verifies admin role allows access
func TestAdminMiddleware_Success(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	w := httptest.NewRecorder()

	// Add admin claims to context
	claims := &auth.Claims{
		UserID: "user-123",
		Email:  "admin@example.com",
		Role:   "admin",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// TestAdminMiddleware_NonAdminForbidden verifies non-admin role is rejected
func TestAdminMiddleware_NonAdminForbidden(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for non-admin")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	w := httptest.NewRecorder()

	// Add regular user claims to context
	claims := &auth.Claims{
		UserID: "user-456",
		Email:  "user@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}

	// Verify error response format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "FORBIDDEN" {
		t.Errorf("expected error code 'FORBIDDEN', got '%v'", errObj["code"])
	}
}

// TestAdminMiddleware_NoClaims verifies missing claims returns 401
func TestAdminMiddleware_NoClaims(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called without claims")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	w := httptest.NewRecorder()

	// No claims in context
	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	// Verify error response format
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	errObj, ok := response["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}

	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code 'UNAUTHORIZED', got '%v'", errObj["code"])
	}
}

// TestAdminMiddleware_EmptyRole verifies empty role is rejected
func TestAdminMiddleware_EmptyRole(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called with empty role")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	w := httptest.NewRecorder()

	// Add claims with empty role
	claims := &auth.Claims{
		UserID: "user-789",
		Email:  "test@example.com",
		Role:   "",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestAdminMiddleware_ModeratorNotAdmin verifies moderator role doesn't have admin access
func TestAdminMiddleware_ModeratorNotAdmin(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called for moderator")
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	w := httptest.NewRecorder()

	// Add moderator claims to context
	claims := &auth.Claims{
		UserID: "user-mod",
		Email:  "mod@example.com",
		Role:   "moderator",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// TestAdminMiddleware_ContentTypeJSON verifies error responses are JSON
func TestAdminMiddleware_ContentTypeJSON(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("handler should not be called")
	}))

	tests := []struct {
		name   string
		claims *auth.Claims
	}{
		{"no claims", nil},
		{"non-admin", &auth.Claims{UserID: "user-1", Role: "user"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
			w := httptest.NewRecorder()

			if tt.claims != nil {
				ctx := auth.ContextWithClaims(req.Context(), tt.claims)
				req = req.WithContext(ctx)
			}

			handler.ServeHTTP(w, req)

			contentType := w.Header().Get("Content-Type")
			if contentType != "application/json" {
				t.Errorf("expected Content-Type 'application/json', got '%s'", contentType)
			}
		})
	}
}

// TestAdminMiddleware_SuperAdminAccess verifies super_admin role has access
func TestAdminMiddleware_SuperAdminAccess(t *testing.T) {
	handler := AdminOnly(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"success":true}`))
	}))

	req := httptest.NewRequest(http.MethodGet, "/admin/test", nil)
	w := httptest.NewRecorder()

	// Add super_admin claims - should also have access per SPEC.md Part 16.4
	claims := &auth.Claims{
		UserID: "super-admin-1",
		Email:  "superadmin@example.com",
		Role:   "super_admin",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	req = req.WithContext(ctx)

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for super_admin, got %d", w.Code)
	}
}

// TestAdminMiddleware_AdminOrAbove tests the AdminOrAbove helper function
func TestAdminMiddleware_AdminOrAbove(t *testing.T) {
	tests := []struct {
		role     string
		expected bool
	}{
		{"admin", true},
		{"super_admin", true},
		{"user", false},
		{"moderator", false},
		{"", false},
		{"ADMIN", false}, // case sensitive
	}

	for _, tt := range tests {
		t.Run(tt.role, func(t *testing.T) {
			result := IsAdminOrAbove(tt.role)
			if result != tt.expected {
				t.Errorf("IsAdminOrAbove(%q) = %v, expected %v", tt.role, result, tt.expected)
			}
		})
	}
}
