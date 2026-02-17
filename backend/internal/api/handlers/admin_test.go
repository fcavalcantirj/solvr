package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

func TestAdminHandler_HardDeleteUser_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	// Create a user to delete
	userRepo := db.NewUserRepository(pool)
	ctx := context.Background()
	user := &models.User{
		Username:       "deletetest",
		DisplayName:    "Delete Test",
		Email:          "deletetest@example.com",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "deletetest_123",
		Role:           models.UserRoleUser,
	}
	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create user: %v", err)
	}

	// Set admin API key
	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	// Create handler
	handler := NewAdminHandler(pool)

	// Create request
	req := httptest.NewRequest(http.MethodDelete, "/admin/users/"+created.ID, nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	// Add chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", created.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()

	// Execute
	handler.HardDeleteUser(w, req)

	// Verify response
	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["message"] != "User permanently deleted" {
		t.Errorf("unexpected message: %v", resp)
	}

	// Verify user is hard-deleted
	_, err = userRepo.FindByID(ctx, created.ID)
	if err != db.ErrNotFound {
		t.Errorf("expected user to be hard-deleted, got err: %v", err)
	}
}

func TestAdminHandler_HardDeleteUser_Unauthorized(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	// Request without admin key
	req := httptest.NewRequest(http.MethodDelete, "/admin/users/123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.HardDeleteUser(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAdminHandler_HardDeleteUser_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/admin/users/nonexistent", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.HardDeleteUser(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAdminHandler_HardDeleteAgent_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	// Create an agent to delete
	agentRepo := db.NewAgentRepository(pool)
	ctx := context.Background()
	agent := &models.Agent{
		ID:          "deletetest_agent",
		DisplayName: "Delete Test Agent",
	}
	err := agentRepo.Create(ctx, agent)
	if err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/admin/agents/"+agent.ID, nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", agent.ID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.HardDeleteAgent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify agent is hard-deleted
	_, err = agentRepo.FindByID(ctx, agent.ID)
	if err != db.ErrAgentNotFound {
		t.Errorf("expected agent to be hard-deleted, got err: %v", err)
	}
}

func TestAdminHandler_HardDeleteAgent_Unauthorized(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/admin/agents/123", nil)
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "123")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.HardDeleteAgent(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAdminHandler_HardDeleteAgent_NotFound(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodDelete, "/admin/agents/nonexistent", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "nonexistent")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.HardDeleteAgent(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestAdminHandler_ListDeletedUsers_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/deleted?page=1&per_page=20", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	w := httptest.NewRecorder()
	handler.ListDeletedUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["meta"] == nil {
		t.Error("expected meta field in response")
	}
}

func TestAdminHandler_ListDeletedUsers_Pagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/deleted?page=2&per_page=5", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	w := httptest.NewRecorder()
	handler.ListDeletedUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	meta := resp["meta"].(map[string]interface{})
	if meta["page"].(float64) != 2 {
		t.Errorf("expected page 2, got %v", meta["page"])
	}
	if meta["per_page"].(float64) != 5 {
		t.Errorf("expected per_page 5, got %v", meta["per_page"])
	}
}

func TestAdminHandler_ListDeletedUsers_Unauthorized(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/admin/users/deleted", nil)
	// No admin key

	w := httptest.NewRecorder()
	handler.ListDeletedUsers(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestAdminHandler_ListDeletedAgents_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/admin/agents/deleted?page=1&per_page=20", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	w := httptest.NewRecorder()
	handler.ListDeletedAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	if resp["meta"] == nil {
		t.Error("expected meta field in response")
	}
}

func TestAdminHandler_ListDeletedAgents_Pagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/admin/agents/deleted?page=2&per_page=5", nil)
	req.Header.Set("X-Admin-API-Key", "test-admin-key")

	w := httptest.NewRecorder()
	handler.ListDeletedAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	meta := resp["meta"].(map[string]interface{})
	if meta["page"].(float64) != 2 {
		t.Errorf("expected page 2, got %v", meta["page"])
	}
	if meta["per_page"].(float64) != 5 {
		t.Errorf("expected per_page 5, got %v", meta["per_page"])
	}
}

func TestAdminHandler_ListDeletedAgents_Unauthorized(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	os.Setenv("ADMIN_API_KEY", "test-admin-key")
	defer os.Unsetenv("ADMIN_API_KEY")

	handler := NewAdminHandler(pool)

	req := httptest.NewRequest(http.MethodGet, "/admin/agents/deleted", nil)
	// No admin key

	w := httptest.NewRecorder()
	handler.ListDeletedAgents(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

// getTestPool returns a test database pool or skips the test if DATABASE_URL is not set.
func getTestPool(t *testing.T) *db.Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		return nil
	}

	ctx := context.Background()
	pool, err := db.NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	return pool
}
