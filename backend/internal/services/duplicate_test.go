// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync"
	"testing"
	"time"

	"github.com/google/uuid"
)

// MockDuplicateStore implements DuplicateStore for testing.
type MockDuplicateStore struct {
	mu     sync.Mutex
	hashes map[string]DuplicateRecord
}

// NewMockDuplicateStore creates a new mock store.
func NewMockDuplicateStore() *MockDuplicateStore {
	return &MockDuplicateStore{
		hashes: make(map[string]DuplicateRecord),
	}
}

// Store stores a content hash.
func (m *MockDuplicateStore) Store(ctx context.Context, hash string, record DuplicateRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.hashes[hash] = record
	return nil
}

// Find looks up a content hash.
func (m *MockDuplicateStore) Find(ctx context.Context, hash string) (*DuplicateRecord, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if record, ok := m.hashes[hash]; ok {
		return &record, nil
	}
	return nil, nil
}

// Delete removes a hash.
func (m *MockDuplicateStore) Delete(ctx context.Context, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.hashes, hash)
	return nil
}

// CleanupOlderThan removes records older than the given time.
func (m *MockDuplicateStore) CleanupOlderThan(ctx context.Context, cutoff time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for hash, record := range m.hashes {
		if record.CreatedAt.Before(cutoff) {
			delete(m.hashes, hash)
		}
	}
	return nil
}

// --- HashContent Tests ---

func TestHashContent(t *testing.T) {
	title := "Test Title"
	description := "This is a test description."

	hash := HashContent(title, description)

	if hash == "" {
		t.Error("Expected non-empty hash")
	}

	// Hash should be hex-encoded SHA256 (64 chars)
	if len(hash) != 64 {
		t.Errorf("Expected 64 char hex hash, got %d chars", len(hash))
	}

	// Verify it's a valid hex string
	_, err := hex.DecodeString(hash)
	if err != nil {
		t.Errorf("Hash is not valid hex: %v", err)
	}
}

func TestHashContent_Deterministic(t *testing.T) {
	title := "Same Title"
	description := "Same description content here."

	hash1 := HashContent(title, description)
	hash2 := HashContent(title, description)

	if hash1 != hash2 {
		t.Errorf("Hash should be deterministic, got %s and %s", hash1, hash2)
	}
}

func TestHashContent_DifferentInputsDifferentHash(t *testing.T) {
	hash1 := HashContent("Title A", "Description A")
	hash2 := HashContent("Title B", "Description B")

	if hash1 == hash2 {
		t.Error("Different inputs should produce different hashes")
	}
}

func TestHashContent_EmptyInputs(t *testing.T) {
	hash := HashContent("", "")
	if hash == "" {
		t.Error("Should handle empty inputs")
	}
}

func TestHashContent_VerifyAlgorithm(t *testing.T) {
	title := "Test"
	description := "Description"

	// Manually compute expected hash
	combined := title + "|" + description
	h := sha256.Sum256([]byte(combined))
	expected := hex.EncodeToString(h[:])

	actual := HashContent(title, description)
	if actual != expected {
		t.Errorf("Expected hash %s, got %s", expected, actual)
	}
}

// --- DuplicateDetectionService Tests ---

func TestDuplicateService_CheckDuplicate_NoDuplicate(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	result, err := service.CheckDuplicate(ctx, "Unique Title", "Unique description")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for non-duplicate content")
	}
}

func TestDuplicateService_CheckDuplicate_Found(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	// First, register some content
	postID := uuid.New()
	title := "Duplicate Title"
	description := "Duplicate description content."
	now := time.Now()

	err := service.RegisterContent(ctx, postID, title, description, now)
	if err != nil {
		t.Fatalf("Unexpected error registering content: %v", err)
	}

	// Now check for duplicate
	result, err := service.CheckDuplicate(ctx, title, description)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("Expected duplicate record, got nil")
	}

	if result.PostID != postID {
		t.Errorf("Expected PostID %s, got %s", postID, result.PostID)
	}
}

func TestDuplicateService_CheckDuplicate_ExpiredNotReturned(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	// Register content with old timestamp (more than 24h ago)
	postID := uuid.New()
	title := "Old Title"
	description := "Old description."
	oldTime := time.Now().Add(-25 * time.Hour)

	err := service.RegisterContent(ctx, postID, title, description, oldTime)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check should NOT find duplicate (older than 24h)
	result, err := service.CheckDuplicate(ctx, title, description)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != nil {
		t.Error("Expected nil result for expired content, got duplicate record")
	}
}

func TestDuplicateService_RegisterContent(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	postID := uuid.New()
	title := "New Post Title"
	description := "New post description."
	now := time.Now()

	err := service.RegisterContent(ctx, postID, title, description, now)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Verify it was stored
	hash := HashContent(title, description)
	record, err := store.Find(ctx, hash)
	if err != nil {
		t.Fatalf("Unexpected error finding record: %v", err)
	}

	if record == nil {
		t.Fatal("Expected record to be stored")
	}

	if record.PostID != postID {
		t.Errorf("Expected PostID %s, got %s", postID, record.PostID)
	}
}

func TestDuplicateService_Cleanup(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	// Add an old record
	oldPostID := uuid.New()
	oldTime := time.Now().Add(-48 * time.Hour)
	err := service.RegisterContent(ctx, oldPostID, "Old", "Old description", oldTime)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Add a recent record
	newPostID := uuid.New()
	newTime := time.Now().Add(-1 * time.Hour)
	err = service.RegisterContent(ctx, newPostID, "New", "New description", newTime)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Run cleanup for records older than 24h
	cutoff := time.Now().Add(-24 * time.Hour)
	err = service.Cleanup(ctx, cutoff)
	if err != nil {
		t.Fatalf("Unexpected error during cleanup: %v", err)
	}

	// Old record should be gone
	oldHash := HashContent("Old", "Old description")
	oldRecord, _ := store.Find(ctx, oldHash)
	if oldRecord != nil {
		t.Error("Old record should have been cleaned up")
	}

	// New record should still exist
	newHash := HashContent("New", "New description")
	newRecord, _ := store.Find(ctx, newHash)
	if newRecord == nil {
		t.Error("New record should still exist")
	}
}

func TestDuplicateService_SimilarButNotDuplicate(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	// Register content
	postID := uuid.New()
	err := service.RegisterContent(ctx, postID, "Original Title", "Original description.", time.Now())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check with slightly different content
	result, err := service.CheckDuplicate(ctx, "Original Title", "Original description!") // added !
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != nil {
		t.Error("Slightly different content should not be considered duplicate")
	}
}

func TestDuplicateService_CaseSensitive(t *testing.T) {
	ctx := context.Background()
	store := NewMockDuplicateStore()
	service := NewDuplicateDetectionService(store)

	// Register content
	postID := uuid.New()
	err := service.RegisterContent(ctx, postID, "Title", "Description", time.Now())
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	// Check with different case - should NOT be duplicate (hash is case-sensitive)
	result, err := service.CheckDuplicate(ctx, "TITLE", "DESCRIPTION")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if result != nil {
		t.Error("Case-different content should not be considered duplicate (hash is case-sensitive)")
	}
}

// --- DuplicateRecord Tests ---

func TestDuplicateRecord_IsExpired(t *testing.T) {
	tests := []struct {
		name      string
		createdAt time.Time
		maxAge    time.Duration
		expected  bool
	}{
		{
			name:      "Fresh content not expired",
			createdAt: time.Now().Add(-1 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  false,
		},
		{
			name:      "Old content expired",
			createdAt: time.Now().Add(-25 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  true,
		},
		{
			name:      "Exactly at boundary",
			createdAt: time.Now().Add(-24 * time.Hour),
			maxAge:    24 * time.Hour,
			expected:  true, // At or after maxAge is considered expired
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			record := DuplicateRecord{
				PostID:    uuid.New(),
				CreatedAt: tt.createdAt,
			}

			result := record.IsExpired(tt.maxAge)
			if result != tt.expected {
				t.Errorf("Expected IsExpired=%v, got %v", tt.expected, result)
			}
		})
	}
}
