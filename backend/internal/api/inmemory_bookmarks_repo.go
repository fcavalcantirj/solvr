// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// InMemoryBookmarksRepository is an in-memory implementation of BookmarksRepositoryInterface.
// Used for testing when no database is available.
type InMemoryBookmarksRepository struct {
	mu        sync.RWMutex
	bookmarks map[string]*models.Bookmark
}

// NewInMemoryBookmarksRepository creates a new in-memory bookmarks repository.
func NewInMemoryBookmarksRepository() *InMemoryBookmarksRepository {
	return &InMemoryBookmarksRepository{
		bookmarks: make(map[string]*models.Bookmark),
	}
}

// makeKey creates a unique key for a bookmark.
func makeKey(userType, userID, postID string) string {
	return userType + ":" + userID + ":" + postID
}

// Add creates a new bookmark.
func (r *InMemoryBookmarksRepository) Add(ctx context.Context, userType, userID, postID string) (*models.Bookmark, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeKey(userType, userID, postID)
	if _, exists := r.bookmarks[key]; exists {
		return nil, db.ErrBookmarkExists
	}

	bookmark := &models.Bookmark{
		ID:        uuid.New().String(),
		UserType:  userType,
		UserID:    userID,
		PostID:    postID,
		CreatedAt: time.Now(),
	}

	r.bookmarks[key] = bookmark
	return bookmark, nil
}

// Remove deletes a bookmark.
func (r *InMemoryBookmarksRepository) Remove(ctx context.Context, userType, userID, postID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := makeKey(userType, userID, postID)
	if _, exists := r.bookmarks[key]; !exists {
		return db.ErrBookmarkNotFound
	}

	delete(r.bookmarks, key)
	return nil
}

// ListByUser returns all bookmarks for a user.
func (r *InMemoryBookmarksRepository) ListByUser(ctx context.Context, userType, userID string, page, perPage int) ([]models.BookmarkWithPost, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userBookmarks []models.BookmarkWithPost
	for _, b := range r.bookmarks {
		if b.UserType == userType && b.UserID == userID {
			bwp := models.BookmarkWithPost{
				Bookmark: *b,
				Post: models.PostWithAuthor{
					Post: models.Post{
						ID:    b.PostID,
						Title: "Mock Post Title",
					},
				},
			}
			userBookmarks = append(userBookmarks, bwp)
		}
	}

	total := len(userBookmarks)

	// Apply pagination
	start := (page - 1) * perPage
	if start > len(userBookmarks) {
		return []models.BookmarkWithPost{}, total, nil
	}
	end := start + perPage
	if end > len(userBookmarks) {
		end = len(userBookmarks)
	}

	return userBookmarks[start:end], total, nil
}

// IsBookmarked checks if a post is bookmarked by a user.
func (r *InMemoryBookmarksRepository) IsBookmarked(ctx context.Context, userType, userID, postID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key := makeKey(userType, userID, postID)
	_, exists := r.bookmarks[key]
	return exists, nil
}

// Ensure InMemoryBookmarksRepository implements the interface (compile-time check).
var _ = func() error {
	var _ interface {
		Add(ctx context.Context, userType, userID, postID string) (*models.Bookmark, error)
		Remove(ctx context.Context, userType, userID, postID string) error
		ListByUser(ctx context.Context, userType, userID string, page, perPage int) ([]models.BookmarkWithPost, int, error)
		IsBookmarked(ctx context.Context, userType, userID, postID string) (bool, error)
	} = (*InMemoryBookmarksRepository)(nil)
	return nil
}()
