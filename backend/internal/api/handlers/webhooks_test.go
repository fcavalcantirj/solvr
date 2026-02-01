package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// MockWebhookRepository implements WebhookRepositoryInterface for testing.
type MockWebhookRepository struct {
	CreateFunc    func(ctx context.Context, webhook *models.Webhook) error
	FindByIDFunc  func(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	ListFunc      func(ctx context.Context, agentID string) ([]models.Webhook, error)
	UpdateFunc    func(ctx context.Context, webhook *models.Webhook) error
	DeleteFunc    func(ctx context.Context, id uuid.UUID) error
	FindAgentFunc func(ctx context.Context, agentID string) (*models.Agent, error)
}

func (m *MockWebhookRepository) Create(ctx context.Context, webhook *models.Webhook) error {
	if m.CreateFunc != nil {
		return m.CreateFunc(ctx, webhook)
	}
	return nil
}

func (m *MockWebhookRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	if m.FindByIDFunc != nil {
		return m.FindByIDFunc(ctx, id)
	}
	return nil, ErrWebhookNotFound
}

func (m *MockWebhookRepository) List(ctx context.Context, agentID string) ([]models.Webhook, error) {
	if m.ListFunc != nil {
		return m.ListFunc(ctx, agentID)
	}
	return []models.Webhook{}, nil
}

func (m *MockWebhookRepository) Update(ctx context.Context, webhook *models.Webhook) error {
	if m.UpdateFunc != nil {
		return m.UpdateFunc(ctx, webhook)
	}
	return nil
}

func (m *MockWebhookRepository) Delete(ctx context.Context, id uuid.UUID) error {
	if m.DeleteFunc != nil {
		return m.DeleteFunc(ctx, id)
	}
	return nil
}

func (m *MockWebhookRepository) FindAgent(ctx context.Context, agentID string) (*models.Agent, error) {
	if m.FindAgentFunc != nil {
		return m.FindAgentFunc(ctx, agentID)
	}
	return nil, ErrAgentNotFound
}

// Helper to add auth context
func addWebhookAuthContext(r *http.Request, userID, role string) *http.Request {
	claims := &auth.Claims{
		UserID: userID,
		Role:   role,
	}
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// createTestWebhook creates a test webhook with defaults
func createTestWebhook(agentID string) *models.Webhook {
	now := time.Now()
	return &models.Webhook{
		ID:        uuid.New(),
		AgentID:   agentID,
		URL:       "https://example.com/webhook",
		Events:    []string{"answer.created", "problem.solved"},
		Status:    models.WebhookStatusActive,
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// createTestAgentForWebhook creates a test agent with owner
func createTestAgentForWebhook(id, ownerID string) *models.Agent {
	return &models.Agent{
		ID:          id,
		DisplayName: "Test Agent",
		HumanID:     &ownerID,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
}

// ====== CREATE WEBHOOK TESTS ======

func TestCreateWebhook_Success(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		CreateFunc: func(ctx context.Context, webhook *models.Webhook) error {
			webhook.ID = uuid.New()
			return nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, agentID)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].(map[string]interface{})
	if data["url"] != "https://example.com/webhook" {
		t.Errorf("expected url in response, got %v", data)
	}
}

func TestCreateWebhook_NoAuth(t *testing.T) {
	repo := &MockWebhookRepository{}
	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestCreateWebhook_NotOwner(t *testing.T) {
	userID := uuid.New().String()
	otherUserID := uuid.New().String()
	agentID := "test_agent"

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, otherUserID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, agentID)

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWebhook_AgentNotFound(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return nil, ErrAgentNotFound
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/nonexistent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "nonexistent")

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestCreateWebhook_InvalidJSON(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString("{invalid}"))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateWebhook_MissingURL(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateWebhook_InvalidURLNotHTTPS(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "http://example.com/webhook", "events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errResp := resp["error"].(map[string]interface{})
	if errResp["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected VALIDATION_ERROR, got %v", errResp)
	}
}

func TestCreateWebhook_EmptyEvents(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": [], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateWebhook_InvalidEventType(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["invalid.event"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errResp := resp["error"].(map[string]interface{})
	if errResp["code"] != "INVALID_EVENT_TYPE" {
		t.Errorf("expected INVALID_EVENT_TYPE, got %v", errResp)
	}
}

func TestCreateWebhook_MissingSecret(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created"]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestCreateWebhook_AllEventTypes(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		CreateFunc: func(ctx context.Context, webhook *models.Webhook) error {
			webhook.ID = uuid.New()
			return nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created", "comment.created", "approach.stuck", "problem.solved", "mention"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCreateWebhook_DatabaseError(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		CreateFunc: func(ctx context.Context, webhook *models.Webhook) error {
			return errors.New("database error")
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://example.com/webhook", "events": ["answer.created"], "secret": "mysecret"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/test_agent/webhooks", bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.CreateWebhook(w, req, "test_agent")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d", w.Code)
	}
}

// ====== LIST WEBHOOKS TESTS ======

func TestListWebhooks_Success(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"

	webhook1 := createTestWebhook(agentID)
	webhook2 := createTestWebhook(agentID)
	webhook2.ID = uuid.New()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		ListFunc: func(ctx context.Context, id string) ([]models.Webhook, error) {
			return []models.Webhook{*webhook1, *webhook2}, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks", nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.ListWebhooks(w, req, agentID)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].([]interface{})
	if len(data) != 2 {
		t.Errorf("expected 2 webhooks, got %d", len(data))
	}
}

func TestListWebhooks_NoAuth(t *testing.T) {
	repo := &MockWebhookRepository{}
	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks", nil)
	w := httptest.NewRecorder()

	handler.ListWebhooks(w, req, "test_agent")

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestListWebhooks_NotOwner(t *testing.T) {
	userID := uuid.New().String()
	otherUserID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, otherUserID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks", nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.ListWebhooks(w, req, "test_agent")

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestListWebhooks_Empty(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		ListFunc: func(ctx context.Context, id string) ([]models.Webhook, error) {
			return []models.Webhook{}, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks", nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.ListWebhooks(w, req, "test_agent")

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	data := resp["data"].([]interface{})
	if len(data) != 0 {
		t.Errorf("expected 0 webhooks, got %d", len(data))
	}
}

// ====== VALIDATION TESTS ======

func TestIsValidWebhookEventType(t *testing.T) {
	tests := []struct {
		event string
		valid bool
	}{
		{"answer.created", true},
		{"comment.created", true},
		{"approach.stuck", true},
		{"problem.solved", true},
		{"mention", true},
		{"invalid.event", false},
		{"", false},
		{"ANSWER.CREATED", false}, // case-sensitive
	}

	for _, tt := range tests {
		t.Run(tt.event, func(t *testing.T) {
			result := models.IsValidWebhookEventType(tt.event)
			if result != tt.valid {
				t.Errorf("IsValidWebhookEventType(%q) = %v, want %v", tt.event, result, tt.valid)
			}
		})
	}
}

func TestIsValidWebhookStatus(t *testing.T) {
	tests := []struct {
		status string
		valid  bool
	}{
		{"active", true},
		{"paused", true},
		{"failing", true},
		{"disabled", true},
		{"invalid", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			result := models.IsValidWebhookStatus(tt.status)
			if result != tt.valid {
				t.Errorf("IsValidWebhookStatus(%q) = %v, want %v", tt.status, result, tt.valid)
			}
		})
	}
}
