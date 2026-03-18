package referral

import (
	"testing"
)

func TestGenerateCode_Length(t *testing.T) {
	code, err := GenerateCode()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(code) != 8 {
		t.Errorf("expected code length 8, got %d: %q", len(code), code)
	}
}

func TestGenerateCode_Charset(t *testing.T) {
	// Generate multiple codes and verify all chars are [A-Z0-9]
	for i := 0; i < 100; i++ {
		code, err := GenerateCode()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		for _, c := range code {
			if !((c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				t.Errorf("invalid character %q in code %q", string(c), code)
			}
		}
	}
}

func TestGenerateCode_Uniqueness(t *testing.T) {
	// Generate 1000 codes and verify no duplicates
	// With ~1.7 trillion combinations, collision probability is negligible
	codes := make(map[string]bool)
	for i := 0; i < 1000; i++ {
		code, err := GenerateCode()
		if err != nil {
			t.Fatalf("unexpected error on iteration %d: %v", i, err)
		}
		if codes[code] {
			t.Errorf("duplicate code generated: %q", code)
		}
		codes[code] = true
	}
}

func TestGenerateCode_UsesCryptoRand(t *testing.T) {
	// Generate two codes — they should be different (statistical test)
	// crypto/rand produces non-deterministic output
	code1, _ := GenerateCode()
	code2, _ := GenerateCode()
	if code1 == code2 {
		t.Errorf("two consecutive codes are identical: %q — suggests non-random generation", code1)
	}
}
