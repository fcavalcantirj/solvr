package middleware

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestInMemoryRateLimitStore_IncrementAndGet(t *testing.T) {
	store := &InMemoryRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
	ctx := context.Background()
	window := time.Minute

	// First increment: count should be 1
	record, err := store.IncrementAndGet(ctx, "key1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Count != 1 {
		t.Errorf("expected count 1, got %d", record.Count)
	}
	if record.Key != "key1" {
		t.Errorf("expected key 'key1', got %q", record.Key)
	}

	// Second increment: count should be 2
	record, err = store.IncrementAndGet(ctx, "key1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Count != 2 {
		t.Errorf("expected count 2, got %d", record.Count)
	}

	// Different key: count should be 1
	record, err = store.IncrementAndGet(ctx, "key2", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Count != 1 {
		t.Errorf("expected count 1 for key2, got %d", record.Count)
	}
}

func TestInMemoryRateLimitStore_GetRecord(t *testing.T) {
	store := &InMemoryRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
	ctx := context.Background()

	// Non-existent key returns nil
	record, err := store.GetRecord(ctx, "missing")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record != nil {
		t.Errorf("expected nil for missing key, got %+v", record)
	}

	// After increment, GetRecord should return the record
	store.IncrementAndGet(ctx, "exists", time.Minute)
	record, err = store.GetRecord(ctx, "exists")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record == nil {
		t.Fatal("expected record, got nil")
	}
	if record.Count != 1 {
		t.Errorf("expected count 1, got %d", record.Count)
	}
	if record.Key != "exists" {
		t.Errorf("expected key 'exists', got %q", record.Key)
	}
}

func TestInMemoryRateLimitStore_WindowReset(t *testing.T) {
	store := &InMemoryRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
	ctx := context.Background()

	// Use a very short window
	window := 50 * time.Millisecond

	// Increment a few times
	store.IncrementAndGet(ctx, "key1", window)
	store.IncrementAndGet(ctx, "key1", window)
	record, _ := store.IncrementAndGet(ctx, "key1", window)
	if record.Count != 3 {
		t.Errorf("expected count 3, got %d", record.Count)
	}
	originalWindowStart := record.WindowStart

	// Wait for window to expire
	time.Sleep(60 * time.Millisecond)

	// Next increment should reset the window
	record, err := store.IncrementAndGet(ctx, "key1", window)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if record.Count != 1 {
		t.Errorf("expected count to reset to 1, got %d", record.Count)
	}
	if !record.WindowStart.After(originalWindowStart) {
		t.Error("expected window start to be updated after reset")
	}
}

func TestInMemoryRateLimitStore_CleanupDoesNotBlockRequests(t *testing.T) {
	store := &InMemoryRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
	ctx := context.Background()

	// Populate store with 2000 expired records
	expiredTime := time.Now().Add(-2 * time.Hour)
	store.mu.Lock()
	for i := 0; i < 2000; i++ {
		key := fmt.Sprintf("expired-%d", i)
		store.records[key] = &RateLimitRecord{
			Key:         key,
			Count:       5,
			WindowStart: expiredTime,
		}
	}
	store.mu.Unlock()

	// Start cleanup in a goroutine
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		store.doCleanup()
	}()

	// Simultaneously try to IncrementAndGet — should complete quickly
	done := make(chan struct{})
	go func() {
		// Try multiple increments to ensure we can interleave with cleanup
		for i := 0; i < 10; i++ {
			_, err := store.IncrementAndGet(ctx, "live-key", time.Minute)
			if err != nil {
				t.Errorf("IncrementAndGet failed: %v", err)
				break
			}
		}
		close(done)
	}()

	// IncrementAndGet must complete within 100ms (not blocked by cleanup)
	select {
	case <-done:
		// success
	case <-time.After(100 * time.Millisecond):
		t.Fatal("IncrementAndGet blocked for >100ms — cleanup is holding the lock too long")
	}

	wg.Wait()

	// Verify cleanup actually removed expired records
	store.mu.RLock()
	remaining := len(store.records)
	store.mu.RUnlock()

	// Only "live-key" should remain (expired records should be cleaned up)
	if remaining > 10 {
		t.Errorf("expected most expired records cleaned up, but %d remain", remaining)
	}
}
