// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"sync"
)

// InMemoryViewsRepository is an in-memory implementation of ViewsRepositoryInterface.
// Used for testing when no database is available.
type InMemoryViewsRepository struct {
	mu         sync.RWMutex
	views      map[string]map[string]bool // postID -> set of viewer keys
	viewCounts map[string]int             // postID -> view count
}

// NewInMemoryViewsRepository creates a new in-memory views repository.
func NewInMemoryViewsRepository() *InMemoryViewsRepository {
	return &InMemoryViewsRepository{
		views:      make(map[string]map[string]bool),
		viewCounts: make(map[string]int),
	}
}

// makeViewKey creates a unique key for a viewer.
func makeViewKey(viewerType, viewerID string) string {
	return viewerType + ":" + viewerID
}

// RecordView records a view for a post and returns the updated view count.
// If the user has already viewed the post, it returns the current count without incrementing.
func (r *InMemoryViewsRepository) RecordView(ctx context.Context, postID, viewerType, viewerID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Initialize post views map if needed
	if r.views[postID] == nil {
		r.views[postID] = make(map[string]bool)
	}

	key := makeViewKey(viewerType, viewerID)

	// Check if this viewer has already viewed the post
	if !r.views[postID][key] {
		r.views[postID][key] = true
		r.viewCounts[postID]++
	}

	return r.viewCounts[postID], nil
}

// RecordAnonymousView records a view from an anonymous user.
func (r *InMemoryViewsRepository) RecordAnonymousView(ctx context.Context, postID, sessionID string) (int, error) {
	return r.RecordView(ctx, postID, "anonymous", sessionID)
}

// GetViewCount returns the view count for a post.
func (r *InMemoryViewsRepository) GetViewCount(ctx context.Context, postID string) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return r.viewCounts[postID], nil
}

// Ensure InMemoryViewsRepository implements the interface (compile-time check).
var _ = func() error {
	var _ interface {
		RecordView(ctx context.Context, postID, viewerType, viewerID string) (int, error)
		RecordAnonymousView(ctx context.Context, postID, sessionID string) (int, error)
		GetViewCount(ctx context.Context, postID string) (int, error)
	} = (*InMemoryViewsRepository)(nil)
	return nil
}()
