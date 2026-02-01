// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"time"

	"github.com/google/uuid"
)

// DefaultDuplicateMaxAge is the default maximum age for duplicate detection (24 hours).
const DefaultDuplicateMaxAge = 24 * time.Hour

// DuplicateRecord represents a stored content hash for duplicate detection.
type DuplicateRecord struct {
	PostID    uuid.UUID `json:"post_id"`
	Hash      string    `json:"hash"`
	CreatedAt time.Time `json:"created_at"`
}

// IsExpired returns true if the record is older than maxAge.
func (r *DuplicateRecord) IsExpired(maxAge time.Duration) bool {
	return time.Since(r.CreatedAt) >= maxAge
}

// DuplicateStore is an interface for storing and retrieving content hashes.
// Can be implemented with in-memory store, Redis, or database.
type DuplicateStore interface {
	// Store saves a content hash with its associated record.
	Store(ctx context.Context, hash string, record DuplicateRecord) error

	// Find looks up a content hash and returns the record if found.
	Find(ctx context.Context, hash string) (*DuplicateRecord, error)

	// Delete removes a hash from the store.
	Delete(ctx context.Context, hash string) error

	// CleanupOlderThan removes all records older than the given time.
	CleanupOlderThan(ctx context.Context, cutoff time.Time) error
}

// DuplicateDetectionService provides duplicate content detection functionality.
type DuplicateDetectionService struct {
	store  DuplicateStore
	maxAge time.Duration
}

// NewDuplicateDetectionService creates a new duplicate detection service.
func NewDuplicateDetectionService(store DuplicateStore) *DuplicateDetectionService {
	return &DuplicateDetectionService{
		store:  store,
		maxAge: DefaultDuplicateMaxAge,
	}
}

// NewDuplicateDetectionServiceWithMaxAge creates a service with custom max age.
func NewDuplicateDetectionServiceWithMaxAge(store DuplicateStore, maxAge time.Duration) *DuplicateDetectionService {
	return &DuplicateDetectionService{
		store:  store,
		maxAge: maxAge,
	}
}

// HashContent computes a SHA-256 hash of the combined title and description.
// The hash is used to detect duplicate content.
func HashContent(title, description string) string {
	combined := title + "|" + description
	h := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(h[:])
}

// CheckDuplicate checks if content with the same title and description
// has been posted within the last 24 hours.
// Returns the duplicate record if found and not expired, nil otherwise.
func (s *DuplicateDetectionService) CheckDuplicate(ctx context.Context, title, description string) (*DuplicateRecord, error) {
	hash := HashContent(title, description)

	record, err := s.store.Find(ctx, hash)
	if err != nil {
		return nil, err
	}

	if record == nil {
		return nil, nil
	}

	// Check if the record has expired
	if record.IsExpired(s.maxAge) {
		// Optionally clean up the expired record
		_ = s.store.Delete(ctx, hash)
		return nil, nil
	}

	return record, nil
}

// RegisterContent stores a content hash for future duplicate detection.
// This should be called after a post is successfully created.
func (s *DuplicateDetectionService) RegisterContent(ctx context.Context, postID uuid.UUID, title, description string, createdAt time.Time) error {
	hash := HashContent(title, description)
	record := DuplicateRecord{
		PostID:    postID,
		Hash:      hash,
		CreatedAt: createdAt,
	}
	return s.store.Store(ctx, hash, record)
}

// Cleanup removes all content hashes older than the cutoff time.
// This should be called periodically to free up memory/storage.
func (s *DuplicateDetectionService) Cleanup(ctx context.Context, cutoff time.Time) error {
	return s.store.CleanupOlderThan(ctx, cutoff)
}

// InMemoryDuplicateStore is a simple in-memory implementation of DuplicateStore.
// Suitable for single-instance deployments or testing.
type InMemoryDuplicateStore struct {
	records map[string]DuplicateRecord
}

// NewInMemoryDuplicateStore creates a new in-memory store.
func NewInMemoryDuplicateStore() *InMemoryDuplicateStore {
	return &InMemoryDuplicateStore{
		records: make(map[string]DuplicateRecord),
	}
}

// Store saves a content hash.
func (s *InMemoryDuplicateStore) Store(ctx context.Context, hash string, record DuplicateRecord) error {
	s.records[hash] = record
	return nil
}

// Find looks up a content hash.
func (s *InMemoryDuplicateStore) Find(ctx context.Context, hash string) (*DuplicateRecord, error) {
	if record, ok := s.records[hash]; ok {
		return &record, nil
	}
	return nil, nil
}

// Delete removes a hash.
func (s *InMemoryDuplicateStore) Delete(ctx context.Context, hash string) error {
	delete(s.records, hash)
	return nil
}

// CleanupOlderThan removes records older than the cutoff.
func (s *InMemoryDuplicateStore) CleanupOlderThan(ctx context.Context, cutoff time.Time) error {
	for hash, record := range s.records {
		if record.CreatedAt.Before(cutoff) {
			delete(s.records, hash)
		}
	}
	return nil
}
