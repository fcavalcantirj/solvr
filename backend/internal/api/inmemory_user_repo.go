// Package api provides HTTP routing and handlers for the Solvr API.
package api

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
)

// InMemoryNotificationsRepository is an in-memory implementation of NotificationsRepositoryInterface.
// Used for testing when no database is available.
type InMemoryNotificationsRepository struct {
	mu            sync.RWMutex
	notifications map[string]*handlers.Notification
}

// NewInMemoryNotificationsRepository creates a new in-memory notifications repository.
func NewInMemoryNotificationsRepository() *InMemoryNotificationsRepository {
	return &InMemoryNotificationsRepository{
		notifications: make(map[string]*handlers.Notification),
	}
}

// GetNotificationsForUser returns notifications for a user.
// Returns paginated notifications ordered by created_at DESC.
func (r *InMemoryNotificationsRepository) GetNotificationsForUser(ctx context.Context, userID string, page, perPage int) ([]handlers.Notification, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userNotifications []handlers.Notification
	for _, n := range r.notifications {
		if n.UserID != nil && *n.UserID == userID {
			userNotifications = append(userNotifications, *n)
		}
	}

	total := len(userNotifications)

	// Apply pagination
	start := (page - 1) * perPage
	if start > len(userNotifications) {
		return []handlers.Notification{}, total, nil
	}
	end := start + perPage
	if end > len(userNotifications) {
		end = len(userNotifications)
	}

	return userNotifications[start:end], total, nil
}

// GetNotificationsForAgent returns notifications for an agent.
func (r *InMemoryNotificationsRepository) GetNotificationsForAgent(ctx context.Context, agentID string, page, perPage int) ([]handlers.Notification, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var agentNotifications []handlers.Notification
	for _, n := range r.notifications {
		if n.AgentID != nil && *n.AgentID == agentID {
			agentNotifications = append(agentNotifications, *n)
		}
	}

	total := len(agentNotifications)

	// Apply pagination
	start := (page - 1) * perPage
	if start > len(agentNotifications) {
		return []handlers.Notification{}, total, nil
	}
	end := start + perPage
	if end > len(agentNotifications) {
		end = len(agentNotifications)
	}

	return agentNotifications[start:end], total, nil
}

// MarkRead marks a notification as read.
func (r *InMemoryNotificationsRepository) MarkRead(ctx context.Context, id string) (*handlers.Notification, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	n, exists := r.notifications[id]
	if !exists {
		return nil, handlers.ErrNotificationNotFound
	}

	now := time.Now()
	n.ReadAt = &now
	return n, nil
}

// MarkAllReadForUser marks all unread notifications as read for a user.
func (r *InMemoryNotificationsRepository) MarkAllReadForUser(ctx context.Context, userID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	now := time.Now()
	for _, n := range r.notifications {
		if n.UserID != nil && *n.UserID == userID && n.ReadAt == nil {
			n.ReadAt = &now
			count++
		}
	}
	return count, nil
}

// MarkAllReadForAgent marks all unread notifications as read for an agent.
func (r *InMemoryNotificationsRepository) MarkAllReadForAgent(ctx context.Context, agentID string) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	count := 0
	now := time.Now()
	for _, n := range r.notifications {
		if n.AgentID != nil && *n.AgentID == agentID && n.ReadAt == nil {
			n.ReadAt = &now
			count++
		}
	}
	return count, nil
}

// FindByID finds a notification by ID.
func (r *InMemoryNotificationsRepository) FindByID(ctx context.Context, id string) (*handlers.Notification, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	n, exists := r.notifications[id]
	if !exists {
		return nil, handlers.ErrNotificationNotFound
	}
	return n, nil
}

// InMemoryUserAPIKeyRepository is an in-memory implementation of UserAPIKeyRepositoryInterface.
// Used for testing when no database is available.
type InMemoryUserAPIKeyRepository struct {
	mu   sync.RWMutex
	keys map[string]*models.UserAPIKey
}

// NewInMemoryUserAPIKeyRepository creates a new in-memory user API key repository.
func NewInMemoryUserAPIKeyRepository() *InMemoryUserAPIKeyRepository {
	return &InMemoryUserAPIKeyRepository{
		keys: make(map[string]*models.UserAPIKey),
	}
}

// errUserAPIKeyNotFound is returned when an API key is not found.
var errUserAPIKeyNotFound = errors.New("API key not found")

// Create inserts a new API key for a user.
func (r *InMemoryUserAPIKeyRepository) Create(ctx context.Context, key *models.UserAPIKey) (*models.UserAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Generate ID if not set
	if key.ID == "" {
		key.ID = uuid.New().String()
	}

	now := time.Now()
	if key.CreatedAt.IsZero() {
		key.CreatedAt = now
	}
	key.UpdatedAt = now

	keyCopy := *key
	r.keys[key.ID] = &keyCopy
	return &keyCopy, nil
}

// FindByUserID returns all active API keys for a user.
func (r *InMemoryUserAPIKeyRepository) FindByUserID(ctx context.Context, userID string) ([]*models.UserAPIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var userKeys []*models.UserAPIKey
	for _, key := range r.keys {
		if key.UserID == userID && key.RevokedAt == nil {
			keyCopy := *key
			userKeys = append(userKeys, &keyCopy)
		}
	}

	if userKeys == nil {
		userKeys = []*models.UserAPIKey{}
	}
	return userKeys, nil
}

// FindByID finds a single API key by its ID.
func (r *InMemoryUserAPIKeyRepository) FindByID(ctx context.Context, id string) (*models.UserAPIKey, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	key, exists := r.keys[id]
	if !exists {
		return nil, errUserAPIKeyNotFound
	}

	keyCopy := *key
	return &keyCopy, nil
}

// Revoke soft-deletes an API key.
func (r *InMemoryUserAPIKeyRepository) Revoke(ctx context.Context, id, userID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key, exists := r.keys[id]
	if !exists || key.UserID != userID || key.RevokedAt != nil {
		return errUserAPIKeyNotFound
	}

	now := time.Now()
	key.RevokedAt = &now
	key.UpdatedAt = now
	return nil
}

// UpdateLastUsed updates the last_used_at timestamp.
func (r *InMemoryUserAPIKeyRepository) UpdateLastUsed(ctx context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key, exists := r.keys[id]
	if !exists {
		return errUserAPIKeyNotFound
	}

	now := time.Now()
	key.LastUsedAt = &now
	key.UpdatedAt = now
	return nil
}

// Regenerate updates an API key with a new hash value.
func (r *InMemoryUserAPIKeyRepository) Regenerate(ctx context.Context, id, userID, newKeyHash string) (*models.UserAPIKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	key, exists := r.keys[id]
	if !exists || key.UserID != userID || key.RevokedAt != nil {
		return nil, errUserAPIKeyNotFound
	}

	key.KeyHash = newKeyHash
	key.UpdatedAt = time.Now()

	keyCopy := *key
	return &keyCopy, nil
}
