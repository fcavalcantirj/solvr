package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// =============================================================================
// Moltbook Auth Tests (PRD AGENT-AUTH requirements)
// Per SPEC.md Part 5.2: POST /auth/moltbook
// =============================================================================

// TestMoltbookAuth_MissingToken tests that missing identity_token returns 400.
func TestMoltbookAuth_MissingToken(t *testing.T) {
	handler := NewMoltbookHandler(&MoltbookConfig{
		MoltbookAPIURL: "http://mock-moltbook.test",
	}, nil)

	// Empty body
	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader("{}"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "VALIDATION_ERROR" {
		t.Errorf("expected error code VALIDATION_ERROR, got %s", resp.Error.Code)
	}
	if !strings.Contains(resp.Error.Message, "identity_token") {
		t.Errorf("expected error message to mention identity_token, got %s", resp.Error.Message)
	}
}

// TestMoltbookAuth_InvalidJSON tests that invalid JSON returns 400.
func TestMoltbookAuth_InvalidJSON(t *testing.T) {
	handler := NewMoltbookHandler(&MoltbookConfig{
		MoltbookAPIURL: "http://mock-moltbook.test",
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader("not json"))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestMoltbookAuth_EmptyToken tests that empty identity_token returns 400.
func TestMoltbookAuth_EmptyToken(t *testing.T) {
	handler := NewMoltbookHandler(&MoltbookConfig{
		MoltbookAPIURL: "http://mock-moltbook.test",
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader(`{"identity_token": ""}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Errorf("expected status %d, got %d", http.StatusBadRequest, rec.Code)
	}
}

// TestMoltbookAuth_InvalidToken tests that invalid token returns 401.
func TestMoltbookAuth_InvalidToken(t *testing.T) {
	// Mock Moltbook server that returns invalid token error
	mockMoltbook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error": "invalid_token",
		})
	}))
	defer mockMoltbook.Close()

	handler := NewMoltbookHandler(&MoltbookConfig{
		MoltbookAPIURL: mockMoltbook.URL,
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader(`{"identity_token": "invalid-token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusUnauthorized, rec.Code, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "INVALID_MOLTBOOK_TOKEN" {
		t.Errorf("expected error code INVALID_MOLTBOOK_TOKEN, got %s", resp.Error.Code)
	}
}

// TestMoltbookAuth_MoltbookAPIUnavailable tests that unavailable Moltbook API returns 502.
func TestMoltbookAuth_MoltbookAPIUnavailable(t *testing.T) {
	// Use an invalid URL to simulate unavailable API
	handler := NewMoltbookHandler(&MoltbookConfig{
		MoltbookAPIURL: "http://localhost:1", // Invalid port
	}, nil)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader(`{"identity_token": "test-token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusBadGateway {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusBadGateway, rec.Code, rec.Body.String())
	}

	var resp ErrorResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if resp.Error.Code != "BAD_GATEWAY" {
		t.Errorf("expected error code BAD_GATEWAY, got %s", resp.Error.Code)
	}
}

// TestMoltbookAuth_ValidToken_NewAgent tests that valid token creates new agent.
func TestMoltbookAuth_ValidToken_NewAgent(t *testing.T) {
	// Mock Moltbook server that returns valid agent identity
	mockMoltbook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify request
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/auth/verify" {
			t.Errorf("expected /v1/auth/verify, got %s", r.URL.Path)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": true,
			"agent": map[string]interface{}{
				"id":           "moltbook-agent-123",
				"display_name": "Claude Assistant",
				"karma":        150,
				"post_count":   42,
			},
		})
	}))
	defer mockMoltbook.Close()

	// Create mock agent service
	mockAgentService := &MockMoltbookAgentService{
		agents: make(map[string]*MockAgentData),
	}

	handler := NewMoltbookHandlerWithDeps(
		&MoltbookConfig{
			MoltbookAPIURL: mockMoltbook.URL,
		},
		mockAgentService,
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader(`{"identity_token": "valid-token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp MoltbookAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify agent was created
	if resp.Data.Agent.ID == "" {
		t.Error("expected agent ID to be set")
	}
	if resp.Data.Agent.DisplayName != "Claude Assistant" {
		t.Errorf("expected display_name Claude Assistant, got %s", resp.Data.Agent.DisplayName)
	}
	if !resp.Data.Agent.MoltbookVerified {
		t.Error("expected moltbook_verified to be true")
	}
	if resp.Data.Agent.ImportedReputation != 150 {
		t.Errorf("expected imported_reputation 150, got %d", resp.Data.Agent.ImportedReputation)
	}

	// Verify API key was returned
	if resp.Data.APIKey == "" {
		t.Error("expected api_key to be set")
	}
	if !strings.HasPrefix(resp.Data.APIKey, "solvr_") {
		t.Errorf("expected api_key to start with solvr_, got %s", resp.Data.APIKey)
	}
}

// TestMoltbookAuth_ValidToken_ExistingAgent tests that valid token returns existing agent.
func TestMoltbookAuth_ValidToken_ExistingAgent(t *testing.T) {
	// Mock Moltbook server
	mockMoltbook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": true,
			"agent": map[string]interface{}{
				"id":           "moltbook-agent-123",
				"display_name": "Claude Assistant",
				"karma":        200, // Updated karma
				"post_count":   50,
			},
		})
	}))
	defer mockMoltbook.Close()

	// Create mock agent service with existing agent
	mockAgentService := &MockMoltbookAgentService{
		agents: map[string]*MockAgentData{
			"moltbook-agent-123": {
				ID:              "solvr-agent-456",
				MoltbookID:      "moltbook-agent-123",
				DisplayName:     "Claude Assistant",
				MoltbookVerified: true,
				Reputation:      150,
			},
		},
	}

	handler := NewMoltbookHandlerWithDeps(
		&MoltbookConfig{
			MoltbookAPIURL: mockMoltbook.URL,
		},
		mockAgentService,
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader(`{"identity_token": "valid-token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp MoltbookAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify existing agent was returned
	if resp.Data.Agent.ID != "solvr-agent-456" {
		t.Errorf("expected agent ID solvr-agent-456, got %s", resp.Data.Agent.ID)
	}
	if !resp.Data.Agent.MoltbookVerified {
		t.Error("expected moltbook_verified to be true")
	}
}

// TestMoltbookAuth_KarmaConversion tests that Moltbook karma is imported as reputation.
func TestMoltbookAuth_KarmaConversion(t *testing.T) {
	// Mock Moltbook server
	mockMoltbook := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"valid": true,
			"agent": map[string]interface{}{
				"id":           "moltbook-agent-789",
				"display_name": "Helper Bot",
				"karma":        250,
				"post_count":   100,
			},
		})
	}))
	defer mockMoltbook.Close()

	mockAgentService := &MockMoltbookAgentService{
		agents: make(map[string]*MockAgentData),
	}

	handler := NewMoltbookHandlerWithDeps(
		&MoltbookConfig{
			MoltbookAPIURL: mockMoltbook.URL,
		},
		mockAgentService,
	)

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/moltbook", strings.NewReader(`{"identity_token": "valid-token"}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	handler.Authenticate(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d. Body: %s", http.StatusOK, rec.Code, rec.Body.String())
	}

	var resp MoltbookAuthResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	// Verify karma was imported (1:1 conversion per SPEC)
	if resp.Data.Agent.ImportedReputation != 250 {
		t.Errorf("expected imported_reputation 250, got %d", resp.Data.Agent.ImportedReputation)
	}
}

// =============================================================================
// Mock Types and Helpers
// =============================================================================

// MoltbookAuthResponse is the response for Moltbook authentication.
type MoltbookAuthResponse struct {
	Data struct {
		Agent  MoltbookAgentResponse `json:"agent"`
		APIKey string                `json:"api_key"`
	} `json:"data"`
}

// MoltbookAgentResponse represents the agent in Moltbook auth response.
type MoltbookAgentResponse struct {
	ID               string `json:"id"`
	DisplayName      string `json:"display_name"`
	MoltbookVerified bool   `json:"moltbook_verified"`
	ImportedReputation int  `json:"imported_reputation"`
}

// MockMoltbookAgentService is a mock implementation of MoltbookAgentServiceInterface.
type MockMoltbookAgentService struct {
	agents map[string]*MockAgentData
}

// MockAgentData represents mock agent data.
type MockAgentData struct {
	ID               string
	MoltbookID       string
	DisplayName      string
	MoltbookVerified bool
	Reputation       int
	APIKeyHash       string
}

// FindByMoltbookID finds an agent by Moltbook ID.
func (m *MockMoltbookAgentService) FindByMoltbookID(ctx context.Context, moltbookID string) (*MoltbookAgentRecord, error) {
	if agent, ok := m.agents[moltbookID]; ok {
		return &MoltbookAgentRecord{
			ID:               agent.ID,
			MoltbookID:       agent.MoltbookID,
			DisplayName:      agent.DisplayName,
			MoltbookVerified: agent.MoltbookVerified,
			Reputation:       agent.Reputation,
		}, nil
	}
	return nil, nil // Not found
}

// CreateAgentFromMoltbook creates a new agent from Moltbook data.
func (m *MockMoltbookAgentService) CreateAgentFromMoltbook(ctx context.Context, data *MoltbookVerifyAgentData) (*MoltbookAgentRecord, string, error) {
	agent := &MockAgentData{
		ID:               "solvr-new-" + data.MoltbookID,
		MoltbookID:       data.MoltbookID,
		DisplayName:      data.DisplayName,
		MoltbookVerified: true,
		Reputation:       data.Reputation,
	}
	m.agents[data.MoltbookID] = agent
	return &MoltbookAgentRecord{
		ID:               agent.ID,
		MoltbookID:       agent.MoltbookID,
		DisplayName:      agent.DisplayName,
		MoltbookVerified: agent.MoltbookVerified,
		Reputation:       agent.Reputation,
	}, "solvr_test_api_key_" + data.MoltbookID, nil
}

// GenerateNewAPIKey generates a new API key for an existing agent.
func (m *MockMoltbookAgentService) GenerateNewAPIKey(ctx context.Context, agentID string) (string, error) {
	return "solvr_new_api_key_" + agentID, nil
}
