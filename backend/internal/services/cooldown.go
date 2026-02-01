// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"fmt"
	"math"
	"time"
)

// ContentType represents the type of content for cooldown purposes.
type ContentType string

// Content types with their associated cooldowns per SPEC.md Part 8.3.
const (
	ContentTypeProblem  ContentType = "problem"
	ContentTypeQuestion ContentType = "question"
	ContentTypeIdea     ContentType = "idea"
	ContentTypeAnswer   ContentType = "answer"
	ContentTypeComment  ContentType = "comment"
)

// CooldownConfig holds the cooldown durations for each content type.
// Per SPEC.md Part 8.3: "Cooldown Periods"
type CooldownConfig struct {
	Problem  time.Duration // 10 minutes
	Question time.Duration // 5 minutes
	Idea     time.Duration // 5 minutes
	Answer   time.Duration // 2 minutes
	Comment  time.Duration // 30 seconds
}

// DefaultCooldownConfig returns the default cooldown configuration per SPEC.md.
func DefaultCooldownConfig() CooldownConfig {
	return CooldownConfig{
		Problem:  10 * time.Minute,
		Question: 5 * time.Minute,
		Idea:     5 * time.Minute,
		Answer:   2 * time.Minute,
		Comment:  30 * time.Second,
	}
}

// CooldownResult contains information about an active cooldown.
type CooldownResult struct {
	ContentType ContentType   `json:"content_type"`
	Remaining   time.Duration `json:"remaining"`
	ResetAt     time.Time     `json:"reset_at"`
}

// RetryAfterSeconds returns the number of seconds to wait before retrying.
// Always rounds up to ensure the cooldown has fully expired.
func (r *CooldownResult) RetryAfterSeconds() int {
	seconds := r.Remaining.Seconds()
	return int(math.Ceil(seconds))
}

// CooldownStore is an interface for storing and retrieving last post times.
// Can be implemented with in-memory store, Redis, or database.
type CooldownStore interface {
	// GetLastPostTime returns the last post time for a user/content type key.
	GetLastPostTime(ctx context.Context, key string) (*time.Time, error)

	// SetLastPostTime sets the last post time for a user/content type key.
	SetLastPostTime(ctx context.Context, key string, t time.Time) error
}

// CooldownService provides cooldown enforcement functionality.
// Prevents rapid-fire low-quality content per SPEC.md Part 8.3.
type CooldownService struct {
	store  CooldownStore
	config CooldownConfig
}

// NewCooldownService creates a new cooldown service.
func NewCooldownService(store CooldownStore, config CooldownConfig) *CooldownService {
	return &CooldownService{
		store:  store,
		config: config,
	}
}

// CheckCooldown checks if a user is on cooldown for a specific content type.
// Returns a CooldownResult if on cooldown, nil otherwise.
func (s *CooldownService) CheckCooldown(ctx context.Context, userID string, contentType ContentType) (*CooldownResult, error) {
	key := s.buildKey(userID, contentType)

	lastTime, err := s.store.GetLastPostTime(ctx, key)
	if err != nil {
		return nil, err
	}

	// No previous post, no cooldown
	if lastTime == nil {
		return nil, nil
	}

	cooldownDuration := s.GetCooldownDuration(contentType)
	if cooldownDuration == 0 {
		// Unknown content type, no cooldown
		return nil, nil
	}

	elapsed := time.Since(*lastTime)
	if elapsed >= cooldownDuration {
		// Cooldown has expired
		return nil, nil
	}

	remaining := cooldownDuration - elapsed
	resetAt := lastTime.Add(cooldownDuration)

	return &CooldownResult{
		ContentType: contentType,
		Remaining:   remaining,
		ResetAt:     resetAt,
	}, nil
}

// RecordPost records a post time for a user/content type.
// This should be called after a post is successfully created.
func (s *CooldownService) RecordPost(ctx context.Context, userID string, contentType ContentType, postedAt time.Time) error {
	key := s.buildKey(userID, contentType)
	return s.store.SetLastPostTime(ctx, key, postedAt)
}

// GetCooldownDuration returns the cooldown duration for a content type.
func (s *CooldownService) GetCooldownDuration(contentType ContentType) time.Duration {
	switch contentType {
	case ContentTypeProblem:
		return s.config.Problem
	case ContentTypeQuestion:
		return s.config.Question
	case ContentTypeIdea:
		return s.config.Idea
	case ContentTypeAnswer:
		return s.config.Answer
	case ContentTypeComment:
		return s.config.Comment
	default:
		return 0
	}
}

// buildKey creates a unique key for storing cooldown state.
// Format: "cooldown:{userID}:{contentType}"
func (s *CooldownService) buildKey(userID string, contentType ContentType) string {
	return fmt.Sprintf("cooldown:%s:%s", userID, contentType)
}

// InMemoryCooldownStore is a simple in-memory implementation of CooldownStore.
// Suitable for single-instance deployments or testing.
type InMemoryCooldownStore struct {
	lastPosts map[string]time.Time
}

// NewInMemoryCooldownStore creates a new in-memory store.
func NewInMemoryCooldownStore() *InMemoryCooldownStore {
	return &InMemoryCooldownStore{
		lastPosts: make(map[string]time.Time),
	}
}

// GetLastPostTime returns the last post time for a key.
func (s *InMemoryCooldownStore) GetLastPostTime(ctx context.Context, key string) (*time.Time, error) {
	if t, ok := s.lastPosts[key]; ok {
		return &t, nil
	}
	return nil, nil
}

// SetLastPostTime sets the last post time for a key.
func (s *InMemoryCooldownStore) SetLastPostTime(ctx context.Context, key string, t time.Time) error {
	s.lastPosts[key] = t
	return nil
}
