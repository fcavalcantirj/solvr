// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// MockWebhookRepository implements WebhookRepository for testing.
type MockWebhookRepository struct {
	mu       sync.Mutex
	webhooks map[uuid.UUID]*models.Webhook

	// For tracking calls
	UpdateCalls []UpdateWebhookCall
}

// UpdateWebhookCall tracks calls to Update.
type UpdateWebhookCall struct {
	WebhookID           uuid.UUID
	ConsecutiveFailures *int
	LastFailureAt       *time.Time
	LastSuccessAt       *time.Time
	Status              *models.WebhookStatus
}

// NewMockWebhookRepository creates a new mock repository.
func NewMockWebhookRepository() *MockWebhookRepository {
	return &MockWebhookRepository{
		webhooks:    make(map[uuid.UUID]*models.Webhook),
		UpdateCalls: make([]UpdateWebhookCall, 0),
	}
}

// FindByID returns a webhook by ID.
func (m *MockWebhookRepository) FindByID(ctx context.Context, id uuid.UUID) (*models.Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if wh, ok := m.webhooks[id]; ok {
		return wh, nil
	}
	return nil, ErrWebhookNotFound
}

// FindByAgentAndEvent returns webhooks for an agent that are subscribed to an event.
func (m *MockWebhookRepository) FindByAgentAndEvent(ctx context.Context, agentID string, event string) ([]*models.Webhook, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	result := make([]*models.Webhook, 0)
	for _, wh := range m.webhooks {
		if wh.AgentID == agentID && wh.Status == models.WebhookStatusActive {
			for _, e := range wh.Events {
				if e == event {
					result = append(result, wh)
					break
				}
			}
		}
	}
	return result, nil
}

// UpdateDeliveryStatus updates the delivery status of a webhook.
func (m *MockWebhookRepository) UpdateDeliveryStatus(ctx context.Context, id uuid.UUID, failures int, lastFailure, lastSuccess *time.Time, status *models.WebhookStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.UpdateCalls = append(m.UpdateCalls, UpdateWebhookCall{
		WebhookID:           id,
		ConsecutiveFailures: &failures,
		LastFailureAt:       lastFailure,
		LastSuccessAt:       lastSuccess,
		Status:              status,
	})
	if wh, ok := m.webhooks[id]; ok {
		wh.ConsecutiveFailures = failures
		wh.LastFailureAt = lastFailure
		wh.LastSuccessAt = lastSuccess
		if status != nil {
			wh.Status = *status
		}
	}
	return nil
}

// AddWebhook adds a webhook to the mock.
func (m *MockWebhookRepository) AddWebhook(wh *models.Webhook) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.webhooks[wh.ID] = wh
}

// GetWebhook returns a webhook from the mock.
func (m *MockWebhookRepository) GetWebhook(id uuid.UUID) *models.Webhook {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.webhooks[id]
}

// --- Tests ---

func TestDeliverWebhook_Success(t *testing.T) {
	// Create a test server that returns 200 OK
	receivedPayload := make(chan []byte, 1)
	receivedHeaders := make(chan http.Header, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedPayload <- body
		receivedHeaders <- r.Header.Clone()
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-webhook-secret"

	repo.AddWebhook(&models.Webhook{
		ID:         webhookID,
		AgentID:    "test_agent",
		URL:        server.URL,
		Events:     []string{"answer.created"},
		SecretHash: secret, // In real use, this would be hashed
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	data := map[string]interface{}{
		"answer_id":   "abc123",
		"question_id": "xyz789",
	}

	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", data, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify payload was received
	select {
	case payload := <-receivedPayload:
		var received models.WebhookPayload
		if err := json.Unmarshal(payload, &received); err != nil {
			t.Fatalf("failed to unmarshal payload: %v", err)
		}
		if received.Event != "answer.created" {
			t.Errorf("expected event 'answer.created', got '%s'", received.Event)
		}
		if received.Data["answer_id"] != "abc123" {
			t.Errorf("expected answer_id 'abc123', got '%v'", received.Data["answer_id"])
		}
		// Verify timestamp is valid ISO 8601
		_, err := time.Parse(time.RFC3339, received.Timestamp)
		if err != nil {
			t.Errorf("timestamp not valid RFC3339: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for payload")
	}

	// Verify headers
	select {
	case headers := <-receivedHeaders:
		if headers.Get("X-Solvr-Delivery-Attempt") != "1" {
			t.Errorf("expected X-Solvr-Delivery-Attempt '1', got '%s'", headers.Get("X-Solvr-Delivery-Attempt"))
		}
		if headers.Get("X-Solvr-Webhook-ID") != webhookID.String() {
			t.Errorf("expected X-Solvr-Webhook-ID '%s', got '%s'", webhookID.String(), headers.Get("X-Solvr-Webhook-ID"))
		}
		if headers.Get("Content-Type") != "application/json" {
			t.Errorf("expected Content-Type 'application/json', got '%s'", headers.Get("Content-Type"))
		}
		sig := headers.Get("X-Solvr-Signature")
		if sig == "" {
			t.Error("expected X-Solvr-Signature header")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for headers")
	}

	// Verify webhook status was updated (success)
	wh := repo.GetWebhook(webhookID)
	if wh.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive_failures 0, got %d", wh.ConsecutiveFailures)
	}
	if wh.LastSuccessAt == nil {
		t.Error("expected last_success_at to be set")
	}
}

func TestDeliverWebhook_SignatureFormat(t *testing.T) {
	// Verify signature is HMAC-SHA256 in correct format
	receivedPayload := make(chan []byte, 1)
	receivedSignature := make(chan string, 1)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedPayload <- body
		receivedSignature <- r.Header.Get("X-Solvr-Signature")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "my-secret-key"

	repo.AddWebhook(&models.Webhook{
		ID:         webhookID,
		AgentID:    "test_agent",
		URL:        server.URL,
		Events:     []string{"answer.created"},
		SecretHash: secret,
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	data := map[string]interface{}{"test": "data"}
	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", data, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	// Verify signature format is "sha256=..."
	select {
	case sig := <-receivedSignature:
		if len(sig) < 8 || sig[:7] != "sha256=" {
			t.Errorf("signature should start with 'sha256=', got '%s'", sig)
		}

		// Verify it's a valid HMAC-SHA256
		payload := <-receivedPayload
		mac := hmac.New(sha256.New, []byte(secret))
		mac.Write(payload)
		expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))

		if sig != expectedSig {
			t.Errorf("signature mismatch: expected '%s', got '%s'", expectedSig, sig)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for signature")
	}
}

func TestDeliverWebhook_Failure_ServerError(t *testing.T) {
	// Create a test server that returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	repo.AddWebhook(&models.Webhook{
		ID:                  webhookID,
		AgentID:             "test_agent",
		URL:                 server.URL,
		Events:              []string{"answer.created"},
		SecretHash:          secret,
		Status:              models.WebhookStatusActive,
		ConsecutiveFailures: 0,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)
	if err == nil {
		t.Fatal("expected error for 500 response")
	}

	// Verify failure was recorded
	wh := repo.GetWebhook(webhookID)
	if wh.ConsecutiveFailures != 1 {
		t.Errorf("expected consecutive_failures 1, got %d", wh.ConsecutiveFailures)
	}
	if wh.LastFailureAt == nil {
		t.Error("expected last_failure_at to be set")
	}
}

func TestDeliverWebhook_Failure_Timeout(t *testing.T) {
	// Create a test server that times out
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Exceed client timeout
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	repo.AddWebhook(&models.Webhook{
		ID:         webhookID,
		AgentID:    "test_agent",
		URL:        server.URL,
		Events:     []string{"answer.created"},
		SecretHash: secret,
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	// Use a very short timeout
	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 50 * time.Millisecond})

	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)
	if err == nil {
		t.Fatal("expected error for timeout")
	}

	// Verify failure was recorded
	wh := repo.GetWebhook(webhookID)
	if wh.ConsecutiveFailures != 1 {
		t.Errorf("expected consecutive_failures 1, got %d", wh.ConsecutiveFailures)
	}
}

func TestDeliverWebhook_MarkFailingAfter5Failures(t *testing.T) {
	// Create a test server that always returns 500
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	// Start with 4 consecutive failures
	repo.AddWebhook(&models.Webhook{
		ID:                  webhookID,
		AgentID:             "test_agent",
		URL:                 server.URL,
		Events:              []string{"answer.created"},
		SecretHash:          secret,
		Status:              models.WebhookStatusActive,
		ConsecutiveFailures: 4,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	// 5th failure should mark as failing
	_ = service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)

	wh := repo.GetWebhook(webhookID)
	if wh.Status != models.WebhookStatusFailing {
		t.Errorf("expected status 'failing', got '%s'", wh.Status)
	}
	if wh.ConsecutiveFailures != 5 {
		t.Errorf("expected consecutive_failures 5, got %d", wh.ConsecutiveFailures)
	}
}

func TestDeliverWebhook_SuccessResetsFailures(t *testing.T) {
	// Create a test server that returns 200
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	// Start with some failures
	repo.AddWebhook(&models.Webhook{
		ID:                  webhookID,
		AgentID:             "test_agent",
		URL:                 server.URL,
		Events:              []string{"answer.created"},
		SecretHash:          secret,
		Status:              models.WebhookStatusFailing,
		ConsecutiveFailures: 3,
		CreatedAt:           time.Now(),
		UpdatedAt:           time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	wh := repo.GetWebhook(webhookID)
	if wh.ConsecutiveFailures != 0 {
		t.Errorf("expected consecutive_failures 0, got %d", wh.ConsecutiveFailures)
	}
	if wh.Status != models.WebhookStatusActive {
		t.Errorf("expected status 'active', got '%s'", wh.Status)
	}
}

func TestDeliverWebhook_WebhookNotFound(t *testing.T) {
	repo := NewMockWebhookRepository()
	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	err := service.DeliverWebhook(context.Background(), uuid.New(), "answer.created", nil, "secret")
	if err != ErrWebhookNotFound {
		t.Errorf("expected ErrWebhookNotFound, got %v", err)
	}
}

func TestDeliverWebhook_AttemptNumber(t *testing.T) {
	// Verify attempt number increases correctly
	attemptNumbers := make(chan string, 5)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptNumbers <- r.Header.Get("X-Solvr-Delivery-Attempt")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	repo.AddWebhook(&models.Webhook{
		ID:         webhookID,
		AgentID:    "test_agent",
		URL:        server.URL,
		Events:     []string{"answer.created"},
		SecretHash: secret,
		Status:     models.WebhookStatusActive,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	// Deliver with attempt 1 (default)
	_ = service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)

	// Deliver with explicit attempt 3
	_ = service.DeliverWebhookWithAttempt(context.Background(), webhookID, "answer.created", nil, secret, 3)

	select {
	case attempt := <-attemptNumbers:
		if attempt != "1" {
			t.Errorf("expected attempt '1', got '%s'", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}

	select {
	case attempt := <-attemptNumbers:
		if attempt != "3" {
			t.Errorf("expected attempt '3', got '%s'", attempt)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout")
	}
}

func TestGenerateSignature(t *testing.T) {
	service := &WebhookDeliveryService{}
	payload := []byte(`{"event":"answer.created","timestamp":"2026-01-31T19:00:00Z","data":{}}`)
	secret := "my-webhook-secret"

	sig := service.GenerateSignature(payload, secret)

	// Verify format
	if len(sig) < 8 || sig[:7] != "sha256=" {
		t.Errorf("signature should start with 'sha256=', got '%s'", sig)
	}

	// Verify deterministic
	sig2 := service.GenerateSignature(payload, secret)
	if sig != sig2 {
		t.Error("signature should be deterministic")
	}

	// Verify different secret produces different signature
	sig3 := service.GenerateSignature(payload, "different-secret")
	if sig == sig3 {
		t.Error("different secret should produce different signature")
	}

	// Verify HMAC-SHA256 calculation
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedSig := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	if sig != expectedSig {
		t.Errorf("signature mismatch: expected '%s', got '%s'", expectedSig, sig)
	}
}

func TestBuildPayload(t *testing.T) {
	service := &WebhookDeliveryService{}

	data := map[string]interface{}{
		"answer_id": "abc123",
		"content":   "Test answer",
	}

	payload, err := service.BuildPayload("answer.created", data)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var parsed models.WebhookPayload
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed.Event != "answer.created" {
		t.Errorf("expected event 'answer.created', got '%s'", parsed.Event)
	}

	// Verify timestamp is valid RFC3339
	_, err = time.Parse(time.RFC3339, parsed.Timestamp)
	if err != nil {
		t.Errorf("timestamp not valid RFC3339: %v", err)
	}

	if parsed.Data["answer_id"] != "abc123" {
		t.Errorf("expected answer_id 'abc123', got '%v'", parsed.Data["answer_id"])
	}
}

func TestBuildPayload_NilData(t *testing.T) {
	service := &WebhookDeliveryService{}

	payload, err := service.BuildPayload("problem.solved", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	var parsed models.WebhookPayload
	if err := json.Unmarshal(payload, &parsed); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}

	if parsed.Data == nil {
		t.Error("expected empty data object, got nil")
	}
}

func TestGetRetryDelays(t *testing.T) {
	service := &WebhookDeliveryService{}
	delays := service.GetRetryDelays()

	expected := []time.Duration{
		0,                  // Attempt 1: Immediate
		1 * time.Minute,    // Attempt 2: 1 minute
		5 * time.Minute,    // Attempt 3: 5 minutes
		30 * time.Minute,   // Attempt 4: 30 minutes
		2 * time.Hour,      // Attempt 5: 2 hours
	}

	if len(delays) != len(expected) {
		t.Fatalf("expected %d delays, got %d", len(expected), len(delays))
	}

	for i, d := range delays {
		if d != expected[i] {
			t.Errorf("delay[%d]: expected %v, got %v", i, expected[i], d)
		}
	}
}

func TestDeliverWebhook_HTTP2xxSuccess(t *testing.T) {
	// Test various 2xx status codes are treated as success
	successCodes := []int{200, 201, 202, 204}

	for _, code := range successCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			repo := NewMockWebhookRepository()
			webhookID := uuid.New()
			secret := "test-secret"

			repo.AddWebhook(&models.Webhook{
				ID:         webhookID,
				AgentID:    "test_agent",
				URL:        server.URL,
				Events:     []string{"answer.created"},
				SecretHash: secret,
				Status:     models.WebhookStatusActive,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			})

			service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

			err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)
			if err != nil {
				t.Errorf("expected no error for %d, got %v", code, err)
			}
		})
	}
}

func TestDeliverWebhook_Non2xxFailure(t *testing.T) {
	// Test various non-2xx status codes are treated as failure
	failureCodes := []int{400, 401, 403, 404, 500, 502, 503}

	for _, code := range failureCodes {
		t.Run(http.StatusText(code), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(code)
			}))
			defer server.Close()

			repo := NewMockWebhookRepository()
			webhookID := uuid.New()
			secret := "test-secret"

			repo.AddWebhook(&models.Webhook{
				ID:         webhookID,
				AgentID:    "test_agent",
				URL:        server.URL,
				Events:     []string{"answer.created"},
				SecretHash: secret,
				Status:     models.WebhookStatusActive,
				CreatedAt:  time.Now(),
				UpdatedAt:  time.Now(),
			})

			service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

			err := service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)
			if err == nil {
				t.Errorf("expected error for %d", code)
			}
		})
	}
}

func TestDeliverWebhook_AutoDisableAfter24hContinuousFailure(t *testing.T) {
	// Per SPEC.md Part 12.3:
	// "After 24h of continuous failure: webhook auto-paused"
	// The webhook should be set to status='disabled' after 24h continuous failure

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	// Webhook has been failing for more than 24 hours
	failureTime := time.Now().Add(-25 * time.Hour)
	repo.AddWebhook(&models.Webhook{
		ID:                  webhookID,
		AgentID:             "test_agent",
		URL:                 server.URL,
		Events:              []string{"answer.created"},
		SecretHash:          secret,
		Status:              models.WebhookStatusFailing,
		ConsecutiveFailures: 10,
		LastFailureAt:       &failureTime,
		CreatedAt:           time.Now().Add(-48 * time.Hour),
		UpdatedAt:           time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	// Another failure - this should trigger auto-disable
	_ = service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)

	wh := repo.GetWebhook(webhookID)
	if wh.Status != models.WebhookStatusDisabled {
		t.Errorf("expected status 'disabled' after 24h continuous failure, got '%s'", wh.Status)
	}
}

func TestDeliverWebhook_NotDisabledIfFailingLessThan24h(t *testing.T) {
	// Webhook should NOT be disabled if it hasn't been failing for 24h yet

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	repo := NewMockWebhookRepository()
	webhookID := uuid.New()
	secret := "test-secret"

	// Webhook has been failing for only 12 hours (not yet 24h)
	failureTime := time.Now().Add(-12 * time.Hour)
	repo.AddWebhook(&models.Webhook{
		ID:                  webhookID,
		AgentID:             "test_agent",
		URL:                 server.URL,
		Events:              []string{"answer.created"},
		SecretHash:          secret,
		Status:              models.WebhookStatusFailing,
		ConsecutiveFailures: 10,
		LastFailureAt:       &failureTime,
		CreatedAt:           time.Now().Add(-48 * time.Hour),
		UpdatedAt:           time.Now(),
	})

	service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

	// Another failure - should still be 'failing', not 'disabled'
	_ = service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)

	wh := repo.GetWebhook(webhookID)
	if wh.Status != models.WebhookStatusFailing {
		t.Errorf("expected status 'failing' (less than 24h), got '%s'", wh.Status)
	}
}

func TestDeliverWebhook_CalculateContinuousFailureDuration(t *testing.T) {
	// Tests that we correctly calculate the duration since first failure
	// When a webhook is already failing, we use LastFailureAt to determine
	// continuous failure duration

	testCases := []struct {
		name           string
		lastFailureAge time.Duration
		expectedStatus models.WebhookStatus
	}{
		{
			name:           "failure for 23h59m - still failing",
			lastFailureAge: 23*time.Hour + 59*time.Minute,
			expectedStatus: models.WebhookStatusFailing,
		},
		{
			name:           "failure for exactly 24h - should be disabled",
			lastFailureAge: 24 * time.Hour,
			expectedStatus: models.WebhookStatusDisabled,
		},
		{
			name:           "failure for 48h - should be disabled",
			lastFailureAge: 48 * time.Hour,
			expectedStatus: models.WebhookStatusDisabled,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer server.Close()

			repo := NewMockWebhookRepository()
			webhookID := uuid.New()
			secret := "test-secret"

			failureTime := time.Now().Add(-tc.lastFailureAge)
			repo.AddWebhook(&models.Webhook{
				ID:                  webhookID,
				AgentID:             "test_agent",
				URL:                 server.URL,
				Events:              []string{"answer.created"},
				SecretHash:          secret,
				Status:              models.WebhookStatusFailing,
				ConsecutiveFailures: 10,
				LastFailureAt:       &failureTime,
				CreatedAt:           time.Now().Add(-72 * time.Hour),
				UpdatedAt:           time.Now(),
			})

			service := NewWebhookDeliveryService(repo, &http.Client{Timeout: 10 * time.Second})

			_ = service.DeliverWebhook(context.Background(), webhookID, "answer.created", nil, secret)

			wh := repo.GetWebhook(webhookID)
			if wh.Status != tc.expectedStatus {
				t.Errorf("expected status '%s', got '%s'", tc.expectedStatus, wh.Status)
			}
		})
	}
}
