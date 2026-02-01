// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"fmt"
)

// Reputation point values per PRD requirements:
// - Rep: upvote +10 - Add 10 rep per upvote received
// - Rep: accepted answer +25 - Add 25 rep when answer accepted
// - Rep: downvote -2 - Subtract 2 rep per downvote received
const (
	ReputationPerUpvote         = 10
	ReputationPerDownvote       = -2
	ReputationPerAcceptedAnswer = 25
)

// ReputationRepository defines database operations for reputation calculation.
type ReputationRepository interface {
	// GetUpvotesReceived returns the count of upvotes received by an entity.
	// entityType is "human" or "agent", entityID is the user/agent ID.
	GetUpvotesReceived(ctx context.Context, entityType, entityID string) (int, error)

	// GetDownvotesReceived returns the count of downvotes received by an entity.
	GetDownvotesReceived(ctx context.Context, entityType, entityID string) (int, error)

	// GetAcceptedAnswersCount returns the count of accepted answers by an entity.
	GetAcceptedAnswersCount(ctx context.Context, entityType, entityID string) (int, error)
}

// ReputationService handles reputation calculation for users and agents.
type ReputationService struct {
	repo ReputationRepository
}

// NewReputationService creates a new reputation service.
func NewReputationService(repo ReputationRepository) *ReputationService {
	return &ReputationService{
		repo: repo,
	}
}

// CalculateReputation calculates the total reputation for a user or agent.
// Per SPEC.md Part 10.3 and PRD requirements:
//   - Upvote: +10 rep per upvote received
//   - Accepted answer: +25 rep per accepted answer
//   - Downvote: -2 rep per downvote received
//
// Parameters:
//   - entityType: "human" or "agent"
//   - entityID: the user ID or agent ID
//
// Returns the calculated reputation score, which can be negative.
func (s *ReputationService) CalculateReputation(ctx context.Context, entityType, entityID string) (int, error) {
	// Get upvotes received
	upvotes, err := s.repo.GetUpvotesReceived(ctx, entityType, entityID)
	if err != nil {
		return 0, fmt.Errorf("failed to get upvotes received: %w", err)
	}

	// Get downvotes received
	downvotes, err := s.repo.GetDownvotesReceived(ctx, entityType, entityID)
	if err != nil {
		return 0, fmt.Errorf("failed to get downvotes received: %w", err)
	}

	// Get accepted answers count
	acceptedAnswers, err := s.repo.GetAcceptedAnswersCount(ctx, entityType, entityID)
	if err != nil {
		return 0, fmt.Errorf("failed to get accepted answers count: %w", err)
	}

	// Calculate total reputation
	// Per PRD:
	// - upvote +10
	// - accepted answer +25
	// - downvote -2
	reputation := (upvotes * ReputationPerUpvote) +
		(acceptedAnswers * ReputationPerAcceptedAnswer) +
		(downvotes * ReputationPerDownvote)

	return reputation, nil
}
