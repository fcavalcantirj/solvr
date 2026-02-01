// Package handlers provides HTTP handlers for the Solvr API.
package handlers

import (
	"context"
	"encoding/json"
	"errors"
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

// MockAdminRepository implements AdminRepositoryInterface for testing
type MockAdminRepository struct {
	Flags        []models.Flag
	Users        []models.User
	Agents       []models.Agent
	AuditEntries []models.AuditLog
	FlagByIDFunc func(ctx context.Context, id string) (*models.Flag, error)
	UpdateFlagFunc func(ctx context.Context, flag *models.Flag) error
	CreateAuditLogFunc func(ctx context.Context, entry *models.AuditLog) error
	GetUserByIDFunc func(ctx context.Context, id string) (*models.User, error)
	UpdateUserFunc func(ctx context.Context, user *models.User) error
	GetAgentByIDFunc func(ctx context.Context, id string) (*models.Agent, error)
	UpdateAgentFunc func(ctx context.Context, agent *models.Agent) error
	HardDeletePostFunc func(ctx context.Context, id string) error
	RestorePostFunc func(ctx context.Context, id string) error
	StatsFunc func(ctx context.Context) (*models.AdminStats, error)
}

func (m *MockAdminRepository) ListFlags(ctx context.Context, opts *models.FlagListOptions) ([]models.Flag, int, error) {
	return m.Flags, len(m.Flags), nil
}

func (m *MockAdminRepository) GetFlagByID(ctx context.Context, id string) (*models.Flag, error) {
	if m.FlagByIDFunc != nil {
		return m.FlagByIDFunc(ctx, id)
	}
	for _, f := range m.Flags {
		if f.ID.String() == id {
			return &f, nil
		}
	}
	return nil, errors.New("flag not found")
}

func (m *MockAdminRepository) UpdateFlag(ctx context.Context, flag *models.Flag) error {
	if m.UpdateFlagFunc != nil {
		return m.UpdateFlagFunc(ctx, flag)
	}
	return nil
}

func (m *MockAdminRepository) CreateAuditLog(ctx context.Context, entry *models.AuditLog) error {
	if m.CreateAuditLogFunc != nil {
		return m.CreateAuditLogFunc(ctx, entry)
	}
	m.AuditEntries = append(m.AuditEntries, *entry)
	return nil
}

func (m *MockAdminRepository) ListUsers(ctx context.Context, opts *models.UserListOptions) ([]models.User, int, error) {
	return m.Users, len(m.Users), nil
}

func (m *MockAdminRepository) GetUserByID(ctx context.Context, id string) (*models.User, error) {
	if m.GetUserByIDFunc != nil {
		return m.GetUserByIDFunc(ctx, id)
	}
	for i := range m.Users {
		if m.Users[i].ID == id {
			return &m.Users[i], nil
		}
	}
	return nil, errors.New("user not found")
}

func (m *MockAdminRepository) UpdateUser(ctx context.Context, user *models.User) error {
	if m.UpdateUserFunc != nil {
		return m.UpdateUserFunc(ctx, user)
	}
	return nil
}

func (m *MockAdminRepository) ListAgents(ctx context.Context, opts *models.AgentListOptions) ([]models.Agent, int, error) {
	return m.Agents, len(m.Agents), nil
}

func (m *MockAdminRepository) GetAgentByID(ctx context.Context, id string) (*models.Agent, error) {
	if m.GetAgentByIDFunc != nil {
		return m.GetAgentByIDFunc(ctx, id)
	}
	for _, a := range m.Agents {
		if a.ID == id {
			return &a, nil
		}
	}
	return nil, errors.New("agent not found")
}

func (m *MockAdminRepository) UpdateAgent(ctx context.Context, agent *models.Agent) error {
	if m.UpdateAgentFunc != nil {
		return m.UpdateAgentFunc(ctx, agent)
	}
	return nil
}

func (m *MockAdminRepository) ListAuditLog(ctx context.Context, opts *models.AuditListOptions) ([]models.AuditLog, int, error) {
	return m.AuditEntries, len(m.AuditEntries), nil
}

func (m *MockAdminRepository) GetStats(ctx context.Context) (*models.AdminStats, error) {
	if m.StatsFunc != nil {
		return m.StatsFunc(ctx)
	}
	return &models.AdminStats{
		UsersCount:     100,
		AgentsCount:    50,
		PostsCount:     500,
		RateLimitHits:  10,
	}, nil
}

func (m *MockAdminRepository) HardDeletePost(ctx context.Context, id string) error {
	if m.HardDeletePostFunc != nil {
		return m.HardDeletePostFunc(ctx, id)
	}
	return nil
}

func (m *MockAdminRepository) RestorePost(ctx context.Context, id string) error {
	if m.RestorePostFunc != nil {
		return m.RestorePostFunc(ctx, id)
	}
	return nil
}

// Helper to add admin context to request
func addAdminContext(req *http.Request) *http.Request {
	claims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "admin@example.com",
		Role:   "admin",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	return req.WithContext(ctx)
}

// Helper to add non-admin context to request
func addUserContext(req *http.Request) *http.Request {
	claims := &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "user@example.com",
		Role:   "user",
	}
	ctx := auth.ContextWithClaims(req.Context(), claims)
	return req.WithContext(ctx)
}

// ==================== Flags Tests ====================

func TestListFlags_Success(t *testing.T) {
	flagID := uuid.New()
	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID,
				TargetType:   "post",
				TargetID:     uuid.New(),
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
				CreatedAt:    time.Now(),
			},
		},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/flags", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.ListFlags(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].([]interface{})
	if !ok {
		t.Fatal("expected data array in response")
	}

	if len(data) != 1 {
		t.Errorf("expected 1 flag, got %d", len(data))
	}
}

func TestListFlags_NoAuth(t *testing.T) {
	repo := &MockAdminRepository{}
	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/flags", nil)
	w := httptest.NewRecorder()

	handler.ListFlags(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestListFlags_NonAdmin(t *testing.T) {
	repo := &MockAdminRepository{}
	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/flags", nil)
	req = addUserContext(req)
	w := httptest.NewRecorder()

	handler.ListFlags(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestDismissFlag_Success(t *testing.T) {
	flagID := uuid.New()
	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID,
				TargetType:   "post",
				TargetID:     uuid.New(),
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
			},
		},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/flags/"+flagID.String()+"/dismiss", nil)
	req = addAdminContext(req)

	// Set chi URL param
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", flagID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.DismissFlag(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify audit log entry was created
	if len(repo.AuditEntries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(repo.AuditEntries))
	}

	if repo.AuditEntries[0].Action != "dismiss_flag" {
		t.Errorf("expected action 'dismiss_flag', got '%s'", repo.AuditEntries[0].Action)
	}
}

func TestDismissFlag_NotFound(t *testing.T) {
	repo := &MockAdminRepository{
		FlagByIDFunc: func(ctx context.Context, id string) (*models.Flag, error) {
			return nil, errors.New("flag not found")
		},
	}

	handler := NewAdminHandler(repo)

	flagID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/flags/"+flagID.String()+"/dismiss", nil)
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", flagID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.DismissFlag(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestActionOnFlag_Success(t *testing.T) {
	flagID := uuid.New()
	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{
				ID:           flagID,
				TargetType:   "post",
				TargetID:     uuid.New(),
				ReporterType: "human",
				ReporterID:   "user-123",
				Reason:       "spam",
				Status:       "pending",
			},
		},
	}

	handler := NewAdminHandler(repo)

	body := `{"action": "hide"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/flags/"+flagID.String()+"/action", strings.NewReader(body))
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", flagID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ActionOnFlag(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestActionOnFlag_InvalidAction(t *testing.T) {
	flagID := uuid.New()
	repo := &MockAdminRepository{
		Flags: []models.Flag{
			{ID: flagID, Status: "pending"},
		},
	}

	handler := NewAdminHandler(repo)

	body := `{"action": "invalid_action"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/flags/"+flagID.String()+"/action", strings.NewReader(body))
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", flagID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.ActionOnFlag(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

// ==================== Users Tests ====================

func TestListUsers_Success(t *testing.T) {
	repo := &MockAdminRepository{
		Users: []models.User{
			{
				ID:          uuid.New().String(),
				Username:    "testuser",
				DisplayName: "Test User",
				Email:       "test@example.com",
			},
		},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestListUsers_WithSearch(t *testing.T) {
	repo := &MockAdminRepository{
		Users: []models.User{},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users?q=test", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.ListUsers(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetUserDetail_Success(t *testing.T) {
	userID := uuid.New().String()
	repo := &MockAdminRepository{
		Users: []models.User{
			{
				ID:          userID,
				Username:    "testuser",
				DisplayName: "Test User",
				Email:       "test@example.com",
			},
		},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users/"+userID, nil)
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetUserDetail(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetUserDetail_NotFound(t *testing.T) {
	repo := &MockAdminRepository{
		GetUserByIDFunc: func(ctx context.Context, id string) (*models.User, error) {
			return nil, errors.New("user not found")
		},
	}

	handler := NewAdminHandler(repo)

	userID := uuid.New().String()
	req := httptest.NewRequest(http.MethodGet, "/v1/admin/users/"+userID, nil)
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.GetUserDetail(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestWarnUser_Success(t *testing.T) {
	userID := uuid.New().String()
	repo := &MockAdminRepository{
		Users: []models.User{{ID: userID}},
	}

	handler := NewAdminHandler(repo)

	body := `{"message": "Please follow the guidelines"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/users/"+userID+"/warn", strings.NewReader(body))
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.WarnUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	// Verify audit log entry
	if len(repo.AuditEntries) != 1 {
		t.Errorf("expected 1 audit entry, got %d", len(repo.AuditEntries))
	}
}

func TestSuspendUser_Success(t *testing.T) {
	userID := uuid.New().String()
	repo := &MockAdminRepository{
		Users: []models.User{{ID: userID, Status: "active"}},
	}

	handler := NewAdminHandler(repo)

	body := `{"duration": "7d", "reason": "Repeated violations"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/users/"+userID+"/suspend", strings.NewReader(body))
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.SuspendUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestBanUser_Success(t *testing.T) {
	userID := uuid.New().String()
	repo := &MockAdminRepository{
		Users: []models.User{{ID: userID, Status: "active"}},
	}

	handler := NewAdminHandler(repo)

	body := `{"reason": "Malicious activity"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/users/"+userID+"/ban", strings.NewReader(body))
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", userID)
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.BanUser(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ==================== Agents Tests ====================

func TestListAgents_Success(t *testing.T) {
	repo := &MockAdminRepository{
		Agents: []models.Agent{
			{ID: "agent_1", DisplayName: "Test Agent"},
		},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/agents", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.ListAgents(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestRevokeAgentKey_Success(t *testing.T) {
	repo := &MockAdminRepository{
		Agents: []models.Agent{{ID: "agent_1"}},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/agents/agent_1/revoke-key", nil)
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "agent_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.RevokeAgentKey(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestSuspendAgent_Success(t *testing.T) {
	repo := &MockAdminRepository{
		Agents: []models.Agent{{ID: "agent_1", Status: "active"}},
	}

	handler := NewAdminHandler(repo)

	body := `{"reason": "Suspicious activity"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/agents/agent_1/suspend", strings.NewReader(body))
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", "agent_1")
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.SuspendAgent(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

// ==================== Audit & Stats Tests ====================

func TestListAuditLog_Success(t *testing.T) {
	repo := &MockAdminRepository{
		AuditEntries: []models.AuditLog{
			{
				ID:        uuid.New(),
				AdminID:   uuid.New(),
				Action:    "ban_user",
				CreatedAt: time.Now(),
			},
		},
	}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/audit", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.ListAuditLog(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestListAuditLog_WithFilters(t *testing.T) {
	repo := &MockAdminRepository{}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/audit?action=ban_user&from_date=2024-01-01", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.ListAuditLog(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}

func TestGetStats_Success(t *testing.T) {
	repo := &MockAdminRepository{}

	handler := NewAdminHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/admin/stats", nil)
	req = addAdminContext(req)
	w := httptest.NewRecorder()

	handler.GetStats(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	data, ok := response["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}

	if data["users_count"] == nil {
		t.Error("expected users_count in stats")
	}
	if data["agents_count"] == nil {
		t.Error("expected agents_count in stats")
	}
	if data["posts_count"] == nil {
		t.Error("expected posts_count in stats")
	}
}

// ==================== Posts Admin Tests ====================

func TestHardDeletePost_Success(t *testing.T) {
	repo := &MockAdminRepository{}

	handler := NewAdminHandler(repo)

	postID := uuid.New()
	req := httptest.NewRequest(http.MethodDelete, "/v1/admin/posts/"+postID.String(), nil)
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", postID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.HardDeletePost(w, req)

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d", w.Code)
	}
}

func TestRestorePost_Success(t *testing.T) {
	repo := &MockAdminRepository{}

	handler := NewAdminHandler(repo)

	postID := uuid.New()
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/posts/"+postID.String()+"/restore", nil)
	req = addAdminContext(req)

	rctx := chi.NewRouteContext()
	rctx.URLParams.Add("id", postID.String())
	req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

	w := httptest.NewRecorder()
	handler.RestorePost(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
