package auth

import (
	"strings"
	"testing"
)

func TestGenerateAPIKey(t *testing.T) {
	key := GenerateAPIKey()

	// Should start with "solvr_" prefix
	if !strings.HasPrefix(key, "solvr_") {
		t.Errorf("GenerateAPIKey() = %v, want prefix 'solvr_'", key)
	}

	// Should be longer than just the prefix (at least 32 bytes base64 encoded)
	// solvr_ (6) + base64(32 bytes) = 6 + 43 = 49 chars minimum
	if len(key) < 49 {
		t.Errorf("GenerateAPIKey() length = %d, want at least 49", len(key))
	}

	// Two keys should be different
	key2 := GenerateAPIKey()
	if key == key2 {
		t.Error("GenerateAPIKey() produced duplicate keys")
	}
}

func TestGenerateAPIKeyUniqueness(t *testing.T) {
	// Generate 100 keys and ensure they're all unique
	keys := make(map[string]bool)
	for i := 0; i < 100; i++ {
		key := GenerateAPIKey()
		if keys[key] {
			t.Errorf("GenerateAPIKey() produced duplicate key: %v", key)
		}
		keys[key] = true
	}
}

func TestHashAPIKey(t *testing.T) {
	key := "solvr_test123456789abcdefghijklmnopqrstuvwxyz"

	hash, err := HashAPIKey(key)
	if err != nil {
		t.Fatalf("HashAPIKey() error = %v", err)
	}

	// Hash should not be empty
	if hash == "" {
		t.Error("HashAPIKey() returned empty hash")
	}

	// Hash should not equal the original key
	if hash == key {
		t.Error("HashAPIKey() returned the original key")
	}

	// Hash should be consistent with bcrypt format (starts with $2a$ or $2b$)
	if !strings.HasPrefix(hash, "$2a$") && !strings.HasPrefix(hash, "$2b$") {
		t.Errorf("HashAPIKey() hash doesn't look like bcrypt: %v", hash)
	}
}

func TestCompareAPIKey(t *testing.T) {
	key := "solvr_test123456789abcdefghijklmnopqrstuvwxyz"

	hash, err := HashAPIKey(key)
	if err != nil {
		t.Fatalf("HashAPIKey() error = %v", err)
	}

	tests := []struct {
		name    string
		key     string
		hash    string
		wantErr bool
	}{
		{
			name:    "valid key matches hash",
			key:     key,
			hash:    hash,
			wantErr: false,
		},
		{
			name:    "wrong key doesn't match",
			key:     "solvr_wrongkey123456789",
			hash:    hash,
			wantErr: true,
		},
		{
			name:    "empty key fails",
			key:     "",
			hash:    hash,
			wantErr: true,
		},
		{
			name:    "empty hash fails",
			key:     key,
			hash:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CompareAPIKey(tt.key, tt.hash)
			if (err != nil) != tt.wantErr {
				t.Errorf("CompareAPIKey() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestAPIKeyFormat(t *testing.T) {
	key := GenerateAPIKey()

	// Verify the format is solvr_{base64}
	parts := strings.SplitN(key, "_", 2)
	if len(parts) != 2 {
		t.Errorf("GenerateAPIKey() format invalid: %v", key)
		return
	}

	if parts[0] != "solvr" {
		t.Errorf("GenerateAPIKey() prefix = %v, want 'solvr'", parts[0])
	}

	// The random part should be URL-safe base64 (no + or /)
	if strings.ContainsAny(parts[1], "+/") {
		t.Error("GenerateAPIKey() should use URL-safe base64 encoding")
	}
}
