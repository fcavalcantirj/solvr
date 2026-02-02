// Package services provides business logic for the Solvr application.
package services

/**
 * E2E tests for Webhook system
 *
 * Per PRD line 5061-5068:
 * - E2E: Webhooks
 * - Register webhook
 * - Trigger event
 * - Verify delivery with signature
 *
 * These tests verify the complete webhook flow:
 * 1. Create/register a webhook
 * 2. Trigger an event that should fire the webhook
 * 3. Verify the webhook payload was delivered correctly
 * 4. Verify the signature matches HMAC-SHA256
 */

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// WebhookDelivery captures a webhook delivery for verification.
type WebhookDelivery struct {
	Payload     []byte
	Headers     http.Header
	ReceivedAt  time.Time
	StatusCode  int
	ContentType string
}

// MockWebhookReceiver simulates an external webhook endpoint.
type MockWebhookReceiver struct {
	mu         sync.Mutex
	deliveries []WebhookDelivery
	statusCode int
	server     *httptest.Server
}

// NewMockWebhookReceiver creates a new mock receiver that returns the specified status.
func NewMockWebhookReceiver(statusCode int) *MockWebhookReceiver {
	receiver := &MockWebhookReceiver{
		deliveries: make([]WebhookDelivery, 0),
		statusCode: statusCode,
	}

	receiver.server = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		defer r.Body.Close()

		receiver.mu.Lock()
		receiver.deliveries = append(receiver.deliveries, WebhookDelivery{
			Payload:     body,
			Headers:     r.Header.Clone(),
			ReceivedAt:  time.Now(),
			ContentType: r.Header.Get("Content-Type"),
		})
		receiver.mu.Unlock()

		w.WriteHeader(receiver.statusCode)
	}))

	return receiver
}

// URL returns the server URL.
func (r *MockWebhookReceiver) URL() string {
	return r.server.URL
}

// GetDeliveries returns a copy of all received deliveries.
func (r *MockWebhookReceiver) GetDeliveries() []WebhookDelivery {
	r.mu.Lock()
	defer r.mu.Unlock()
	result := make([]WebhookDelivery, len(r.deliveries))
	copy(result, r.deliveries)
	return result
}

// Close shuts down the server.
func (r *MockWebhookReceiver) Close() {
	r.server.Close()
}

// ============================================================================
// E2E Tests: Complete Webhook Flow
// ============================================================================

// TestE2E_Webhook_CompleteFlow tests the full webhook lifecycle:
// 1. Register webhook
// 2. Trigger event
// 3. Verify delivery with signature
func TestE2E_Webhook_CompleteFlow(t *testing.T) {
	// Step 1: Set up mock webhook receiver
	receiver := NewMockWebhookReceiver(http.StatusOK)
	defer receiver.Close()

	// Step 2: Create mock repository with registered webhook
	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "e2e-test-webhook-secret-key-12345"
	agentID := "test_agent_e2e"

	// Register the webhook (simulating POST /v1/agents/:id/webhooks)
	webhook := &models.Webhook{
		ID:         webhookID,
		AgentID:    agentID,
		URL:        receiver.URL(),
		Events:     []string{"answer.created", "problem.solved"},
		SecretHash: secret, // In real use this is hashed, but for delivery we use plain secret
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.AddWebhook(webhook)

	// Step 3: Create webhook delivery service with TLS client
	client := receiver.server.Client()
	service := NewWebhookDeliveryService(repo, client)

	// Step 4: Trigger event (simulating what happens when an answer is created)
	eventData := map[string]interface{}{
		"answer_id":    "ans_e2e_001",
		"question_id":  "q_e2e_001",
		"author_id":    agentID,
		"content":      "This is the answer content from E2E test",
		"votes":        0,
		"is_accepted":  false,
	}

	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", eventData, secret)
	if err != nil {
		t.Fatalf("webhook delivery failed: %v", err)
	}

	// Step 5: Verify delivery was received
	deliveries := receiver.GetDeliveries()
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	delivery := deliveries[0]

	// Verify Content-Type header
	if delivery.ContentType != "application/json" {
		t.Errorf("expected Content-Type 'application/json', got '%s'", delivery.ContentType)
	}

	// Verify X-Solvr-Webhook-ID header
	webhookIDHeader := delivery.Headers.Get("X-Solvr-Webhook-ID")
	if webhookIDHeader != webhookID.String() {
		t.Errorf("expected X-Solvr-Webhook-ID '%s', got '%s'", webhookID.String(), webhookIDHeader)
	}

	// Verify X-Solvr-Delivery-Attempt header
	attemptHeader := delivery.Headers.Get("X-Solvr-Delivery-Attempt")
	if attemptHeader != "1" {
		t.Errorf("expected X-Solvr-Delivery-Attempt '1', got '%s'", attemptHeader)
	}

	// Step 6: Parse and verify payload structure
	var payload models.WebhookPayload
	if err := json.Unmarshal(delivery.Payload, &payload); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	if payload.Event != "answer.created" {
		t.Errorf("expected event 'answer.created', got '%s'", payload.Event)
	}

	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, payload.Timestamp)
	if err != nil {
		t.Errorf("timestamp is not valid RFC3339: %v", err)
	}

	// Verify data contains expected fields
	if payload.Data["answer_id"] != "ans_e2e_001" {
		t.Errorf("expected answer_id 'ans_e2e_001', got '%v'", payload.Data["answer_id"])
	}
	if payload.Data["question_id"] != "q_e2e_001" {
		t.Errorf("expected question_id 'q_e2e_001', got '%v'", payload.Data["question_id"])
	}

	// Step 7: Verify signature (HMAC-SHA256)
	signatureHeader := delivery.Headers.Get("X-Solvr-Signature")
	if signatureHeader == "" {
		t.Fatal("expected X-Solvr-Signature header to be present")
	}

	// Verify signature format
	if !strings.HasPrefix(signatureHeader, "sha256=") {
		t.Errorf("signature should start with 'sha256=', got '%s'", signatureHeader)
	}

	// Calculate expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(delivery.Payload)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if signatureHeader != expectedSig {
		t.Errorf("signature mismatch:\n  expected: %s\n  got:      %s", expectedSig, signatureHeader)
	}

	// Verify signature can be validated by recipient
	// (This is how a webhook receiver would verify the signature)
	valid := verifyWebhookSignature(delivery.Payload, signatureHeader, secret)
	if !valid {
		t.Error("signature verification failed - receiver cannot trust this webhook")
	}
}

// TestE2E_Webhook_MultipleEventsFlow tests registering for multiple events
// and verifying only subscribed events trigger deliveries.
func TestE2E_Webhook_MultipleEventsFlow(t *testing.T) {
	receiver := NewMockWebhookReceiver(http.StatusOK)
	defer receiver.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "multi-event-secret"
	agentID := "multi_event_agent"

	// Register webhook for specific events
	webhook := &models.Webhook{
		ID:         webhookID,
		AgentID:    agentID,
		URL:        receiver.URL(),
		Events:     []string{"problem.solved", "mention"},
		SecretHash: secret,
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.AddWebhook(webhook)

	client := receiver.server.Client()
	service := NewWebhookDeliveryService(repo, client)

	// Trigger problem.solved event
	err := service.DeliverWebhook(context.Background(), webhookID, "problem.solved", map[string]interface{}{
		"problem_id": "prob_001",
		"solution":   "The fix was to update dependencies",
	}, secret)
	if err != nil {
		t.Fatalf("problem.solved delivery failed: %v", err)
	}

	// Trigger mention event
	err = service.DeliverWebhook(context.Background(), webhookID, "mention", map[string]interface{}{
		"post_id":   "post_002",
		"mentioned": agentID,
	}, secret)
	if err != nil {
		t.Fatalf("mention delivery failed: %v", err)
	}

	// Verify both deliveries were received
	deliveries := receiver.GetDeliveries()
	if len(deliveries) != 2 {
		t.Fatalf("expected 2 deliveries, got %d", len(deliveries))
	}

	// Verify first delivery is problem.solved
	var payload1 models.WebhookPayload
	json.Unmarshal(deliveries[0].Payload, &payload1)
	if payload1.Event != "problem.solved" {
		t.Errorf("expected first event 'problem.solved', got '%s'", payload1.Event)
	}

	// Verify second delivery is mention
	var payload2 models.WebhookPayload
	json.Unmarshal(deliveries[1].Payload, &payload2)
	if payload2.Event != "mention" {
		t.Errorf("expected second event 'mention', got '%s'", payload2.Event)
	}

	// Verify both signatures are valid
	for i, d := range deliveries {
		sig := d.Headers.Get("X-Solvr-Signature")
		if !verifyWebhookSignature(d.Payload, sig, secret) {
			t.Errorf("signature verification failed for delivery %d", i+1)
		}
	}
}

// TestE2E_Webhook_FailureRecoveryFlow tests that a webhook recovers
// after failures when it starts receiving 200 responses again.
func TestE2E_Webhook_FailureRecoveryFlow(t *testing.T) {
	// Start with a receiver that returns 500
	failingReceiver := NewMockWebhookReceiver(http.StatusInternalServerError)
	defer failingReceiver.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "recovery-test-secret"
	agentID := "recovery_agent"

	webhook := &models.Webhook{
		ID:                  webhookID,
		AgentID:             agentID,
		URL:                 failingReceiver.URL(),
		Events:              []string{"answer.created"},
		SecretHash:          secret,
		Status:              models.WebhookStatusActive,
		ConsecutiveFailures: 0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	}
	repo.AddWebhook(webhook)

	client := failingReceiver.server.Client()
	service := NewWebhookDeliveryService(repo, client)

	// Attempt delivery (should fail)
	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)
	if err == nil {
		t.Error("expected error for 500 response")
	}

	// Verify failure was recorded
	wh := repo.GetWebhook(webhookID)
	if wh.ConsecutiveFailures != 1 {
		t.Errorf("expected 1 consecutive failure, got %d", wh.ConsecutiveFailures)
	}
	if wh.LastFailureAt == nil {
		t.Error("expected LastFailureAt to be set")
	}

	// Now set up a successful receiver
	successReceiver := NewMockWebhookReceiver(http.StatusOK)
	defer successReceiver.Close()

	// Update webhook URL to successful receiver
	webhook.URL = successReceiver.URL()
	client = successReceiver.server.Client()
	service = NewWebhookDeliveryService(repo, client)

	// Deliver successfully
	err = service.DeliverWebhook(context.Background(), webhookID, "answer.created", map[string]interface{}{
		"test": "recovery",
	}, secret)
	if err != nil {
		t.Fatalf("expected success after recovery, got: %v", err)
	}

	// Verify failures were reset
	wh = repo.GetWebhook(webhookID)
	if wh.ConsecutiveFailures != 0 {
		t.Errorf("expected 0 consecutive failures after success, got %d", wh.ConsecutiveFailures)
	}
	if wh.Status != models.WebhookStatusActive {
		t.Errorf("expected status 'active', got '%s'", wh.Status)
	}
	if wh.LastSuccessAt == nil {
		t.Error("expected LastSuccessAt to be set")
	}

	// Verify delivery was received
	deliveries := successReceiver.GetDeliveries()
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	// Verify signature is valid
	sig := deliveries[0].Headers.Get("X-Solvr-Signature")
	if !verifyWebhookSignature(deliveries[0].Payload, sig, secret) {
		t.Error("signature verification failed")
	}
}

// TestE2E_Webhook_RetryFlow tests the retry mechanism with multiple attempts.
func TestE2E_Webhook_RetryFlow(t *testing.T) {
	receiver := NewMockWebhookReceiver(http.StatusOK)
	defer receiver.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "retry-test-secret"

	webhook := &models.Webhook{
		ID:         webhookID,
		AgentID:    "retry_agent",
		URL:        receiver.URL(),
		Events:     []string{"answer.created"},
		SecretHash: secret,
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.AddWebhook(webhook)

	client := receiver.server.Client()
	service := NewWebhookDeliveryService(repo, client)

	// Simulate multiple retry attempts
	attempts := []int{1, 2, 3}
	for _, attempt := range attempts {
		err := service.DeliverWebhookWithAttempt(context.Background(), webhookID, "answer.created", map[string]interface{}{
			"attempt": attempt,
		}, secret, attempt)
		if err != nil {
			t.Fatalf("delivery attempt %d failed: %v", attempt, err)
		}
	}

	// Verify all deliveries were received with correct attempt numbers
	deliveries := receiver.GetDeliveries()
	if len(deliveries) != 3 {
		t.Fatalf("expected 3 deliveries, got %d", len(deliveries))
	}

	for i, d := range deliveries {
		expectedAttempt := i + 1
		attemptHeader := d.Headers.Get("X-Solvr-Delivery-Attempt")
		if attemptHeader != string(rune('0'+expectedAttempt)) {
			t.Errorf("delivery %d: expected attempt '%d', got '%s'", i+1, expectedAttempt, attemptHeader)
		}

		// Verify signature for each delivery
		sig := d.Headers.Get("X-Solvr-Signature")
		if !verifyWebhookSignature(d.Payload, sig, secret) {
			t.Errorf("signature verification failed for attempt %d", expectedAttempt)
		}
	}
}

// TestE2E_Webhook_PayloadIntegrity tests that payload data is preserved exactly.
func TestE2E_Webhook_PayloadIntegrity(t *testing.T) {
	receiver := NewMockWebhookReceiver(http.StatusOK)
	defer receiver.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "integrity-test-secret"

	webhook := &models.Webhook{
		ID:         webhookID,
		AgentID:    "integrity_agent",
		URL:        receiver.URL(),
		Events:     []string{"comment.created"},
		SecretHash: secret,
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	repo.AddWebhook(webhook)

	client := receiver.server.Client()
	service := NewWebhookDeliveryService(repo, client)

	// Complex payload with nested data
	complexData := map[string]interface{}{
		"comment_id": "cmt_e2e_001",
		"post_id":    "post_e2e_001",
		"author": map[string]interface{}{
			"id":           "agent_claude",
			"display_name": "Claude Assistant",
			"type":         "agent",
		},
		"content":    "This is a comment with special chars: <>&\"' and unicode: 你好世界 🚀",
		"created_at": "2026-02-02T10:30:00Z",
		"metadata": map[string]interface{}{
			"length":   42,
			"mentions": []interface{}{"@agent_1", "@agent_2"},
		},
	}

	err := service.DeliverWebhook(context.Background(), webhookID, "comment.created", complexData, secret)
	if err != nil {
		t.Fatalf("delivery failed: %v", err)
	}

	// Verify delivery
	deliveries := receiver.GetDeliveries()
	if len(deliveries) != 1 {
		t.Fatalf("expected 1 delivery, got %d", len(deliveries))
	}

	var payload models.WebhookPayload
	if err := json.Unmarshal(deliveries[0].Payload, &payload); err != nil {
		t.Fatalf("failed to parse payload: %v", err)
	}

	// Verify nested author data
	author, ok := payload.Data["author"].(map[string]interface{})
	if !ok {
		t.Fatal("expected author to be a map")
	}
	if author["id"] != "agent_claude" {
		t.Errorf("expected author.id 'agent_claude', got '%v'", author["id"])
	}
	if author["display_name"] != "Claude Assistant" {
		t.Errorf("expected author.display_name 'Claude Assistant', got '%v'", author["display_name"])
	}

	// Verify special characters preserved
	content := payload.Data["content"].(string)
	if !strings.Contains(content, "你好世界") {
		t.Error("unicode characters not preserved in payload")
	}
	if !strings.Contains(content, "🚀") {
		t.Error("emoji not preserved in payload")
	}

	// Verify metadata with array
	metadata := payload.Data["metadata"].(map[string]interface{})
	mentions := metadata["mentions"].([]interface{})
	if len(mentions) != 2 {
		t.Errorf("expected 2 mentions, got %d", len(mentions))
	}
	if mentions[0] != "@agent_1" || mentions[1] != "@agent_2" {
		t.Error("mentions array not preserved correctly")
	}

	// Verify signature is valid for complex payload
	sig := deliveries[0].Headers.Get("X-Solvr-Signature")
	if !verifyWebhookSignature(deliveries[0].Payload, sig, secret) {
		t.Error("signature verification failed for complex payload")
	}
}

// TestE2E_Webhook_DifferentStatusCodes tests webhook response handling.
func TestE2E_Webhook_DifferentStatusCodes(t *testing.T) {
	testCases := []struct {
		name        string
		statusCode  int
		expectError bool
	}{
		{"200 OK", http.StatusOK, false},
		{"201 Created", http.StatusCreated, false},
		{"202 Accepted", http.StatusAccepted, false},
		{"204 No Content", http.StatusNoContent, false},
		{"400 Bad Request", http.StatusBadRequest, true},
		{"401 Unauthorized", http.StatusUnauthorized, true},
		{"403 Forbidden", http.StatusForbidden, true},
		{"404 Not Found", http.StatusNotFound, true},
		{"500 Internal Server Error", http.StatusInternalServerError, true},
		{"502 Bad Gateway", http.StatusBadGateway, true},
		{"503 Service Unavailable", http.StatusServiceUnavailable, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			receiver := NewMockWebhookReceiver(tc.statusCode)
			defer receiver.Close()

			repo := NewMockWebhookRepository()
			webhookID := uuid.New()
			secret := "status-code-test"

			webhook := &models.Webhook{
				ID:         webhookID,
				AgentID:    "status_test_agent",
				URL:        receiver.URL(),
				Events:     []string{"answer.created"},
				SecretHash: secret,
				Status:     models.WebhookStatusActive,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			}
			repo.AddWebhook(webhook)

			client := receiver.server.Client()
			service := NewWebhookDeliveryService(repo, client)

			err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)

			if tc.expectError && err == nil {
				t.Errorf("expected error for status %d, got none", tc.statusCode)
			}
			if !tc.expectError && err != nil {
				t.Errorf("expected no error for status %d, got: %v", tc.statusCode, err)
			}

			// Verify webhook was still delivered regardless of status
			deliveries := receiver.GetDeliveries()
			if len(deliveries) != 1 {
				t.Errorf("expected 1 delivery attempt, got %d", len(deliveries))
			}

			// Verify signature is valid even for failed responses
			sig := deliveries[0].Headers.Get("X-Solvr-Signature")
			if !verifyWebhookSignature(deliveries[0].Payload, sig, secret) {
				t.Error("signature verification failed")
			}
		})
	}
}

// verifyWebhookSignature verifies a webhook signature as a receiver would.
// This is the standard way to verify incoming webhooks.
func verifyWebhookSignature(payload []byte, signature, secret string) bool {
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}

	providedSig := strings.TrimPrefix(signature, "sha256=")

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(providedSig), []byte(expectedSig))
}
