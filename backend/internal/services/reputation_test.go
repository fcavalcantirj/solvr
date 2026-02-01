package services

import (
	"context"
	"testing"
)

// MockReputationRepository is a mock implementation of ReputationRepository.
type MockReputationRepository struct {
	upvotesReceived   int
	downvotesReceived int
	answersAccepted   int
	err               error
}

func (m *MockReputationRepository) GetUpvotesReceived(ctx context.Context, entityType, entityID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.upvotesReceived, nil
}

func (m *MockReputationRepository) GetDownvotesReceived(ctx context.Context, entityType, entityID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.downvotesReceived, nil
}

func (m *MockReputationRepository) GetAcceptedAnswersCount(ctx context.Context, entityType, entityID string) (int, error) {
	if m.err != nil {
		return 0, m.err
	}
	return m.answersAccepted, nil
}

func TestNewReputationService(t *testing.T) {
	repo := &MockReputationRepository{}
	svc := NewReputationService(repo)

	if svc == nil {
		t.Fatal("expected non-nil service")
	}

	if svc.repo != repo {
		t.Error("expected repo to be set")
	}
}

func TestCalculateReputation_UpvotesOnly(t *testing.T) {
	// Per PRD: upvote +10 rep
	repo := &MockReputationRepository{
		upvotesReceived:   5,
		downvotesReceived: 0,
		answersAccepted:   0,
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "human", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 5 upvotes * 10 = 50
	expected := 50
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_DownvotesOnly(t *testing.T) {
	// Per PRD: downvote -2 rep
	repo := &MockReputationRepository{
		upvotesReceived:   0,
		downvotesReceived: 3,
		answersAccepted:   0,
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "human", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 3 downvotes * -2 = -6
	expected := -6
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_AcceptedAnswersOnly(t *testing.T) {
	// Per PRD: accepted answer +25 rep
	repo := &MockReputationRepository{
		upvotesReceived:   0,
		downvotesReceived: 0,
		answersAccepted:   4,
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "human", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 4 accepted answers * 25 = 100
	expected := 100
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_Combined(t *testing.T) {
	// Combined calculation: upvotes + accepted answers - downvotes
	repo := &MockReputationRepository{
		upvotesReceived:   10, // 10 * 10 = 100
		downvotesReceived: 5,  // 5 * -2 = -10
		answersAccepted:   2,  // 2 * 25 = 50
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "human", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 100 + 50 - 10 = 140
	expected := 140
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_ZeroActivity(t *testing.T) {
	// No activity = 0 reputation
	repo := &MockReputationRepository{
		upvotesReceived:   0,
		downvotesReceived: 0,
		answersAccepted:   0,
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "agent", "agent-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := 0
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_NegativeTotal(t *testing.T) {
	// More downvotes than upvotes can result in negative reputation
	repo := &MockReputationRepository{
		upvotesReceived:   1,   // 1 * 10 = 10
		downvotesReceived: 10,  // 10 * -2 = -20
		answersAccepted:   0,
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "human", "user-123")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 10 - 20 = -10
	expected := -10
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_AgentType(t *testing.T) {
	// Should work for both human and agent entity types
	repo := &MockReputationRepository{
		upvotesReceived:   3,  // 3 * 10 = 30
		downvotesReceived: 0,
		answersAccepted:   1,  // 1 * 25 = 25
	}
	svc := NewReputationService(repo)

	rep, err := svc.CalculateReputation(context.Background(), "agent", "claude_assistant")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 30 + 25 = 55
	expected := 55
	if rep != expected {
		t.Errorf("expected reputation %d, got %d", expected, rep)
	}
}

func TestCalculateReputation_RepositoryError(t *testing.T) {
	repo := &MockReputationRepository{
		err: context.DeadlineExceeded,
	}
	svc := NewReputationService(repo)

	_, err := svc.CalculateReputation(context.Background(), "human", "user-123")
	if err == nil {
		t.Error("expected error when repository fails")
	}
}

// Reputation constants tests
func TestReputationConstants(t *testing.T) {
	// Verify the reputation point values per PRD requirements
	if ReputationPerUpvote != 10 {
		t.Errorf("expected ReputationPerUpvote to be 10, got %d", ReputationPerUpvote)
	}

	if ReputationPerDownvote != -2 {
		t.Errorf("expected ReputationPerDownvote to be -2, got %d", ReputationPerDownvote)
	}

	if ReputationPerAcceptedAnswer != 25 {
		t.Errorf("expected ReputationPerAcceptedAnswer to be 25, got %d", ReputationPerAcceptedAnswer)
	}
}
