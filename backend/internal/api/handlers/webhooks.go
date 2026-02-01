package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// Error types for webhook operations
var (
	ErrWebhookNotFound = errors.New("webhook not found")
)

// WebhookRepositoryInterface defines the database operations for webhooks.
type WebhookRepositoryInterface interface {
	Create(ctx context.Context, webhook *models.Webhook) error
	FindByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	List(ctx context.Context, agentID string) ([]models.Webhook, error)
	Update(ctx context.Context, webhook *models.Webhook) error
	Delete(ctx context.Context, id uuid.UUID) error
	FindAgent(ctx context.Context, agentID string) (*models.Agent, error)
}

// WebhooksHandler handles webhook-related HTTP requests.
type WebhooksHandler struct {
	repo WebhookRepositoryInterface
}

// NewWebhooksHandler creates a new WebhooksHandler.
func NewWebhooksHandler(repo WebhookRepositoryInterface) *WebhooksHandler {
	return &WebhooksHandler{
		repo: repo,
	}
}

// CreateWebhook handles POST /v1/agents/:id/webhooks - create a new webhook.
// Per SPEC.md Part 12.3.
func (h *WebhooksHandler) CreateWebhook(w http.ResponseWriter, r *http.Request, agentID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeWebhookUnauthorized(w, "authentication required")
		return
	}

	// Verify agent exists and caller is owner
	agent, err := h.repo.FindAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeWebhookError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Parse request body
	var req models.CreateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWebhookValidationError(w, "invalid JSON body")
		return
	}

	// Validate URL - must be HTTPS per SPEC.md Part 12.3
	if req.URL == "" {
		writeWebhookValidationError(w, "url is required")
		return
	}
	if !strings.HasPrefix(req.URL, "https://") {
		writeWebhookValidationError(w, "url must use HTTPS")
		return
	}

	// Validate events - must not be empty
	if len(req.Events) == 0 {
		writeWebhookValidationError(w, "events is required and must not be empty")
		return
	}

	// Validate each event type
	if invalid := models.ValidateWebhookEvents(req.Events); invalid != "" {
		writeWebhookError(w, http.StatusBadRequest, "INVALID_EVENT_TYPE", "invalid event type: "+invalid)
		return
	}

	// Validate secret - required
	if req.Secret == "" {
		writeWebhookValidationError(w, "secret is required")
		return
	}

	// Hash the secret for storage
	secretHash, err := bcrypt.GenerateFromPassword([]byte(req.Secret), bcrypt.DefaultCost)
	if err != nil {
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to hash secret")
		return
	}

	// Create webhook
	now := time.Now()
	webhook := &models.Webhook{
		AgentID:    agentID,
		URL:        req.URL,
		Events:     req.Events,
		SecretHash: string(secretHash),
		Status:     models.WebhookStatusActive,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if err := h.repo.Create(r.Context(), webhook); err != nil {
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to create webhook")
		return
	}

	// Return created webhook (without secret hash)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": webhook,
	})
}

// ListWebhooks handles GET /v1/agents/:id/webhooks - list all webhooks.
// Per SPEC.md Part 12.3.
func (h *WebhooksHandler) ListWebhooks(w http.ResponseWriter, r *http.Request, agentID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeWebhookUnauthorized(w, "authentication required")
		return
	}

	// Verify agent exists and caller is owner
	agent, err := h.repo.FindAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeWebhookError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Get webhooks
	webhooks, err := h.repo.List(r.Context(), agentID)
	if err != nil {
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to list webhooks")
		return
	}

	// Ensure empty array instead of null
	if webhooks == nil {
		webhooks = []models.Webhook{}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": webhooks,
	})
}

// GetWebhook handles GET /v1/agents/:id/webhooks/:wh_id - get single webhook.
// Per SPEC.md Part 12.3.
func (h *WebhooksHandler) GetWebhook(w http.ResponseWriter, r *http.Request, agentID, webhookID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeWebhookUnauthorized(w, "authentication required")
		return
	}

	// Verify agent exists and caller is owner
	agent, err := h.repo.FindAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeWebhookError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Parse webhook ID
	whID, err := uuid.Parse(webhookID)
	if err != nil {
		writeWebhookError(w, http.StatusBadRequest, "INVALID_ID", "invalid webhook ID format")
		return
	}

	// Get webhook
	webhook, err := h.repo.FindByID(r.Context(), whID)
	if err != nil {
		if errors.Is(err, ErrWebhookNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get webhook")
		return
	}

	// Verify webhook belongs to this agent
	if webhook.AgentID != agentID {
		writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": webhook,
	})
}

// UpdateWebhook handles PATCH /v1/agents/:id/webhooks/:wh_id - update webhook.
// Per SPEC.md Part 12.3.
func (h *WebhooksHandler) UpdateWebhook(w http.ResponseWriter, r *http.Request, agentID, webhookID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeWebhookUnauthorized(w, "authentication required")
		return
	}

	// Verify agent exists and caller is owner
	agent, err := h.repo.FindAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeWebhookError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Parse webhook ID
	whID, err := uuid.Parse(webhookID)
	if err != nil {
		writeWebhookError(w, http.StatusBadRequest, "INVALID_ID", "invalid webhook ID format")
		return
	}

	// Get existing webhook
	webhook, err := h.repo.FindByID(r.Context(), whID)
	if err != nil {
		if errors.Is(err, ErrWebhookNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get webhook")
		return
	}

	// Verify webhook belongs to this agent
	if webhook.AgentID != agentID {
		writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
		return
	}

	// Parse request body
	var req models.UpdateWebhookRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeWebhookValidationError(w, "invalid JSON body")
		return
	}

	// Update fields if provided
	if req.URL != nil {
		if !strings.HasPrefix(*req.URL, "https://") {
			writeWebhookValidationError(w, "url must use HTTPS")
			return
		}
		webhook.URL = *req.URL
	}

	if req.Events != nil {
		if len(req.Events) == 0 {
			writeWebhookValidationError(w, "events must not be empty")
			return
		}
		if invalid := models.ValidateWebhookEvents(req.Events); invalid != "" {
			writeWebhookError(w, http.StatusBadRequest, "INVALID_EVENT_TYPE", "invalid event type: "+invalid)
			return
		}
		webhook.Events = req.Events
	}

	if req.Secret != nil {
		secretHash, err := bcrypt.GenerateFromPassword([]byte(*req.Secret), bcrypt.DefaultCost)
		if err != nil {
			writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to hash secret")
			return
		}
		webhook.SecretHash = string(secretHash)
	}

	if req.Status != nil {
		if !models.IsValidWebhookStatus(*req.Status) {
			writeWebhookValidationError(w, "invalid status: must be active, paused, failing, or disabled")
			return
		}
		webhook.Status = models.WebhookStatus(*req.Status)
	}

	webhook.UpdatedAt = time.Now()

	if err := h.repo.Update(r.Context(), webhook); err != nil {
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to update webhook")
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"data": webhook,
	})
}

// DeleteWebhook handles DELETE /v1/agents/:id/webhooks/:wh_id - delete webhook.
// Per SPEC.md Part 12.3.
func (h *WebhooksHandler) DeleteWebhook(w http.ResponseWriter, r *http.Request, agentID, webhookID string) {
	// Require JWT authentication
	claims := auth.ClaimsFromContext(r.Context())
	if claims == nil {
		writeWebhookUnauthorized(w, "authentication required")
		return
	}

	// Verify agent exists and caller is owner
	agent, err := h.repo.FindAgent(r.Context(), agentID)
	if err != nil {
		if errors.Is(err, ErrAgentNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "agent not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get agent")
		return
	}

	// Verify ownership
	if agent.HumanID == nil || *agent.HumanID != claims.UserID {
		writeWebhookError(w, http.StatusForbidden, "FORBIDDEN", "you do not own this agent")
		return
	}

	// Parse webhook ID
	whID, err := uuid.Parse(webhookID)
	if err != nil {
		writeWebhookError(w, http.StatusBadRequest, "INVALID_ID", "invalid webhook ID format")
		return
	}

	// Get existing webhook
	webhook, err := h.repo.FindByID(r.Context(), whID)
	if err != nil {
		if errors.Is(err, ErrWebhookNotFound) {
			writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
			return
		}
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to get webhook")
		return
	}

	// Verify webhook belongs to this agent
	if webhook.AgentID != agentID {
		writeWebhookError(w, http.StatusNotFound, "NOT_FOUND", "webhook not found")
		return
	}

	// Delete webhook
	if err := h.repo.Delete(r.Context(), whID); err != nil {
		writeWebhookError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "failed to delete webhook")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// writeWebhookError writes an error response.
func writeWebhookError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error": map[string]interface{}{
			"code":    code,
			"message": message,
		},
	})
}

// writeWebhookUnauthorized writes a 401 Unauthorized error.
func writeWebhookUnauthorized(w http.ResponseWriter, message string) {
	writeWebhookError(w, http.StatusUnauthorized, "UNAUTHORIZED", message)
}

// writeWebhookValidationError writes a 400 Validation Error.
func writeWebhookValidationError(w http.ResponseWriter, message string) {
	writeWebhookError(w, http.StatusBadRequest, "VALIDATION_ERROR", message)
}
