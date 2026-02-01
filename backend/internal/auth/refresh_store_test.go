package auth

import (
	"context"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MockDB is a mock database for testing refresh token storage.
// In actual usage, the real db.Pool would be used.
type MockDB struct {
	storedTokens map[string]RefreshTokenRecord
	shouldFail   bool
}

func NewMockDB() *MockDB {
	return &MockDB{
		storedTokens: make(map[string]RefreshTokenRecord),
	}
}

func (m *MockDB) StoreToken(ctx context.Context, record RefreshTokenRecord) error {
	if m.shouldFail {
		return &AuthError{Code: "DB_ERROR", Message: "database error"}
	}
	m.storedTokens[record.TokenHash] = record
	return nil
}

func (m *MockDB) GetByTokenHash(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error) {
	if m.shouldFail {
		return nil, &AuthError{Code: "DB_ERROR", Message: "database error"}
	}
	record, ok := m.storedTokens[tokenHash]
	if !ok {
		return nil, nil
	}
	return &record, nil
}

func (m *MockDB) DeleteByID(ctx context.Context, id string) error {
	if m.shouldFail {
		return &AuthError{Code: "DB_ERROR", Message: "database error"}
	}
	for hash, record := range m.storedTokens {
		if record.ID == id {
			delete(m.storedTokens, hash)
			return nil
		}
	}
	return nil
}

func (m *MockDB) DeleteByUserID(ctx context.Context, userID string) error {
	if m.shouldFail {
		return &AuthError{Code: "DB_ERROR", Message: "database error"}
	}
	for hash, record := range m.storedTokens {
		if record.UserID == userID {
			delete(m.storedTokens, hash)
		}
	}
	return nil
}

func (m *MockDB) DeleteExpired(ctx context.Context) (int64, error) {
	if m.shouldFail {
		return 0, &AuthError{Code: "DB_ERROR", Message: "database error"}
	}
	var count int64
	now := time.Now()
	for hash, record := range m.storedTokens {
		if record.ExpiresAt.Before(now) {
			delete(m.storedTokens, hash)
			count++
		}
	}
	return count, nil
}

func TestRefreshTokenStore_StoreToken(t *testing.T) {
	t.Run("successfully stores token", func(t *testing.T) {
		store := NewRefreshTokenStore(NewMockDB())
		ctx := context.Background()
		userID := uuid.New().String()
		token := GenerateRefreshToken()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		record, err := store.StoreToken(ctx, userID, token, expiresAt)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if record.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, record.UserID)
		}

		if record.TokenHash == "" {
			t.Error("expected token hash to be set")
		}

		if record.TokenHash == token {
			t.Error("token hash should be different from plain token")
		}

		if record.ID == "" {
			t.Error("expected ID to be set")
		}
	})

	t.Run("returns error for empty userID", func(t *testing.T) {
		store := NewRefreshTokenStore(NewMockDB())
		ctx := context.Background()
		token := GenerateRefreshToken()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		_, err := store.StoreToken(ctx, "", token, expiresAt)
		if err == nil {
			t.Error("expected error for empty userID")
		}
	})

	t.Run("returns error for empty token", func(t *testing.T) {
		store := NewRefreshTokenStore(NewMockDB())
		ctx := context.Background()
		userID := uuid.New().String()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		_, err := store.StoreToken(ctx, userID, "", expiresAt)
		if err == nil {
			t.Error("expected error for empty token")
		}
	})

	t.Run("returns error for past expiry", func(t *testing.T) {
		store := NewRefreshTokenStore(NewMockDB())
		ctx := context.Background()
		userID := uuid.New().String()
		token := GenerateRefreshToken()
		expiresAt := time.Now().Add(-1 * time.Hour) // Past

		_, err := store.StoreToken(ctx, userID, token, expiresAt)
		if err == nil {
			t.Error("expected error for past expiry")
		}
	})
}

func TestRefreshTokenStore_ValidateToken(t *testing.T) {
	t.Run("validates stored token", func(t *testing.T) {
		mockDB := NewMockDB()
		store := NewRefreshTokenStore(mockDB)
		ctx := context.Background()
		userID := uuid.New().String()
		token := GenerateRefreshToken()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		// Store the token first
		record, err := store.StoreToken(ctx, userID, token, expiresAt)
		if err != nil {
			t.Fatalf("failed to store token: %v", err)
		}

		// Validate it
		validated, err := store.ValidateToken(ctx, token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		if validated.ID != record.ID {
			t.Errorf("expected ID %s, got %s", record.ID, validated.ID)
		}

		if validated.UserID != userID {
			t.Errorf("expected userID %s, got %s", userID, validated.UserID)
		}
	})

	t.Run("returns error for invalid token", func(t *testing.T) {
		store := NewRefreshTokenStore(NewMockDB())
		ctx := context.Background()

		_, err := store.ValidateToken(ctx, "invalid_token")
		if err == nil {
			t.Error("expected error for invalid token")
		}
	})

	t.Run("returns error for empty token", func(t *testing.T) {
		store := NewRefreshTokenStore(NewMockDB())
		ctx := context.Background()

		_, err := store.ValidateToken(ctx, "")
		if err == nil {
			t.Error("expected error for empty token")
		}
	})

	t.Run("returns error for expired token", func(t *testing.T) {
		mockDB := NewMockDB()
		store := NewRefreshTokenStore(mockDB)
		ctx := context.Background()
		userID := uuid.New().String()
		token := GenerateRefreshToken()

		// Manually create an expired record
		hash, _ := HashRefreshToken(token)
		mockDB.storedTokens[hash] = RefreshTokenRecord{
			ID:        uuid.New().String(),
			UserID:    userID,
			TokenHash: hash,
			ExpiresAt: time.Now().Add(-1 * time.Hour), // Expired
			CreatedAt: time.Now().Add(-8 * 24 * time.Hour),
		}

		_, err := store.ValidateToken(ctx, token)
		if err == nil {
			t.Error("expected error for expired token")
		}
	})
}

func TestRefreshTokenStore_RevokeToken(t *testing.T) {
	t.Run("revokes token by ID", func(t *testing.T) {
		mockDB := NewMockDB()
		store := NewRefreshTokenStore(mockDB)
		ctx := context.Background()
		userID := uuid.New().String()
		token := GenerateRefreshToken()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		record, _ := store.StoreToken(ctx, userID, token, expiresAt)

		err := store.RevokeToken(ctx, record.ID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Try to validate - should fail
		_, err = store.ValidateToken(ctx, token)
		if err == nil {
			t.Error("expected error after revocation")
		}
	})
}

func TestRefreshTokenStore_RevokeAllUserTokens(t *testing.T) {
	t.Run("revokes all user tokens", func(t *testing.T) {
		mockDB := NewMockDB()
		store := NewRefreshTokenStore(mockDB)
		ctx := context.Background()
		userID := uuid.New().String()
		expiresAt := time.Now().Add(7 * 24 * time.Hour)

		// Store multiple tokens for the same user
		token1 := GenerateRefreshToken()
		token2 := GenerateRefreshToken()
		store.StoreToken(ctx, userID, token1, expiresAt)
		store.StoreToken(ctx, userID, token2, expiresAt)

		err := store.RevokeAllUserTokens(ctx, userID)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Both tokens should be invalid
		_, err1 := store.ValidateToken(ctx, token1)
		_, err2 := store.ValidateToken(ctx, token2)
		if err1 == nil || err2 == nil {
			t.Error("expected both tokens to be invalid after revoke all")
		}
	})
}
