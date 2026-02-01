package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	// RefreshTokenRandomBytes is the number of random bytes for refresh token generation.
	// 64 bytes provides 512 bits of entropy, which is highly secure.
	RefreshTokenRandomBytes = 64
)

// GenerateRefreshToken creates a new refresh token.
// The token is 64 random bytes, URL-safe base64 encoded.
// Per SPEC.md Part 5.2: "Refresh token: opaque, 7 days expiry"
func GenerateRefreshToken() string {
	randomBytes := make([]byte, RefreshTokenRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		// This should never happen in practice, but panic if crypto/rand fails
		// as this indicates a serious system issue.
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}

	// Use URL-safe base64 encoding (no + or /, uses - and _ instead)
	return base64.RawURLEncoding.EncodeToString(randomBytes)
}

// HashRefreshToken hashes a refresh token using SHA-256 for storage lookup.
// SHA-256 is used instead of bcrypt because we need deterministic hashes
// for database lookups. The token itself is already high-entropy (64 bytes).
func HashRefreshToken(token string) (string, error) {
	if token == "" {
		return "", fmt.Errorf("token cannot be empty")
	}

	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:]), nil
}

// CompareRefreshToken compares a plaintext refresh token with a hashed token.
// Returns nil if they match, or an error if they don't.
func CompareRefreshToken(token, hash string) error {
	if token == "" {
		return fmt.Errorf("token cannot be empty")
	}
	if hash == "" {
		return fmt.Errorf("hash cannot be empty")
	}

	computedHash, err := HashRefreshToken(token)
	if err != nil {
		return err
	}

	if computedHash != hash {
		return fmt.Errorf("token does not match")
	}

	return nil
}

// RefreshTokenRecord represents a refresh token stored in the database.
type RefreshTokenRecord struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"token_hash"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// RefreshTokenDB defines the interface for refresh token storage operations.
// This allows for easy mocking in tests.
type RefreshTokenDB interface {
	StoreToken(ctx context.Context, record RefreshTokenRecord) error
	GetByTokenHash(ctx context.Context, tokenHash string) (*RefreshTokenRecord, error)
	DeleteByID(ctx context.Context, id string) error
	DeleteByUserID(ctx context.Context, userID string) error
	DeleteExpired(ctx context.Context) (int64, error)
}

// RefreshTokenStore handles refresh token storage and validation.
type RefreshTokenStore struct {
	db RefreshTokenDB
}

// NewRefreshTokenStore creates a new RefreshTokenStore with the given database.
func NewRefreshTokenStore(db RefreshTokenDB) *RefreshTokenStore {
	return &RefreshTokenStore{db: db}
}

// StoreToken hashes and stores a refresh token in the database.
// Returns the created record with ID set, or an error.
// Per SPEC.md Part 5.2: Hash token before storing.
func (s *RefreshTokenStore) StoreToken(ctx context.Context, userID, token string, expiresAt time.Time) (*RefreshTokenRecord, error) {
	if userID == "" {
		return nil, NewAuthError(ErrCodeUnauthorized, "userID is required")
	}
	if token == "" {
		return nil, NewAuthError(ErrCodeInvalidToken, "token is required")
	}
	if expiresAt.Before(time.Now()) {
		return nil, NewAuthError(ErrCodeInvalidToken, "expiry time must be in the future")
	}

	// Hash the token before storage (never store plain tokens)
	hash, err := HashRefreshToken(token)
	if err != nil {
		return nil, fmt.Errorf("failed to hash token: %w", err)
	}

	record := RefreshTokenRecord{
		ID:        uuid.New().String(),
		UserID:    userID,
		TokenHash: hash,
		ExpiresAt: expiresAt,
		CreatedAt: time.Now(),
	}

	if err := s.db.StoreToken(ctx, record); err != nil {
		return nil, fmt.Errorf("failed to store refresh token: %w", err)
	}

	return &record, nil
}

// ValidateToken validates a refresh token and returns its record if valid.
// Returns an error if the token is invalid, expired, or not found.
func (s *RefreshTokenStore) ValidateToken(ctx context.Context, token string) (*RefreshTokenRecord, error) {
	if token == "" {
		return nil, NewAuthError(ErrCodeInvalidToken, "token is required")
	}

	// Hash the incoming token to look it up
	hash, err := HashRefreshToken(token)
	if err != nil {
		return nil, NewAuthError(ErrCodeInvalidToken, "invalid token format")
	}

	record, err := s.db.GetByTokenHash(ctx, hash)
	if err != nil {
		return nil, fmt.Errorf("failed to lookup token: %w", err)
	}
	if record == nil {
		return nil, NewAuthError(ErrCodeInvalidToken, "token not found")
	}

	// Check if expired
	if record.ExpiresAt.Before(time.Now()) {
		return nil, NewAuthError(ErrCodeTokenExpired, "refresh token has expired")
	}

	return record, nil
}

// RevokeToken revokes a specific refresh token by its ID.
func (s *RefreshTokenStore) RevokeToken(ctx context.Context, tokenID string) error {
	if tokenID == "" {
		return NewAuthError(ErrCodeInvalidToken, "token ID is required")
	}

	return s.db.DeleteByID(ctx, tokenID)
}

// RevokeAllUserTokens revokes all refresh tokens for a user (logout all sessions).
func (s *RefreshTokenStore) RevokeAllUserTokens(ctx context.Context, userID string) error {
	if userID == "" {
		return NewAuthError(ErrCodeUnauthorized, "userID is required")
	}

	return s.db.DeleteByUserID(ctx, userID)
}

// CleanupExpired removes all expired refresh tokens from the database.
// Returns the number of tokens removed.
func (s *RefreshTokenStore) CleanupExpired(ctx context.Context) (int64, error) {
	return s.db.DeleteExpired(ctx)
}
