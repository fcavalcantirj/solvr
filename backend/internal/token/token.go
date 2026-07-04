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

// agentRoomTokenPrefix distinguishes per-agent room tokens (mission #3) from the
// shared room token. Both are opaque bearer tokens the BearerGuard resolves to a room;
// a per-agent token additionally identifies the specific agent.
const agentRoomTokenPrefix = "solvr_rt_"

// GenerateRoomToken creates a new opaque bearer token.
// Returns the plaintext token (to give to the user) and its SHA-256 hash (to store in DB).
func GenerateRoomToken() (plaintext string, hashHex string, err error) {
	return generatePrefixedToken(tokenPrefix)
}

// GenerateAgentRoomToken creates a new per-agent room token (solvr_rt_...).
// Returns the plaintext token (given once to the agent) and its SHA-256 hash (stored).
func GenerateAgentRoomToken() (plaintext string, hashHex string, err error) {
	return generatePrefixedToken(agentRoomTokenPrefix)
}

// IsAgentRoomToken reports whether a plaintext token is a per-agent room token.
func IsAgentRoomToken(plaintext string) bool {
	return len(plaintext) > len(agentRoomTokenPrefix) && plaintext[:len(agentRoomTokenPrefix)] == agentRoomTokenPrefix
}

func generatePrefixedToken(prefix string) (plaintext string, hashHex string, err error) {
	b := make([]byte, 32) // 256 bits of entropy
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("generate token: %w", err)
	}
	plaintext = prefix + base64.RawURLEncoding.EncodeToString(b)
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
