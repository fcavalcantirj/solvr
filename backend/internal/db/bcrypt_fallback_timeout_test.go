package db

import (
	"context"
	"testing"
	"time"
)

// TestBcryptFallbackRespectsContextTimeout verifies that the bcrypt fallback
// functions return promptly when the context is already cancelled, instead of
// scanning all rows and burning CPU on bcrypt comparisons.
func TestBcryptFallbackRespectsContextTimeout(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	t.Run("agent fallback returns quickly on cancelled context", func(t *testing.T) {
		repo := NewAgentRepository(pool)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		start := time.Now()
		agent, err := repo.getAgentByKeyBcryptFallback(ctx, "solvr_fake_key", "fake_sha256")
		elapsed := time.Since(start)

		// Should complete almost instantly (well under 1 second).
		if elapsed > 2*time.Second {
			t.Fatalf("bcrypt fallback took %v, expected < 2s with cancelled context", elapsed)
		}

		// With a cancelled context we expect no match — either nil error or context error is fine.
		if agent != nil {
			t.Fatal("expected nil agent for cancelled context")
		}
		// err can be nil or context.Canceled — both are acceptable
		_ = err
	})

	t.Run("user key fallback returns quickly on cancelled context", func(t *testing.T) {
		repo := NewUserAPIKeyRepository(pool)

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately

		start := time.Now()
		user, key, err := repo.getUserByKeyBcryptFallback(ctx, "solvr_sk_fake_key", "fake_sha256")
		elapsed := time.Since(start)

		// Should complete almost instantly (well under 1 second).
		if elapsed > 2*time.Second {
			t.Fatalf("bcrypt fallback took %v, expected < 2s with cancelled context", elapsed)
		}

		if user != nil {
			t.Fatal("expected nil user for cancelled context")
		}
		if key != nil {
			t.Fatal("expected nil key for cancelled context")
		}
		// err can be nil or context.Canceled — both are acceptable
		_ = err
	})
}
