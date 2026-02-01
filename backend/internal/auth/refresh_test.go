package auth

import (
	"testing"
)

func TestGenerateRefreshToken(t *testing.T) {
	t.Run("returns non-empty string", func(t *testing.T) {
		token := GenerateRefreshToken()
		if token == "" {
			t.Error("expected non-empty token")
		}
	})

	t.Run("generates unique tokens", func(t *testing.T) {
		tokens := make(map[string]bool)
		for i := 0; i < 100; i++ {
			token := GenerateRefreshToken()
			if tokens[token] {
				t.Errorf("duplicate token generated: %s", token)
			}
			tokens[token] = true
		}
	})

	t.Run("token is base64 encoded", func(t *testing.T) {
		token := GenerateRefreshToken()
		// Base64 encoded 64 bytes should be 86 characters (without padding)
		// with padding it would be 88
		if len(token) < 80 {
			t.Errorf("token too short, expected >= 80 chars for 64-byte base64, got %d", len(token))
		}
	})

	t.Run("token is URL-safe base64", func(t *testing.T) {
		token := GenerateRefreshToken()
		// URL-safe base64 doesn't contain + or /
		for _, c := range token {
			if c == '+' || c == '/' {
				t.Errorf("token contains non-URL-safe character: %c", c)
			}
		}
	})
}

func TestHashRefreshToken(t *testing.T) {
	t.Run("returns non-empty hash", func(t *testing.T) {
		token := GenerateRefreshToken()
		hash, err := HashRefreshToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash == "" {
			t.Error("expected non-empty hash")
		}
	})

	t.Run("returns error for empty token", func(t *testing.T) {
		_, err := HashRefreshToken("")
		if err == nil {
			t.Error("expected error for empty token")
		}
	})

	t.Run("hash is different from token", func(t *testing.T) {
		token := GenerateRefreshToken()
		hash, err := HashRefreshToken(token)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if hash == token {
			t.Error("hash should be different from token")
		}
	})

	t.Run("same token produces same hash (deterministic SHA-256)", func(t *testing.T) {
		token := GenerateRefreshToken()
		hash1, _ := HashRefreshToken(token)
		hash2, _ := HashRefreshToken(token)
		if hash1 != hash2 {
			t.Error("expected same hashes for same token with SHA-256")
		}
	})

	t.Run("hash is 64 characters (SHA-256 hex)", func(t *testing.T) {
		token := GenerateRefreshToken()
		hash, _ := HashRefreshToken(token)
		// SHA-256 produces 32 bytes = 64 hex characters
		if len(hash) != 64 {
			t.Errorf("expected 64 character hash, got %d", len(hash))
		}
	})
}

func TestCompareRefreshToken(t *testing.T) {
	t.Run("valid token matches hash", func(t *testing.T) {
		token := GenerateRefreshToken()
		hash, _ := HashRefreshToken(token)
		err := CompareRefreshToken(token, hash)
		if err != nil {
			t.Errorf("expected match, got error: %v", err)
		}
	})

	t.Run("invalid token does not match hash", func(t *testing.T) {
		token := GenerateRefreshToken()
		hash, _ := HashRefreshToken(token)
		err := CompareRefreshToken("wrong_token", hash)
		if err == nil {
			t.Error("expected error for wrong token")
		}
	})

	t.Run("returns error for empty token", func(t *testing.T) {
		hash, _ := HashRefreshToken(GenerateRefreshToken())
		err := CompareRefreshToken("", hash)
		if err == nil {
			t.Error("expected error for empty token")
		}
	})

	t.Run("returns error for empty hash", func(t *testing.T) {
		token := GenerateRefreshToken()
		err := CompareRefreshToken(token, "")
		if err == nil {
			t.Error("expected error for empty hash")
		}
	})
}
