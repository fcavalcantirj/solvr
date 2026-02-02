// Package handlers provides HTTP handlers for the Solvr API.
package handlers

// E2E tests for Admin Workflows (PRD line 5080-5088)

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// E2E Tests: Flag Management and Audit Log
// ============================================================================

// TestE2E_Admin_ViewAndListFlags tests the complete flag viewing flow.
func TestE2E_Admin_ViewAndListFlags(t *testing.T) {
	flagID1 := uuid.New()
	flagID2 := uuid.New()
	targetID := uuid.New()

	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID1,
				TargetType:   "post",
				TargetID:     targetID,
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
				CreatedAt:    time.Now().Add(-2 * time.Hour),
			},
			{
				ID:           flagID2,
				TargetType:   "comment",
				TargetID:     uuid.New(),
				ReporterType: "system",
				ReporterID:   "auto-detect",
				Reason:       "offensive",
				Status:       "pending",
				CreatedAt:    time.Now().Add(-1 * time.Hour),
			},
		},
	}

	handler := NewAdminHandler(repo)

	adminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	req := httptest.NewRequest("GET", "/v1/admin/flags", nil)
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ListFlags(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify total count
	if total, ok := resp["total"].(float64); !ok || int(total) != 2 {
		t.Errorf("Expected total 2, got %v", resp["total"])
	}

	// Verify data array
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}
	if len(data) != 2 {
		t.Errorf("Expected 2 flags, got %d", len(data))
	}
}

// TestE2E_Admin_DismissFlagAndAuditLog tests dismissing a flag and verifying audit log.
func TestE2E_Admin_DismissFlagAndAuditLog(t *testing.T) {
	flagID := uuid.New()
	targetID := uuid.New()

	var auditLogEntries []models.AuditLog
	var updatedFlag *models.Flag

	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID,
				TargetType:   "post",
				TargetID:     targetID,
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
		},
		UpdateFlagFunc: func(ctx context.Context, flag *models.Flag) error {
			updatedFlag = flag
			return nil
		},
		CreateAuditLogFunc: func(ctx context.Context, entry *models.AuditLog) error {
			auditLogEntries = append(auditLogEntries, *entry)
			return nil
		},
	}

	handler := NewAdminHandler(repo)

	adminID := uuid.New()
	adminClaims := &auth.Claims{
		UserID: adminID.String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	// Create router to handle URL params
	r := chi.NewRouter()
	r.Post("/v1/admin/flags/{id}/dismiss", handler.DismissFlag)

	req := httptest.NewRequest("POST", "/v1/admin/flags/"+flagID.String()+"/dismiss", nil)
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify flag was updated
	if updatedFlag == nil {
		t.Fatal("Expected flag to be updated")
	}
	if updatedFlag.Status != "dismissed" {
		t.Errorf("Expected flag status 'dismissed', got '%s'", updatedFlag.Status)
	}
	if updatedFlag.ReviewedBy == nil {
		t.Error("Expected ReviewedBy to be set")
	}
	if updatedFlag.ReviewedAt == nil {
		t.Error("Expected ReviewedAt to be set")
	}

	// Verify audit log entry was created
	if len(auditLogEntries) != 1 {
		t.Fatalf("Expected 1 audit log entry, got %d", len(auditLogEntries))
	}

	auditEntry := auditLogEntries[0]
	if auditEntry.Action != "dismiss_flag" {
		t.Errorf("Expected action 'dismiss_flag', got '%s'", auditEntry.Action)
	}
	if auditEntry.AdminID.String() != adminID.String() {
		t.Errorf("Expected admin ID %s, got %s", adminID, auditEntry.AdminID)
	}
	if auditEntry.TargetType != "flag" {
		t.Errorf("Expected target type 'flag', got '%s'", auditEntry.TargetType)
	}
	if auditEntry.TargetID == nil || auditEntry.TargetID.String() != flagID.String() {
		t.Errorf("Expected target ID %s", flagID)
	}
}

// TestE2E_Admin_ActionOnFlagAndAuditLog tests taking action on a flag and verifying audit log.
func TestE2E_Admin_ActionOnFlagAndAuditLog(t *testing.T) {
	flagID := uuid.New()
	targetID := uuid.New()

	var auditLogEntries []models.AuditLog
	var updatedFlag *models.Flag

	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID,
				TargetType:   "post",
				TargetID:     targetID,
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
		},
		UpdateFlagFunc: func(ctx context.Context, flag *models.Flag) error {
			updatedFlag = flag
			return nil
		},
		CreateAuditLogFunc: func(ctx context.Context, entry *models.AuditLog) error {
			auditLogEntries = append(auditLogEntries, *entry)
			return nil
		},
	}

	handler := NewAdminHandler(repo)

	adminID := uuid.New()
	adminClaims := &auth.Claims{
		UserID: adminID.String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	// Create router to handle URL params
	r := chi.NewRouter()
	r.Post("/v1/admin/flags/{id}/action", handler.ActionOnFlag)

	// Test with valid action
	body := `{"action": "hide"}`
	req := httptest.NewRequest("POST", "/v1/admin/flags/"+flagID.String()+"/action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
	}

	// Verify flag was updated
	if updatedFlag == nil {
		t.Fatal("Expected flag to be updated")
	}
	if updatedFlag.Status != "actioned" {
		t.Errorf("Expected flag status 'actioned', got '%s'", updatedFlag.Status)
	}

	// Verify audit log entry was created
	if len(auditLogEntries) != 1 {
		t.Fatalf("Expected 1 audit log entry, got %d", len(auditLogEntries))
	}

	auditEntry := auditLogEntries[0]
	if auditEntry.Action != "action_flag" {
		t.Errorf("Expected action 'action_flag', got '%s'", auditEntry.Action)
	}
	if auditEntry.Details == nil {
		t.Error("Expected audit details to be set")
	} else {
		if actionDetail, ok := auditEntry.Details["action"]; !ok || actionDetail != "hide" {
			t.Errorf("Expected audit details action 'hide', got %v", auditEntry.Details)
		}
	}
}

// TestE2E_Admin_ActionOnFlagInvalidAction tests that invalid actions are rejected.
func TestE2E_Admin_ActionOnFlagInvalidAction(t *testing.T) {
	flagID := uuid.New()

	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:         flagID,
				TargetType: "post",
				TargetID:   uuid.New(),
				Status:     "pending",
			},
		},
	}

	handler := NewAdminHandler(repo)

	adminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	r := chi.NewRouter()
	r.Post("/v1/admin/flags/{id}/action", handler.ActionOnFlag)

	// Test with invalid action
	body := `{"action": "invalid_action"}`
	req := httptest.NewRequest("POST", "/v1/admin/flags/"+flagID.String()+"/action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("Expected 400 Bad Request, got %d", rec.Code)
	}

	var errResp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	errObj, ok := errResp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected error object in response")
	}
	if errObj["code"] != "INVALID_ACTION" {
		t.Errorf("Expected error code INVALID_ACTION, got %v", errObj["code"])
	}
}

// TestE2E_Admin_FlagNotFound tests that dismissing/acting on a non-existent flag returns 404.
func TestE2E_Admin_FlagNotFound(t *testing.T) {
	repo := &MockAdminRepository{
		Flags: []models.Flag{}, // No flags
	}

	handler := NewAdminHandler(repo)

	adminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	r := chi.NewRouter()
	r.Post("/v1/admin/flags/{id}/dismiss", handler.DismissFlag)
	r.Post("/v1/admin/flags/{id}/action", handler.ActionOnFlag)

	nonExistentID := uuid.New().String()

	tests := []struct {
		name string
		path string
		body string
	}{
		{"DismissNotFound", "/v1/admin/flags/" + nonExistentID + "/dismiss", ""},
		{"ActionNotFound", "/v1/admin/flags/" + nonExistentID + "/action", `{"action": "hide"}`},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest("POST", tc.path, strings.NewReader(tc.body))
			req.Header.Set("Content-Type", "application/json")
			ctx := auth.ContextWithClaims(req.Context(), adminClaims)
			req = req.WithContext(ctx)
			rec := httptest.NewRecorder()

			r.ServeHTTP(rec, req)

			if rec.Code != http.StatusNotFound {
				t.Errorf("Expected 404 Not Found, got %d", rec.Code)
			}

			var errResp map[string]interface{}
			if err := json.Unmarshal(rec.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("Failed to parse error response: %v", err)
			}

			errObj, ok := errResp["error"].(map[string]interface{})
			if !ok {
				t.Fatal("Expected error object in response")
			}
			if errObj["code"] != "NOT_FOUND" {
				t.Errorf("Expected error code NOT_FOUND, got %v", errObj["code"])
			}
		})
	}
}

// TestE2E_Admin_AuditLogListWithFilters tests listing audit log with filters.
func TestE2E_Admin_AuditLogListWithFilters(t *testing.T) {
	adminID := uuid.New()
	targetID := uuid.New()

	repo := &MockAdminRepository{
		AuditEntries: []models.AuditLog{
			{
				ID:         uuid.New(),
				AdminID:    adminID,
				Action:     "dismiss_flag",
				TargetType: "flag",
				TargetID:   &targetID,
				CreatedAt:  time.Now().Add(-1 * time.Hour),
			},
			{
				ID:         uuid.New(),
				AdminID:    adminID,
				Action:     "action_flag",
				TargetType: "flag",
				TargetID:   &targetID,
				Details:    map[string]interface{}{"action": "hide"},
				CreatedAt:  time.Now().Add(-30 * time.Minute),
			},
			{
				ID:         uuid.New(),
				AdminID:    adminID,
				Action:     "suspend_user",
				TargetType: "user",
				TargetID:   &targetID,
				CreatedAt:  time.Now().Add(-15 * time.Minute),
			},
		},
	}

	handler := NewAdminHandler(repo)

	adminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	req := httptest.NewRequest("GET", "/v1/admin/audit", nil)
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.ListAuditLog(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify total count
	if total, ok := resp["total"].(float64); !ok || int(total) != 3 {
		t.Errorf("Expected total 3, got %v", resp["total"])
	}

	// Verify data array
	data, ok := resp["data"].([]interface{})
	if !ok {
		t.Fatal("Expected data to be an array")
	}
	if len(data) != 3 {
		t.Errorf("Expected 3 audit entries, got %d", len(data))
	}
}

// TestE2E_Admin_GetStats tests retrieving admin statistics.
func TestE2E_Admin_GetStats(t *testing.T) {
	repo := &MockAdminRepository{
		StatsFunc: func(ctx context.Context) (*models.AdminStats, error) {
			return &models.AdminStats{
				UsersCount:     150,
				AgentsCount:    75,
				PostsCount:     1200,
				RateLimitHits:  25,
				FlagsCount:     12,
				ActiveUsers24h: 45,
			}, nil
		},
	}

	handler := NewAdminHandler(repo)

	adminClaims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	req := httptest.NewRequest("GET", "/v1/admin/stats", nil)
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()

	handler.GetStats(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Expected 200 OK, got %d", rec.Code)
	}

	var resp map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("Expected data to be an object")
	}

	// Verify stats fields
	if users, ok := data["users_count"].(float64); !ok || int(users) != 150 {
		t.Errorf("Expected users_count 150, got %v", data["users_count"])
	}
	if agents, ok := data["agents_count"].(float64); !ok || int(agents) != 75 {
		t.Errorf("Expected agents_count 75, got %v", data["agents_count"])
	}
}

// TestE2E_Admin_UserManagement tests complete user management flow.
func TestE2E_Admin_UserManagement(t *testing.T) {
	userID := uuid.New().String()

	var updatedUser *models.User
	var auditLogEntries []models.AuditLog

	repo := &MockAdminRepository{
		Users: []models.User{
			{
				ID:          userID,
				Username:    "testuser",
				DisplayName: "Test User",
				Email:       "test@example.com",
				Status:      "active",
			},
		},
		UpdateUserFunc: func(ctx context.Context, user *models.User) error {
			updatedUser = user
			return nil
		},
		CreateAuditLogFunc: func(ctx context.Context, entry *models.AuditLog) error {
			auditLogEntries = append(auditLogEntries, *entry)
			return nil
		},
	}

	handler := NewAdminHandler(repo)

	adminID := uuid.New()
	adminClaims := &auth.Claims{
		UserID: adminID.String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	r := chi.NewRouter()
	r.Get("/v1/admin/users/{id}", handler.GetUserDetail)
	r.Post("/v1/admin/users/{id}/warn", handler.WarnUser)
	r.Post("/v1/admin/users/{id}/suspend", handler.SuspendUser)
	r.Post("/v1/admin/users/{id}/ban", handler.BanUser)

	// Test: Get user detail
	t.Run("GetUserDetail", func(t *testing.T) {
		req := httptest.NewRequest("GET", "/v1/admin/users/"+userID, nil)
		ctx := auth.ContextWithClaims(req.Context(), adminClaims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d", rec.Code)
		}

		var resp map[string]interface{}
		if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
			t.Fatalf("Failed to parse response: %v", err)
		}

		data, ok := resp["data"].(map[string]interface{})
		if !ok {
			t.Fatal("Expected data to be an object")
		}
		if data["username"] != "testuser" {
			t.Errorf("Expected username 'testuser', got %v", data["username"])
		}
	})

	// Test: Warn user
	t.Run("WarnUser", func(t *testing.T) {
		auditLogEntries = nil // Reset
		body := `{"message": "Please follow our community guidelines"}`
		req := httptest.NewRequest("POST", "/v1/admin/users/"+userID+"/warn", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.ContextWithClaims(req.Context(), adminClaims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify audit log
		if len(auditLogEntries) != 1 {
			t.Fatalf("Expected 1 audit log entry, got %d", len(auditLogEntries))
		}
		if auditLogEntries[0].Action != "warn_user" {
			t.Errorf("Expected action 'warn_user', got '%s'", auditLogEntries[0].Action)
		}
	})

	// Test: Suspend user
	t.Run("SuspendUser", func(t *testing.T) {
		auditLogEntries = nil // Reset
		body := `{"duration": "24h", "reason": "Spam behavior"}`
		req := httptest.NewRequest("POST", "/v1/admin/users/"+userID+"/suspend", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.ContextWithClaims(req.Context(), adminClaims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify user was updated
		if updatedUser == nil {
			t.Fatal("Expected user to be updated")
		}
		if updatedUser.Status != "suspended" {
			t.Errorf("Expected status 'suspended', got '%s'", updatedUser.Status)
		}

		// Verify audit log
		if len(auditLogEntries) != 1 {
			t.Fatalf("Expected 1 audit log entry, got %d", len(auditLogEntries))
		}
		if auditLogEntries[0].Action != "suspend_user" {
			t.Errorf("Expected action 'suspend_user', got '%s'", auditLogEntries[0].Action)
		}
	})

	// Test: Ban user
	t.Run("BanUser", func(t *testing.T) {
		auditLogEntries = nil // Reset
		body := `{"reason": "Repeated violations"}`
		req := httptest.NewRequest("POST", "/v1/admin/users/"+userID+"/ban", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		ctx := auth.ContextWithClaims(req.Context(), adminClaims)
		req = req.WithContext(ctx)
		rec := httptest.NewRecorder()

		r.ServeHTTP(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("Expected 200 OK, got %d: %s", rec.Code, rec.Body.String())
		}

		// Verify user was updated
		if updatedUser.Status != "banned" {
			t.Errorf("Expected status 'banned', got '%s'", updatedUser.Status)
		}

		// Verify audit log
		if len(auditLogEntries) != 1 {
			t.Fatalf("Expected 1 audit log entry, got %d", len(auditLogEntries))
		}
		if auditLogEntries[0].Action != "ban_user" {
			t.Errorf("Expected action 'ban_user', got '%s'", auditLogEntries[0].Action)
		}
	})
}

// TestE2E_Admin_CompleteWorkflow tests a complete admin workflow from login to action.
func TestE2E_Admin_CompleteWorkflow(t *testing.T) {
	// This test simulates a complete admin workflow:
	// 1. Admin authenticates (simulated via JWT claims)
	// 2. Admin views pending flags
	// 3. Admin takes action on a flag
	// 4. Admin verifies action in audit log

	flagID := uuid.New()
	targetID := uuid.New()
	adminID := uuid.New()

	var auditLogEntries []models.AuditLog

	// Create the repo first, then add the functions that reference it
	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID,
				TargetType:   "post",
				TargetID:     targetID,
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
		},
	}

	// Set functions after repo is created to avoid closure issues
	repo.UpdateFlagFunc = func(ctx context.Context, flag *models.Flag) error {
		// Update the flag in the mock store
		for i := range repo.Flags {
			if repo.Flags[i].ID == flag.ID {
				repo.Flags[i] = *flag
				break
			}
		}
		return nil
	}
	repo.CreateAuditLogFunc = func(ctx context.Context, entry *models.AuditLog) error {
		auditLogEntries = append(auditLogEntries, *entry)
		repo.AuditEntries = auditLogEntries
		return nil
	}

	handler := NewAdminHandler(repo)

	adminClaims := &auth.Claims{
		UserID: adminID.String(),
		Email:  "admin@solvr.dev",
		Role:   "admin",
	}

	r := chi.NewRouter()
	r.Get("/v1/admin/flags", handler.ListFlags)
	r.Post("/v1/admin/flags/{id}/action", handler.ActionOnFlag)
	r.Get("/v1/admin/audit", handler.ListAuditLog)

	// Step 1: View pending flags
	t.Log("Step 1: View pending flags")
	req := httptest.NewRequest("GET", "/v1/admin/flags?status=pending", nil)
	ctx := auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Failed to list flags: %d", rec.Code)
	}

	var flagsResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &flagsResp)
	if total, ok := flagsResp["total"].(float64); !ok || int(total) < 1 {
		t.Fatal("Expected at least 1 pending flag")
	}
	t.Log("  Found pending flags")

	// Step 2: Take action on the flag
	t.Log("Step 2: Take action on flag (delete)")
	body := `{"action": "delete"}`
	req = httptest.NewRequest("POST", "/v1/admin/flags/"+flagID.String()+"/action", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	ctx = auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Failed to action flag: %d - %s", rec.Code, rec.Body.String())
	}
	t.Log("  Action taken successfully")

	// Step 3: Verify in audit log
	t.Log("Step 3: Verify action in audit log")
	req = httptest.NewRequest("GET", "/v1/admin/audit", nil)
	ctx = auth.ContextWithClaims(req.Context(), adminClaims)
	req = req.WithContext(ctx)
	rec = httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("Failed to list audit log: %d", rec.Code)
	}

	var auditResp map[string]interface{}
	json.Unmarshal(rec.Body.Bytes(), &auditResp)

	data, ok := auditResp["data"].([]interface{})
	if !ok || len(data) < 1 {
		t.Fatal("Expected at least 1 audit log entry")
	}

	// Verify the audit entry
	entry := data[0].(map[string]interface{})
	if entry["action"] != "action_flag" {
		t.Errorf("Expected action 'action_flag', got %v", entry["action"])
	}
	t.Log("  Audit log verified")

	t.Log("Complete admin workflow test passed!")
}
