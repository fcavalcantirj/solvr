// Package handlers provides HTTP handlers for the Solvr API.
package handlers

/**
 * E2E tests for Admin Authentication and Authorization
 *
 * Per PRD line 5080-5088:
 * - E2E: Admin flows
 * - Admin login
 *
 * This file tests:
 * 1. Admin authentication (JWT with admin/super_admin role)
 * 2. Non-admin users get 403 Forbidden
 * 3. Unauthenticated requests get 401 Unauthorized
 * 4. AdminOnly middleware integration
 */

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/api/middleware"
	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// E2E Tests: Admin Authentication and Authorization
// ============================================================================

// TestE2E_Admin_AuthenticationRequired tests that admin endpoints require authentication.
// Non-authenticated requests should receive 401 Unauthorized.
func TestE2E_Admin_AuthenticationRequired(t *testing.T) {
	repo := &MockAdminRepository{
		Flags: []models.Flag{},
	}
	handler := NewAdminHandler(repo)

	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{"ListFlags", "GET", "/v1/admin/flags", handler.ListFlags},
		{"ListUsers", "GET", "/v1/admin/users", handler.ListUsers},
		{"ListAgents", "GET", "/v1/admin/agents", handler.ListAgents},
		{"ListAuditLog", "GET", "/v1/admin/audit", handler.ListAuditLog},
		{"GetStats", "GET", "/v1/admin/stats", handler.GetStats},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			// No authentication context added
			rec := httptest.NewRecorder()

			tc.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusUnauthorized {
				t.Errorf("Expected 401 Unauthorized, got %d", rec.Code)
			}

			// Verify response structure
			var errResp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			errObj, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected error object in response")
			}
			if errObj["code"] != "UNAUTHORIZED" {
				t.Errorf("Expected error code UNAUTHORIZED, got %v", errObj["code"])
			}
		})
	}
}

// TestE2E_Admin_NonAdminForbidden tests that non-admin users receive 403 Forbidden.
func TestE2E_Admin_NonAdminForbidden(t *testing.T) {
	repo := &MockAdminRepository{
		Flags: []models.Flag{},
	}
	handler := NewAdminHandler(repo)

	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{"ListFlags", "GET", "/v1/admin/flags", handler.ListFlags},
		{"ListUsers", "GET", "/v1/admin/users", handler.ListUsers},
		{"ListAgents", "GET", "/v1/admin/agents", handler.ListAgents},
		{"ListAuditLog", "GET", "/v1/admin/audit", handler.ListAuditLog},
		{"GetStats", "GET", "/v1/admin/stats", handler.GetStats},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			// Add regular user context (not admin)
			req = addUserContext(req)
			rec := httptest.NewRecorder()

			tc.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusForbidden {
				t.Errorf("Expected 403 Forbidden, got %d", rec.Code)
			}

			// Verify response structure
			var errResp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			errObj, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected error object in response")
			}
			if errObj["code"] != "FORBIDDEN" {
				t.Errorf("Expected error code FORBIDDEN, got %v", errObj["code"])
			}
		})
	}
}

// TestE2E_Admin_LoginFlow tests complete admin authentication flow.
// Admin users should be able to access admin endpoints successfully.
func TestE2E_Admin_LoginFlow(t *testing.T) {
	repo := &MockAdminRepository{
		Flags:  []models.Flag{},
		Users:  []models.User{},
		Agents: []models.Agent{},
	}
	handler := NewAdminHandler(repo)

	// Test that admin can access protected endpoints
	adminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	tests := []struct {
		name    string
		method  string
		path    string
		handler http.HandlerFunc
	}{
		{"ListFlags", "GET", "/v1/admin/flags", handler.ListFlags},
		{"ListUsers", "GET", "/v1/admin/users", handler.ListUsers},
		{"ListAgents", "GET", "/v1/admin/agents", handler.ListAgents},
		{"ListAuditLog", "GET", "/v1/admin/audit", handler.ListAuditLog},
		{"GetStats", "GET", "/v1/admin/stats", handler.GetStats},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			ctx := auth.ContextWithClaims(req.Context(), adminClaims)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			tc.handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusOK {
				t.Errorf("Expected 200 OK, got %d", rec.Code)
			}

			// Verify response is valid JSON
			var resp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
				t.Fatalf("Failed to parse response: %v", err)
			}

			// Verify data field exists
			if _, ok := resp["data"]; !ok {
				t.Error("Expected data field in response")
			}
		})
	}
}

// TestE2E_Admin_SuperAdminAccess tests that super_admin role has admin access.
func TestE2E_Admin_SuperAdminAccess(t *testing.T) {
	repo := &MockAdminRepository{
		Flags: []models.Flag{},
	}
	handler := NewAdminHandler(repo)

	superAdminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "super@solvr.dev",
		Role:   "super_admin",
	}

	req := httptest.NewRequest("GET", "/v1/admin/flags", nil)
	ctx := auth.ContextWithClaims(req.Context(), superAdminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ListFlags(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("Expected 200 OK for super_admin, got %d", rec.Code)
	}
}

// TestE2E_Admin_MiddlewareIntegration tests that AdminOnly middleware works correctly.
func TestE2E_Admin_MiddlewareIntegration(t *testing.T) {
	testHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "success"})
	})

	protectedHandler := middleware.AdminOnly(testHandler)

	tests := []struct {
		name           string
		claims         *auth.Claims
		expectedStatus int
		expectedCode   string
	}{
		{
			name:           "NoAuth",
			claims:         nil,
			expectedStatus: http.StatusUnauthorized,
			expectedCode:   "UNAUTHORIZED",
		},
		{
			name: "RegularUser",
			claims: &auth.Claims{
				UserID: uuid.New().String(),
				Role:   "user",
			},
			expectedStatus: http.StatusForbidden,
			expectedCode:   "FORBIDDEN",
		},
		{
			name: "Admin",
			claims: &auth.Claims{
				UserID: uuid.New().String(),
				Role:   "admin",
			},
			expectedStatus: http.StatusOK,
			expectedCode:   "",
		},
		{
			name: "SuperAdmin",
			claims: &auth.Claims{
				UserID: uuid.New().String(),
				Role:   "super_admin",
			},
			expectedStatus: http.StatusOK,
			expectedCode:   "",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", "/v1/admin/test", nil)
			if tc.claims != nil {
				ctx := auth.ContextWithClaims(req.Context(), tc.claims)
				req = req.WithContext(ctx)
			}
			rec := httptest.NewRecorder()

			protectedHandler.ServeHTTP(rec, req)

			if rec.Code != tc.expectedStatus {
				t.Errorf("Expected status %d, got %d", tc.expectedStatus, rec.Code)
			}

			if tc.expectedCode != "" {
				var errResp map[string]interface{}
				if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
					t.Fatalf("Failed to parse error response: %v", err)
				}
				errObj, ok := errResp["error"].(map[string]interface{})
				if !ok {
					t.Fatal("Expected error object in response")
				}
				if errObj["code"] != tc.expectedCode {
					t.Errorf("Expected error code %s, got %v", tc.expectedCode, errObj["code"])
				}
			}
		})
	}
}
