package auth

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockAgentDB is a mock implementation of the AgentDB interface for testing.
type MockAgentDB struct {
	Agents       map[string]*models.Agent // Key is the agent ID
	ReturnError  error
	QueriedAgent string
}

// NewMockAgentDB creates a new mock database with an optional test agent.
func NewMockAgentDB() *MockAgentDB {
	return &MockAgentDB{
		Agents: make(map[string]*models.Agent),
	}
}

// GetAgentByAPIKeyHash finds an agent by checking the API key against stored hashes.
// In the mock, we iterate through all agents and use CompareAPIKey.
func (m *MockAgentDB) GetAgentByAPIKeyHash(ctx context.Context, key string) (*models.Agent, error) {
	if m.ReturnError != nil {
		return nil, m.ReturnError
	}

	// In real implementation, we'd query the database.
	// Here we simulate by checking each agent's hash.
	for id, agent := range m.Agents {
		if agent.APIKeyHash != "" {
			if err := CompareAPIKey(key, agent.APIKeyHash); err == nil {
				m.QueriedAgent = id
				return agent, nil
			}
		}
	}

	return nil, nil // Not found
}

// AddTestAgent adds a test agent with a hashed API key.
func (m *MockAgentDB) AddTestAgent(id, displayName, apiKey string) (*models.Agent, error) {
	hash, err := HashAPIKey(apiKey)
	if err != nil {
		return nil, err
	}

	agent := &models.Agent{
		ID:          id,
		DisplayName: displayName,
		APIKeyHash:  hash,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.Agents[id] = agent
	return agent, nil
}

func TestValidateAPIKey_ValidKey(t *testing.T) {
	db := NewMockAgentDB()
	testKey := "solvr_testkey123456789012345678901234567890"

	_, err := db.AddTestAgent("test_agent", "Test Agent", testKey)
	if err != nil {
		t.Fatalf("failed to add test agent: %v", err)
	}

	validator := NewAPIKeyValidator(db)
	agent, err := validator.ValidateAPIKey(context.Background(), testKey)

	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if agent == nil {
		t.Fatal("expected agent, got nil")
	}
	if agent.ID != "test_agent" {
		t.Errorf("expected agent ID 'test_agent', got '%s'", agent.ID)
	}
	if agent.DisplayName != "Test Agent" {
		t.Errorf("expected display name 'Test Agent', got '%s'", agent.DisplayName)
	}
}

func TestValidateAPIKey_InvalidKey(t *testing.T) {
	db := NewMockAgentDB()
	testKey := "solvr_testkey123456789012345678901234567890"
	wrongKey := "solvr_wrongkey12345678901234567890123456789"

	_, err := db.AddTestAgent("test_agent", "Test Agent", testKey)
	if err != nil {
		t.Fatalf("failed to add test agent: %v", err)
	}

	validator := NewAPIKeyValidator(db)
	agent, err := validator.ValidateAPIKey(context.Background(), wrongKey)

	if err == nil {
		t.Fatal("expected error for invalid key, got nil")
	}
	if agent != nil {
		t.Error("expected nil agent for invalid key")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T", err)
	}
	if authErr.Code != ErrCodeInvalidAPIKey {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeInvalidAPIKey, authErr.Code)
	}
}

func TestValidateAPIKey_EmptyKey(t *testing.T) {
	db := NewMockAgentDB()
	validator := NewAPIKeyValidator(db)

	agent, err := validator.ValidateAPIKey(context.Background(), "")

	if err == nil {
		t.Fatal("expected error for empty key, got nil")
	}
	if agent != nil {
		t.Error("expected nil agent for empty key")
	}
}

func TestValidateAPIKey_InvalidKeyFormat(t *testing.T) {
	db := NewMockAgentDB()
	validator := NewAPIKeyValidator(db)

	// Key without solvr_ prefix
	agent, err := validator.ValidateAPIKey(context.Background(), "notavalidkeyformat123")

	if err == nil {
		t.Fatal("expected error for invalid key format, got nil")
	}
	if agent != nil {
		t.Error("expected nil agent for invalid format")
	}

	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("expected AuthError, got %T", err)
	}
	if authErr.Code != ErrCodeInvalidAPIKey {
		t.Errorf("expected error code '%s', got '%s'", ErrCodeInvalidAPIKey, authErr.Code)
	}
}

func TestValidateAPIKey_DatabaseError(t *testing.T) {
	db := NewMockAgentDB()
	db.ReturnError = errors.New("database connection failed")

	validator := NewAPIKeyValidator(db)
	agent, err := validator.ValidateAPIKey(context.Background(), "solvr_validkey123456789012345678901234567890")

	if err == nil {
		t.Fatal("expected error for database failure, got nil")
	}
	if agent != nil {
		t.Error("expected nil agent for database error")
	}
}

func TestValidateAPIKey_NoAgentsExist(t *testing.T) {
	db := NewMockAgentDB() // Empty database
	validator := NewAPIKeyValidator(db)

	agent, err := validator.ValidateAPIKey(context.Background(), "solvr_somekey12345678901234567890123456789")

	if err == nil {
		t.Fatal("expected error for non-existent agent, got nil")
	}
	if agent != nil {
		t.Error("expected nil agent when no agents exist")
	}
}

func TestValidateAPIKey_MultipleAgents(t *testing.T) {
	db := NewMockAgentDB()

	key1 := "solvr_agentone1234567890123456789012345678"
	key2 := "solvr_agenttwo1234567890123456789012345678"

	_, err := db.AddTestAgent("agent_one", "Agent One", key1)
	if err != nil {
		t.Fatalf("failed to add agent one: %v", err)
	}
	_, err = db.AddTestAgent("agent_two", "Agent Two", key2)
	if err != nil {
		t.Fatalf("failed to add agent two: %v", err)
	}

	validator := NewAPIKeyValidator(db)

	// Validate first agent's key
	agent, err := validator.ValidateAPIKey(context.Background(), key1)
	if err != nil {
		t.Fatalf("expected no error for agent one's key, got: %v", err)
	}
	if agent.ID != "agent_one" {
		t.Errorf("expected agent ID 'agent_one', got '%s'", agent.ID)
	}

	// Validate second agent's key
	agent, err = validator.ValidateAPIKey(context.Background(), key2)
	if err != nil {
		t.Fatalf("expected no error for agent two's key, got: %v", err)
	}
	if agent.ID != "agent_two" {
		t.Errorf("expected agent ID 'agent_two', got '%s'", agent.ID)
	}
}

func TestIsAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		key      string
		expected bool
	}{
		{"valid key with prefix", "solvr_abc123", true},
		{"valid long key", "solvr_testkey123456789012345678901234567890", true},
		{"missing prefix", "abc123", false},
		{"empty string", "", false},
		{"wrong prefix", "solvrr_abc", false},
		{"only prefix", "solvr_", true}, // Has prefix, even if no suffix
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsAPIKey(tt.key)
			if result != tt.expected {
				t.Errorf("IsAPIKey(%q) = %v, expected %v", tt.key, result, tt.expected)
			}
		})
	}
}
