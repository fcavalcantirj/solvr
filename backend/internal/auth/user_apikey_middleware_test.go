package auth

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// MockUserAPIKeyDB implements UserAPIKeyDB for testing.
type MockUserAPIKeyDB struct {
	keys        map[string]*models.UserAPIKey
	users       map[string]*models.User
	lastUsedIDs []string // track UpdateLastUsed calls
}

// NewMockUserAPIKeyDB creates a new mock database.
func NewMockUserAPIKeyDB() *MockUserAPIKeyDB {
	return &MockUserAPIKeyDB{
		keys:        make(map[string]*models.UserAPIKey),
		users:       make(map[string]*models.User),
		lastUsedIDs: make([]string, 0),
	}
}

// AddTestUserAPIKey adds a test API key to the mock database.
// The key should be the plain text key (it will be hashed).
func (m *MockUserAPIKeyDB) AddTestUserAPIKey(id, userID, name, plainKey string) (*models.UserAPIKey, error) {
	hash, err := HashAPIKey(plainKey)
	if err != nil {
		return nil, err
	}

	key := &models.UserAPIKey{
		ID:        id,
		UserID:    userID,
		Name:      name,
		KeyHash:   hash,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.keys[id] = key

	// Ensure user exists
	if _, ok := m.users[userID]; !ok {
		m.users[userID] = &models.User{
			ID:       userID,
			Username: "testuser",
			Email:    "test@example.com",
		}
	}

	return key, nil
}

// AddTestUser adds a test user to the mock database.
func (m *MockUserAPIKeyDB) AddTestUser(id, username, email string) *models.User {
	user := &models.User{
		ID:       id,
		Username: username,
		Email:    email,
	}
	m.users[id] = user
	return user
}

// GetUserByAPIKey finds a user by validating the API key.
func (m *MockUserAPIKeyDB) GetUserByAPIKey(ctx context.Context, plainKey string) (*models.User, *models.UserAPIKey, error) {
	// Search all keys to find one that matches
	for _, key := range m.keys {
		// Skip revoked keys
		if key.RevokedAt != nil {
			continue
		}
		// Compare using bcrypt
		err := CompareAPIKey(plainKey, key.KeyHash)
		if err == nil {
			// Found matching key
			user, ok := m.users[key.UserID]
			if !ok {
				return nil, nil, nil
			}
			return user, key, nil
		}
	}
	return nil, nil, nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (m *MockUserAPIKeyDB) UpdateLastUsed(ctx context.Context, keyID string) error {
	m.lastUsedIDs = append(m.lastUsedIDs, keyID)
	if key, ok := m.keys[keyID]; ok {
		now := time.Now()
		key.LastUsedAt = &now
	}
	return nil
}

// WasLastUsedUpdated checks if UpdateLastUsed was called with the given ID.
func (m *MockUserAPIKeyDB) WasLastUsedUpdated(keyID string) bool {
	for _, id := range m.lastUsedIDs {
		if id == keyID {
			return true
		}
	}
	return false
}

// RevokeKey simulates revoking a key.
func (m *MockUserAPIKeyDB) RevokeKey(keyID string) {
	if key, ok := m.keys[keyID]; ok {
		now := time.Now()
		key.RevokedAt = &now
	}
}

func TestUserAPIKeyMiddleware(t *testing.T) {
	// Create mock database with test user and API key
	db := NewMockUserAPIKeyDB()
	testUserID := "user-123"
	testKeyID := "key-456"
	testKey := "solvr_sk_testkey123456789012345678901234567890"
	db.AddTestUser(testUserID, "testuser", "test@example.com")
	_, err := db.AddTestUserAPIKey(testKeyID, testUserID, "Test Key", testKey)
	if err != nil {
		t.Fatalf("failed to add test API key: %v", err)
	}

	validator := NewUserAPIKeyValidator(db)

	tests := []struct {
		name           string
		authHeader     string
		wantStatusCode int
		wantClaims     bool
	}{
		{
			name:           "valid user API key",
			authHeader:     "Bearer " + testKey,
			wantStatusCode: http.StatusOK,
			wantClaims:     true,
		},
		{
			name:           "missing authorization header",
			authHeader:     "",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "invalid user API key",
			authHeader:     "Bearer solvr_sk_invalidkey1234567890123456789012",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "agent API key format (should fail)",
			authHeader:     "Bearer solvr_testkey123456789012345678901234567890",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "JWT format (should fail)",
			authHeader:     "Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VyX2lkIjoiMTIzIn0.abcd",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "missing Bearer prefix",
			authHeader:     testKey,
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
		{
			name:           "empty token after Bearer",
			authHeader:     "Bearer ",
			wantStatusCode: http.StatusUnauthorized,
			wantClaims:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var gotClaims *Claims
			nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotClaims = ClaimsFromContext(r.Context())
				w.WriteHeader(http.StatusOK)
			})

			middleware := UserAPIKeyMiddleware(validator)(nextHandler)

			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}

			rr := httptest.NewRecorder()
			middleware.ServeHTTP(rr, req)

			if rr.Code != tt.wantStatusCode {
				t.Errorf("status code = %v, want %v", rr.Code, tt.wantStatusCode)
			}

			if tt.wantClaims && gotClaims == nil {
				t.Error("expected claims in context but got nil")
			}
			if !tt.wantClaims && gotClaims != nil {
				t.Error("expected no claims but got some")
			}

			// Verify claims data if expected
			if tt.wantClaims && gotClaims != nil {
				if gotClaims.UserID != testUserID {
					t.Errorf("claims.UserID = %v, want %v", gotClaims.UserID, testUserID)
				}
			}
		})
	}
}

func TestUserAPIKeyMiddleware_UpdatesLastUsed(t *testing.T) {
	db := NewMockUserAPIKeyDB()
	testUserID := "user-123"
	testKeyID := "key-456"
	testKey := "solvr_sk_testkey123456789012345678901234567890"
	db.AddTestUser(testUserID, "testuser", "test@example.com")
	_, err := db.AddTestUserAPIKey(testKeyID, testUserID, "Test Key", testKey)
	if err != nil {
		t.Fatalf("failed to add test API key: %v", err)
	}

	validator := NewUserAPIKeyValidator(db)

	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserAPIKeyMiddleware(validator)(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+testKey)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("status code = %v, want %v", rr.Code, http.StatusOK)
	}

	// Verify last_used_at was updated
	if !db.WasLastUsedUpdated(testKeyID) {
		t.Error("expected UpdateLastUsed to be called but it wasn't")
	}
}

func TestUserAPIKeyMiddleware_RejectsRevokedKey(t *testing.T) {
	db := NewMockUserAPIKeyDB()
	testUserID := "user-123"
	testKeyID := "key-456"
	testKey := "solvr_sk_testkey123456789012345678901234567890"
	db.AddTestUser(testUserID, "testuser", "test@example.com")
	_, err := db.AddTestUserAPIKey(testKeyID, testUserID, "Test Key", testKey)
	if err != nil {
		t.Fatalf("failed to add test API key: %v", err)
	}

	// Revoke the key
	db.RevokeKey(testKeyID)

	validator := NewUserAPIKeyValidator(db)

	var gotClaims *Claims
	nextHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotClaims = ClaimsFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	})

	middleware := UserAPIKeyMiddleware(validator)(nextHandler)

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	req.Header.Set("Authorization", "Bearer "+testKey)

	rr := httptest.NewRecorder()
	middleware.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("status code = %v, want %v", rr.Code, http.StatusUnauthorized)
	}

	if gotClaims != nil {
		t.Error("expected no claims for revoked key but got some")
	}
}

func TestIsUserAPIKey(t *testing.T) {
	tests := []struct {
		name string
		key  string
		want bool
	}{
		{
			name: "valid user API key prefix",
			key:  "solvr_sk_abc123",
			want: true,
		},
		{
			name: "agent API key (no sk prefix)",
			key:  "solvr_abc123",
			want: false,
		},
		{
			name: "JWT format",
			key:  "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.abc.xyz",
			want: false,
		},
		{
			name: "empty string",
			key:  "",
			want: false,
		},
		{
			name: "random string",
			key:  "random_key_123",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUserAPIKey(tt.key)
			if got != tt.want {
				t.Errorf("IsUserAPIKey(%q) = %v, want %v", tt.key, got, tt.want)
			}
		})
	}
}

func TestUserAPIKeyValidator_ValidateUserAPIKey(t *testing.T) {
	db := NewMockUserAPIKeyDB()
	testUserID := "user-123"
	testKeyID := "key-456"
	testKey := "solvr_sk_testkey123456789012345678901234567890"
	db.AddTestUser(testUserID, "testuser", "test@example.com")
	_, err := db.AddTestUserAPIKey(testKeyID, testUserID, "Test Key", testKey)
	if err != nil {
		t.Fatalf("failed to add test API key: %v", err)
	}

	validator := NewUserAPIKeyValidator(db)

	t.Run("valid key returns user and key", func(t *testing.T) {
		user, key, err := validator.ValidateUserAPIKey(context.Background(), testKey)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if user == nil {
			t.Fatal("expected user but got nil")
		}
		if key == nil {
			t.Fatal("expected key but got nil")
		}
		if user.ID != testUserID {
			t.Errorf("user.ID = %v, want %v", user.ID, testUserID)
		}
		if key.ID != testKeyID {
			t.Errorf("key.ID = %v, want %v", key.ID, testKeyID)
		}
	})

	t.Run("empty key returns error", func(t *testing.T) {
		_, _, err := validator.ValidateUserAPIKey(context.Background(), "")
		if err == nil {
			t.Error("expected error for empty key but got nil")
		}
	})

	t.Run("invalid key format returns error", func(t *testing.T) {
		_, _, err := validator.ValidateUserAPIKey(context.Background(), "solvr_notsk")
		if err == nil {
			t.Error("expected error for invalid key format but got nil")
		}
	})

	t.Run("non-existent key returns error", func(t *testing.T) {
		_, _, err := validator.ValidateUserAPIKey(context.Background(), "solvr_sk_nonexistent123456789")
		if err == nil {
			t.Error("expected error for non-existent key but got nil")
		}
	})
}
