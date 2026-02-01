package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// ====== GET SINGLE WEBHOOK TESTS ======

func TestGetWebhook_Success(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.GetWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetWebhook_NotFound(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return nil, ErrWebhookNotFound
		},
	}

	handler := NewWebhooksHandler(repo)
	webhookID := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks/"+webhookID, nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.GetWebhook(w, req, "test_agent", webhookID)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestGetWebhook_InvalidUUID(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks/invalid-uuid", nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.GetWebhook(w, req, "test_agent", "invalid-uuid")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestGetWebhook_NoAuth(t *testing.T) {
	repo := &MockWebhookRepository{}
	handler := NewWebhooksHandler(repo)
	webhookID := uuid.New().String()

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks/"+webhookID, nil)
	w := httptest.NewRecorder()

	handler.GetWebhook(w, req, "test_agent", webhookID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestGetWebhook_NotOwner(t *testing.T) {
	userID := uuid.New().String()
	otherUserID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, otherUserID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodGet, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.GetWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

// ====== UPDATE WEBHOOK TESTS ======

func TestUpdateWebhook_Success(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
		UpdateFunc: func(ctx context.Context, w *models.Webhook) error {
			return nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://newurl.com/webhook"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateWebhook_NotOwner(t *testing.T) {
	userID := uuid.New().String()
	otherUserID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, otherUserID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://newurl.com/webhook"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestUpdateWebhook_ChangeStatus(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
		UpdateFunc: func(ctx context.Context, w *models.Webhook) error {
			return nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"status": "paused"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}
}

func TestUpdateWebhook_InvalidStatus(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"status": "invalid_status"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}

func TestUpdateWebhook_NoAuth(t *testing.T) {
	repo := &MockWebhookRepository{}
	handler := NewWebhooksHandler(repo)
	webhookID := uuid.New().String()

	body := `{"url": "https://newurl.com/webhook"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhookID, bytes.NewBufferString(body))
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, "test_agent", webhookID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestUpdateWebhook_NotFound(t *testing.T) {
	userID := uuid.New().String()
	webhookID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return nil, ErrWebhookNotFound
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"url": "https://newurl.com/webhook"}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhookID, bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, "test_agent", webhookID)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestUpdateWebhook_ChangeEvents(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	var updatedEvents []string
	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
		UpdateFunc: func(ctx context.Context, w *models.Webhook) error {
			updatedEvents = w.Events
			return nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"events": ["mention", "comment.created"]}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
	}

	if len(updatedEvents) != 2 {
		t.Errorf("expected 2 events, got %d", len(updatedEvents))
	}
}

func TestUpdateWebhook_InvalidEvents(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	body := `{"events": ["invalid.event"]}`
	req := httptest.NewRequest(http.MethodPatch, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), bytes.NewBufferString(body))
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.UpdateWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.NewDecoder(w.Body).Decode(&resp)
	errResp := resp["error"].(map[string]interface{})
	if errResp["code"] != "INVALID_EVENT_TYPE" {
		t.Errorf("expected INVALID_EVENT_TYPE, got %v", errResp)
	}
}

// ====== DELETE WEBHOOK TESTS ======

func TestDeleteWebhook_Success(t *testing.T) {
	userID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	deleted := false
	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
		DeleteFunc: func(ctx context.Context, id uuid.UUID) error {
			deleted = true
			return nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.DeleteWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusNoContent {
		t.Errorf("expected status 204, got %d: %s", w.Code, w.Body.String())
	}

	if !deleted {
		t.Error("expected delete to be called")
	}
}

func TestDeleteWebhook_NotOwner(t *testing.T) {
	userID := uuid.New().String()
	otherUserID := uuid.New().String()
	agentID := "test_agent"
	webhook := createTestWebhook(agentID)

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, otherUserID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return webhook, nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/test_agent/webhooks/"+webhook.ID.String(), nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.DeleteWebhook(w, req, agentID, webhook.ID.String())

	if w.Code != http.StatusForbidden {
		t.Errorf("expected status 403, got %d", w.Code)
	}
}

func TestDeleteWebhook_NotFound(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
		FindByIDFunc: func(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
			return nil, ErrWebhookNotFound
		},
	}

	handler := NewWebhooksHandler(repo)
	webhookID := uuid.New().String()

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/test_agent/webhooks/"+webhookID, nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.DeleteWebhook(w, req, "test_agent", webhookID)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestDeleteWebhook_NoAuth(t *testing.T) {
	repo := &MockWebhookRepository{}
	handler := NewWebhooksHandler(repo)
	webhookID := uuid.New().String()

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/test_agent/webhooks/"+webhookID, nil)
	w := httptest.NewRecorder()

	handler.DeleteWebhook(w, req, "test_agent", webhookID)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}
}

func TestDeleteWebhook_InvalidUUID(t *testing.T) {
	userID := uuid.New().String()

	repo := &MockWebhookRepository{
		FindAgentFunc: func(ctx context.Context, id string) (*models.Agent, error) {
			return createTestAgentForWebhook(id, userID), nil
		},
	}

	handler := NewWebhooksHandler(repo)

	req := httptest.NewRequest(http.MethodDelete, "/v1/agents/test_agent/webhooks/invalid-uuid", nil)
	req = addWebhookAuthContext(req, userID, "user")
	w := httptest.NewRecorder()

	handler.DeleteWebhook(w, req, "test_agent", "invalid-uuid")

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}
}
