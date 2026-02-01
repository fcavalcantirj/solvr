// Package services provides business logic for the Solvr application.
package services

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// ErrWebhookNotFound is returned when a webhook is not found.
var ErrWebhookNotFound = errors.New("webhook not found")

// ErrWebhookDeliveryFailed is returned when webhook delivery fails.
var ErrWebhookDeliveryFailed = errors.New("webhook delivery failed")

// WebhookRepository defines the database operations for webhooks.
type WebhookRepository interface {
	FindByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error)
	FindByAgentAndEvent(ctx context.Context, agentID string, event string) ([]*models.Webhook, error)
	UpdateDeliveryStatus(ctx context.Context, id uuid.UUID, failures int, lastFailure, lastSuccess *time.Time, status *models.WebhookStatus) error
}

// WebhookDeliveryService handles webhook delivery.
type WebhookDeliveryService struct {
	repo   WebhookRepository
	client *http.Client
}

// NewWebhookDeliveryService creates a new webhook delivery service.
func NewWebhookDeliveryService(repo WebhookRepository, client *http.Client) *WebhookDeliveryService {
	if client == nil {
		client = &http.Client{Timeout: 10 * time.Second}
	}
	return &WebhookDeliveryService{
		repo:   repo,
		client: client,
	}
}

// DeliverWebhook delivers a webhook event to the specified webhook.
// This is equivalent to DeliverWebhookWithAttempt with attempt=1.
func (s *WebhookDeliveryService) DeliverWebhook(ctx context.Context, webhookID uuid.UUID, event string, data map[string]interface{}, secret string) error {
	return s.DeliverWebhookWithAttempt(ctx, webhookID, event, data, secret, 1)
}

// DeliverWebhookWithAttempt delivers a webhook event with a specific attempt number.
// Per SPEC.md Part 12.3:
// - Payload: {event, timestamp, data, signature}
// - Headers: X-Solvr-Delivery-Attempt, X-Solvr-Webhook-ID, Content-Type, X-Solvr-Signature
// - Success: HTTP 2xx within 10 seconds
// - After 5 failures: status='failing'
func (s *WebhookDeliveryService) DeliverWebhookWithAttempt(ctx context.Context, webhookID uuid.UUID, event string, data map[string]interface{}, secret string, attempt int) error {
	// Find webhook
	webhook, err := s.repo.FindByID(ctx, webhookID)
	if err != nil {
		return ErrWebhookNotFound
	}

	// Build payload
	payload, err := s.BuildPayload(event, data)
	if err != nil {
		return fmt.Errorf("failed to build payload: %w", err)
	}

	// Generate signature
	signature := s.GenerateSignature(payload, secret)

	// Create request
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, webhook.URL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers per SPEC.md Part 12.3
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Solvr-Signature", signature)
	req.Header.Set("X-Solvr-Webhook-ID", webhookID.String())
	req.Header.Set("X-Solvr-Delivery-Attempt", strconv.Itoa(attempt))

	// Send request
	resp, err := s.client.Do(req)
	if err != nil {
		// Network error - record failure
		return s.recordFailure(ctx, webhook, err)
	}
	defer resp.Body.Close()

	// Check response status - success is 2xx
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return s.recordFailure(ctx, webhook, fmt.Errorf("webhook returned %d", resp.StatusCode))
	}

	// Success - reset failures
	return s.recordSuccess(ctx, webhook)
}

// recordFailure updates the webhook after a failed delivery.
func (s *WebhookDeliveryService) recordFailure(ctx context.Context, webhook *models.Webhook, deliveryErr error) error {
	now := time.Now()
	newFailures := webhook.ConsecutiveFailures + 1

	var newStatus *models.WebhookStatus
	// Per SPEC.md Part 12.3: After 5 failures, mark as failing
	if newFailures >= 5 {
		status := models.WebhookStatusFailing
		newStatus = &status
	}

	err := s.repo.UpdateDeliveryStatus(ctx, webhook.ID, newFailures, &now, nil, newStatus)
	if err != nil {
		return fmt.Errorf("failed to update webhook status: %w", err)
	}

	return fmt.Errorf("%w: %v", ErrWebhookDeliveryFailed, deliveryErr)
}

// recordSuccess updates the webhook after a successful delivery.
func (s *WebhookDeliveryService) recordSuccess(ctx context.Context, webhook *models.Webhook) error {
	now := time.Now()

	// Reset failures to 0 and set status back to active
	status := models.WebhookStatusActive
	err := s.repo.UpdateDeliveryStatus(ctx, webhook.ID, 0, nil, &now, &status)
	if err != nil {
		return fmt.Errorf("failed to update webhook status: %w", err)
	}

	return nil
}

// BuildPayload creates the webhook payload JSON.
// Per SPEC.md Part 12.3:
//
//	{
//	  "event": "answer.created",
//	  "timestamp": "2026-01-31T19:00:00Z",
//	  "data": { ... }
//	}
func (s *WebhookDeliveryService) BuildPayload(event string, data map[string]interface{}) ([]byte, error) {
	if data == nil {
		data = make(map[string]interface{})
	}

	payload := models.WebhookPayload{
		Event:     event,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Data:      data,
	}

	return json.Marshal(payload)
}

// GenerateSignature generates the HMAC-SHA256 signature for a payload.
// Per SPEC.md Part 12.3: "sha256=..." format
func (s *WebhookDeliveryService) GenerateSignature(payload []byte, secret string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// GetRetryDelays returns the retry delay schedule.
// Per SPEC.md Part 12.3:
// - Attempt 1: Immediate
// - Attempt 2: 1 minute
// - Attempt 3: 5 minutes
// - Attempt 4: 30 minutes
// - Attempt 5: 2 hours
func (s *WebhookDeliveryService) GetRetryDelays() []time.Duration {
	return []time.Duration{
		0,                // Attempt 1: Immediate
		1 * time.Minute, // Attempt 2: 1 minute
		5 * time.Minute, // Attempt 3: 5 minutes
		30 * time.Minute, // Attempt 4: 30 minutes
		2 * time.Hour,   // Attempt 5: 2 hours
	}
}

// ShouldRetry determines if a failed delivery should be retried.
func (s *WebhookDeliveryService) ShouldRetry(attempt int) bool {
	return attempt < 5
}

// GetNextRetryDelay returns the delay before the next retry attempt.
func (s *WebhookDeliveryService) GetNextRetryDelay(attempt int) time.Duration {
	delays := s.GetRetryDelays()
	if attempt < 0 || attempt >= len(delays) {
		return 0
	}
	return delays[attempt]
}
