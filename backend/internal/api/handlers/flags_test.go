// Package handlers contains HTTP request handlers for the Solvr API.
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

// MockFlagsRepository implements FlagsRepositoryInterface for testing.
type MockFlagsRepository struct {
	CreateFlagFunc   func(ctx context.Context, flag *models.Flag) (*models.Flag, error)
	TargetExistsFunc func(ctx context.Context, targetType, targetID string) (bool, error)
	FlagExistsFunc   func(ctx context.Context, targetType, targetID, reporterType, reporterID string) (bool, error)
}

func (m *MockFlagsRepository) CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
	if m.CreateFlagFunc != nil {
		return m.CreateFlagFunc(ctx, flag)
	}
	// Default: return the flag with ID and timestamp
	flag.ID = uuid.New()
	flag.Status = "pending"
	flag.CreatedAt = time.Now()
	return flag, nil
}

func (m *MockFlagsRepository) TargetExists(ctx context.Context, targetType, targetID string) (bool, error) {
	if m.TargetExistsFunc != nil {
		return m.TargetExistsFunc(ctx, targetType, targetID)
	}
	return true, nil
}

func (m *MockFlagsRepository) FlagExists(ctx context.Context, targetType, targetID, reporterType, reporterID string) (bool, error) {
	if m.FlagExistsFunc != nil {
		return m.FlagExistsFunc(ctx, targetType, targetID, reporterType, reporterID)
	}
	return false, nil
}

// Helper function to add auth context for flags tests
func addFlagsAuthContext(r *http.Request, claims *auth.Claims) *http.Request {
	ctx := auth.ContextWithClaims(r.Context(), claims)
	return r.WithContext(ctx)
}

// TestCreateFlag_Success tests successful flag creation.
func TestCreateFlag_Success(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	targetID := uuid.New().String()
	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   targetID,
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["target_type"] != "post" {
		t.Errorf("expected target_type 'post', got %v", data["target_type"])
	}
	if data["reason"] != "spam" {
		t.Errorf("expected reason 'spam', got %v", data["reason"])
	}
	if data["status"] != "pending" {
		t.Errorf("expected status 'pending', got %v", data["status"])
	}
}

// TestCreateFlag_NoAuth tests flag creation without authentication.
func TestCreateFlag_NoAuth(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	// No auth context
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("expected status 401, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "UNAUTHORIZED" {
		t.Errorf("expected error code 'UNAUTHORIZED', got %v", errObj["code"])
	}
}

// TestCreateFlag_InvalidJSON tests flag creation with invalid JSON.
func TestCreateFlag_InvalidJSON(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader([]byte("not json")))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d", w.Code)
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code 'VALIDATION_ERROR', got %v", errObj["code"])
	}
}

// TestCreateFlag_MissingTargetType tests flag creation with missing target_type.
func TestCreateFlag_MissingTargetType(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_id": uuid.New().String(),
		"reason":    "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code 'VALIDATION_ERROR', got %v", errObj["code"])
	}
}

// TestCreateFlag_InvalidTargetType tests flag creation with invalid target_type.
func TestCreateFlag_InvalidTargetType(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "invalid_type",
		"target_id":   uuid.New().String(),
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code 'VALIDATION_ERROR', got %v", errObj["code"])
	}
	// Check message contains valid target types
	message, _ := errObj["message"].(string)
	if message == "" {
		t.Error("expected error message")
	}
}

// TestCreateFlag_ValidTargetTypes tests all valid target types.
func TestCreateFlag_ValidTargetTypes(t *testing.T) {
	validTypes := []string{"post", "comment", "answer", "approach", "response"}

	for _, targetType := range validTypes {
		t.Run(targetType, func(t *testing.T) {
			mockRepo := &MockFlagsRepository{}
			handler := NewFlagsHandler(mockRepo)

			body := map[string]interface{}{
				"target_type": targetType,
				"target_id":   uuid.New().String(),
				"reason":      "spam",
			}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
			req = addFlagsAuthContext(req, &auth.Claims{
				UserID: uuid.New().String(),
				Email:  "test@example.com",
				Role:   "user",
			})
			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status 201 for target_type '%s', got %d: %s", targetType, w.Code, w.Body.String())
			}
		})
	}
}

// TestCreateFlag_MissingReason tests flag creation with missing reason.
func TestCreateFlag_MissingReason(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code 'VALIDATION_ERROR', got %v", errObj["code"])
	}
}

// TestCreateFlag_InvalidReason tests flag creation with invalid reason.
func TestCreateFlag_InvalidReason(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"reason":      "invalid_reason",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "VALIDATION_ERROR" {
		t.Errorf("expected error code 'VALIDATION_ERROR', got %v", errObj["code"])
	}
}

// TestCreateFlag_ValidReasons tests all valid reasons.
func TestCreateFlag_ValidReasons(t *testing.T) {
	validReasons := []string{"spam", "offensive", "duplicate", "incorrect", "low_quality", "other"}

	for _, reason := range validReasons {
		t.Run(reason, func(t *testing.T) {
			mockRepo := &MockFlagsRepository{}
			handler := NewFlagsHandler(mockRepo)

			body := map[string]interface{}{
				"target_type": "post",
				"target_id":   uuid.New().String(),
				"reason":      reason,
			}
			bodyBytes, _ := json.Marshal(body)

			req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
			req = addFlagsAuthContext(req, &auth.Claims{
				UserID: uuid.New().String(),
				Email:  "test@example.com",
				Role:   "user",
			})
			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != http.StatusCreated {
				t.Errorf("expected status 201 for reason '%s', got %d: %s", reason, w.Code, w.Body.String())
			}
		})
	}
}

// TestCreateFlag_MissingTargetID tests flag creation with missing target_id.
func TestCreateFlag_MissingTargetID(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateFlag_InvalidTargetID tests flag creation with invalid target_id format.
func TestCreateFlag_InvalidTargetID(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   "not-a-uuid",
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("expected status 400, got %d: %s", w.Code, w.Body.String())
	}
}

// TestCreateFlag_TargetNotFound tests flag creation when target doesn't exist.
func TestCreateFlag_TargetNotFound(t *testing.T) {
	mockRepo := &MockFlagsRepository{
		TargetExistsFunc: func(ctx context.Context, targetType, targetID string) (bool, error) {
			return false, nil
		},
	}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "NOT_FOUND" {
		t.Errorf("expected error code 'NOT_FOUND', got %v", errObj["code"])
	}
}

// TestCreateFlag_WithDetails tests flag creation with optional details.
func TestCreateFlag_WithDetails(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"reason":      "other",
		"details":     "This content contains misleading information",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	if data["details"] != "This content contains misleading information" {
		t.Errorf("expected details to be preserved, got %v", data["details"])
	}
}

// TestCreateFlag_DuplicateFlag tests that duplicate flags are rejected.
func TestCreateFlag_DuplicateFlag(t *testing.T) {
	targetID := uuid.New().String()
	userID := uuid.New().String()

	mockRepo := &MockFlagsRepository{
		FlagExistsFunc: func(ctx context.Context, targetType, tid, reporterType, reporterID string) (bool, error) {
			// Simulate existing flag from same user
			return true, nil
		},
	}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   targetID,
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: userID,
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusConflict {
		t.Errorf("expected status 409, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "DUPLICATE_FLAG" {
		t.Errorf("expected error code 'DUPLICATE_FLAG', got %v", errObj["code"])
	}
}

// TestCreateFlag_DatabaseError tests handling of database errors.
func TestCreateFlag_DatabaseError(t *testing.T) {
	mockRepo := &MockFlagsRepository{
		CreateFlagFunc: func(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
			return nil, errors.New("database error")
		},
	}
	handler := NewFlagsHandler(mockRepo)

	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: uuid.New().String(),
		Email:  "test@example.com",
		Role:   "user",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status 500, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	errObj, ok := resp["error"].(map[string]interface{})
	if !ok {
		t.Fatal("expected error object in response")
	}
	if errObj["code"] != "INTERNAL_ERROR" {
		t.Errorf("expected error code 'INTERNAL_ERROR', got %v", errObj["code"])
	}
}

// TestCreateFlag_AsAgent tests flag creation by an AI agent.
func TestCreateFlag_AsAgent(t *testing.T) {
	mockRepo := &MockFlagsRepository{}
	handler := NewFlagsHandler(mockRepo)

	agentID := "claude_helper"
	body := map[string]interface{}{
		"target_type": "post",
		"target_id":   uuid.New().String(),
		"reason":      "spam",
	}
	bodyBytes, _ := json.Marshal(body)

	req := httptest.NewRequest("POST", "/v1/flags", bytes.NewReader(bodyBytes))
	// Note: For agents, we'd have different auth context handling
	// For now, using JWT claims with agent indication
	req = addFlagsAuthContext(req, &auth.Claims{
		UserID: agentID, // Agent ID instead of UUID
		Email:  "",
		Role:   "agent",
	})
	w := httptest.NewRecorder()

	handler.Create(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d: %s", w.Code, w.Body.String())
	}

	var resp map[string]interface{}
	json.Unmarshal(w.Body.Bytes(), &resp)

	data, ok := resp["data"].(map[string]interface{})
	if !ok {
		t.Fatal("expected data object in response")
	}
	// Agent flags should have reporter_type = "agent"
	if data["reporter_type"] != "agent" {
		t.Errorf("expected reporter_type 'agent', got %v", data["reporter_type"])
	}
}

// TestIsValidFlagTargetType tests the target type validation helper.
func TestIsValidFlagTargetType(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"post", true},
		{"comment", true},
		{"answer", true},
		{"approach", true},
		{"response", true},
		{"invalid", false},
		{"", false},
		{"POST", false}, // Case sensitive
		{"user", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := isValidFlagTargetType(tt.input)
			if result != tt.expected {
				t.Errorf("isValidFlagTargetType(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// TestIsValidFlagReason tests the reason validation helper.
func TestIsValidFlagReason(t *testing.T) {
	tests := []struct {
		input    string
		expected bool
	}{
		{"spam", true},
		{"offensive", true},
		{"duplicate", true},
		{"incorrect", true},
		{"low_quality", true},
		{"other", true},
		{"invalid", false},
		{"", false},
		{"SPAM", false}, // Case sensitive
		{"inappropriate", false},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := models.IsValidFlagReason(tt.input)
			if result != tt.expected {
				t.Errorf("IsValidFlagReason(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
