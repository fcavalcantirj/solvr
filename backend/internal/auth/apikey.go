package auth

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

const (
	// APIKeyPrefix is the prefix for all Solvr API keys.
	APIKeyPrefix = "solvr_"

	// APIKeyRandomBytes is the number of random bytes used for key generation.
	APIKeyRandomBytes = 32

	// bcryptCost is the cost factor for bcrypt hashing (per SPEC.md).
	bcryptCost = 10
)

// GenerateAPIKey creates a new API key with the solvr_ prefix.
// The key is 32 random bytes, URL-safe base64 encoded, prefixed with "solvr_".
func GenerateAPIKey() string {
	randomBytes := make([]byte, APIKeyRandomBytes)
	if _, err := rand.Read(randomBytes); err != nil {
		// This should never happen in practice, but fallback to a less random value
		// if crypto/rand fails (system entropy exhausted, etc.)
		panic(fmt.Sprintf("crypto/rand failed: %v", err))
	}

	// Use URL-safe base64 encoding (no + or /, uses - and _ instead)
	encoded := base64.RawURLEncoding.EncodeToString(randomBytes)

	return APIKeyPrefix + encoded
}

// HashAPIKey hashes an API key using bcrypt for secure storage.
// Returns the hashed key or an error.
func HashAPIKey(key string) (string, error) {
	if key == "" {
		return "", fmt.Errorf("key cannot be empty")
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(key), bcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash API key: %w", err)
	}

	return string(hash), nil
}

// CompareAPIKey compares a plaintext API key with a hashed key.
// Returns nil if they match, or an error if they don't.
func CompareAPIKey(key, hash string) error {
	if key == "" {
		return fmt.Errorf("key cannot be empty")
	}
	if hash == "" {
		return fmt.Errorf("hash cannot be empty")
	}

	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(key))
}
