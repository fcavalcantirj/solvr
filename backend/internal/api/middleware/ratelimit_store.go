// Package middleware provides HTTP middleware for the Solvr API.
package middleware

import (
	"context"
	"sync"
	"time"
)

// InMemoryRateLimitStore implements RateLimitStore using an in-memory map.
// This is suitable for single-instance deployments. For multi-instance,
// use Redis or database-backed store.
type InMemoryRateLimitStore struct {
	mu      sync.RWMutex
	records map[string]*RateLimitRecord
}

// NewInMemoryRateLimitStore creates a new in-memory rate limit store.
func NewInMemoryRateLimitStore() *InMemoryRateLimitStore {
	store := &InMemoryRateLimitStore{
		records: make(map[string]*RateLimitRecord),
	}
	// Start cleanup goroutine to prevent memory leak
	go store.cleanup()
	return store
}

// GetRecord retrieves a rate limit record by key.
func (s *InMemoryRateLimitStore) GetRecord(ctx context.Context, key string) (*RateLimitRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	record, exists := s.records[key]
	if !exists {
		return nil, nil
	}
	return record, nil
}

// IncrementAndGet increments the count and returns the updated record.
// If the window has expired, it starts a new window.
func (s *InMemoryRateLimitStore) IncrementAndGet(ctx context.Context, key string, window time.Duration) (*RateLimitRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	record, exists := s.records[key]

	if !exists || now.Sub(record.WindowStart) >= window {
		// Start new window
		record = &RateLimitRecord{
			Key:         key,
			Count:       1,
			WindowStart: now,
		}
		s.records[key] = record
		return record, nil
	}

	// Increment existing window
	record.Count++
	return record, nil
}

// cleanup periodically removes expired records to prevent memory leak.
func (s *InMemoryRateLimitStore) cleanup() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		s.doCleanup()
	}
}

// doCleanup removes expired records using a two-phase approach to minimize lock contention.
// Phase 1: collect expired keys under a read lock (fast, doesn't block other readers).
// Phase 2: delete expired keys in small batches with brief write locks, allowing
// IncrementAndGet calls to interleave between batches.
func (s *InMemoryRateLimitStore) doCleanup() {
	// Phase 1: Snapshot expired keys under read lock
	s.mu.RLock()
	now := time.Now()
	expired := make([]string, 0)
	for key, record := range s.records {
		if now.Sub(record.WindowStart) > time.Hour {
			expired = append(expired, key)
		}
	}
	s.mu.RUnlock()

	if len(expired) == 0 {
		return
	}

	// Phase 2: Delete expired keys in small batches with brief write locks
	const batchSize = 50
	for i := 0; i < len(expired); i += batchSize {
		end := i + batchSize
		if end > len(expired) {
			end = len(expired)
		}
		s.mu.Lock()
		for _, key := range expired[i:end] {
			// Re-verify expiry (record may have been refreshed between phases)
			if record, exists := s.records[key]; exists {
				if time.Since(record.WindowStart) > time.Hour {
					delete(s.records, key)
				}
			}
		}
		s.mu.Unlock()
	}
}
