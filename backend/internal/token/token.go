package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

const tokenPrefix = "solvr_rm_"

// GenerateRoomToken creates a new opaque bearer token.
// Returns the plaintext token (to give to the user) and its SHA-256 hash (to store in DB).
func GenerateRoomToken() (plaintext string, hashHex string, err error) {
	b := make([]byte, 32) // 256 bits of entropy
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plaintext = tokenPrefix + base64.RawURLEncoding.EncodeToString(b)
	sum := sha256.Sum256([]byte(plaintext))
	hashHex = hex.EncodeToString(sum[:])
	return plaintext, hashHex, nil
}

// HashToken computes the SHA-256 hash of a plaintext token for DB lookup.
func HashToken(plaintext string) string {
	sum := sha256.Sum256([]byte(plaintext))
	return hex.EncodeToString(sum[:])
}

// VerifyToken compares a plaintext token against a stored hash using constant-time comparison.
func VerifyToken(plaintext, storedHash string) bool {
	computed := HashToken(plaintext)
	return subtle.ConstantTimeCompare([]byte(computed), []byte(storedHash)) == 1
}
