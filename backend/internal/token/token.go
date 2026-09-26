package token

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"fmt"
)

// agentRoomTokenPrefix marks per-agent room tokens (mission #3): opaque bearer tokens
// the BearerGuard resolves to a room AND the specific agent. The shared room token
// (solvr_rm_...) they replaced is retired (migration 000098).
const agentRoomTokenPrefix = "solvr_rt_"

// GenerateAgentRoomToken creates a new per-agent room token (solvr_rt_...).
// Returns the plaintext token (given once to the agent) and its SHA-256 hash (stored).
func GenerateAgentRoomToken() (plaintext string, hashHex string, err error) {
	return generatePrefixedToken(agentRoomTokenPrefix)
}

// loginCodePrefix marks the one-time code the OAuth callback hands to the browser instead of a
// JWT. Like a stream ticket it is none of the bearer prefixes and not a JWT, so it can never be
// replayed as a credential; it only ever works as the body of POST /v1/auth/oauth/exchange.
const loginCodePrefix = "solvr_lc_"

// GenerateLoginCode creates a one-time OAuth login code (solvr_lc_...).
// Returns the plaintext code (given once to the browser) and its SHA-256 hash (stored).
func GenerateLoginCode() (plaintext string, hashHex string, err error) {
	return generatePrefixedToken(loginCodePrefix)
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
