// Package services provides business logic for the Solvr application.
package services

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock implementations ---

type mockInboxRepo struct {
	notifications []models.Notification
	totalUnread   int
	err           error
}

func (m *mockInboxRepo) GetRecentUnreadForAgent(_ context.Context, _ string, _ int) ([]models.Notification, int, error) {
	return m.notifications, m.totalUnread, m.err
}

type mockOpenItemsRepo struct {
	result *models.OpenItemsResult
	err    error
}

func (m *mockOpenItemsRepo) GetOpenItemsForAgent(_ context.Context, _ string) (*models.OpenItemsResult, error) {
	return m.result, m.err
}

type mockSuggestedActionsRepo struct {
	actions []models.SuggestedAction
	err     error
}

func (m *mockSuggestedActionsRepo) GetSuggestedActionsForAgent(_ context.Context, _ string) ([]models.SuggestedAction, error) {
	return m.actions, m.err
}

type mockOpportunitiesRepo struct {
	result *models.OpportunitiesSection
	err    error
}

func (m *mockOpportunitiesRepo) GetOpportunitiesForAgent(_ context.Context, _ string, _ []string, _ int) (*models.OpportunitiesSection, error) {
	return m.result, m.err
}

type mockReputationRepo struct {
	result *models.ReputationChangesResult
	err    error
}

func (m *mockReputationRepo) GetReputationChangesSince(_ context.Context, _ string, _ time.Time) (*models.ReputationChangesResult, error) {
	return m.result, m.err
}

type mockAgentBriefingRepo struct {
	calledWith string
	err        error
}

func (m *mockAgentBriefingRepo) UpdateLastBriefingAt(_ context.Context, id string) error {
	m.calledWith = id
	return m.err
}

// TestBriefingService_AllSections verifies that when all repos return data,
// the briefing response has all 5 sections populated.
func TestBriefingService_AllSections(t *testing.T) {
	now := time.Now()
	lastBriefing := now.Add(-4 * time.Hour)

	inboxRepo := &mockInboxRepo{
		notifications: []models.Notification{
			{ID: "n1", Type: "answer.created", Title: "New answer", Body: "Someone answered", Link: "/q/1", CreatedAt: now},
		},
		totalUnread: 3,
	}
	openItemsRepo := &mockOpenItemsRepo{
		result: &models.OpenItemsResult{
			ProblemsNoApproaches: 1,
			QuestionsNoAnswers:   2,
			ApproachesStale:      0,
			Items: []models.OpenItem{
				{Type: "problem", ID: "p1", Title: "Bug", Status: "open", AgeHours: 48},
			},
		},
	}
	suggestedActionsRepo := &mockSuggestedActionsRepo{
		actions: []models.SuggestedAction{
			{Action: "update_approach_status", TargetID: "a1", TargetTitle: "Fix bug", Reason: "Stale"},
		},
	}
	opportunitiesRepo := &mockOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 5,
			Items: []models.Opportunity{
				{ID: "opp1", Title: "Help needed", Tags: []string{"go"}, ApproachesCount: 0, PostedBy: "user1", AgeHours: 12},
			},
		},
	}
	reputationRepo := &mockReputationRepo{
		result: &models.ReputationChangesResult{
			SinceLastCheck: "+20",
			Breakdown: []models.ReputationEvent{
				{Reason: "post_upvoted", PostID: "p1", PostTitle: "Good post", Delta: 10},
				{Reason: "answer_accepted", PostID: "a1", PostTitle: "Great answer", Delta: 50},
			},
		},
	}
	agentRepo := &mockAgentBriefingRepo{}

	svc := NewBriefingService(
		inboxRepo,
		openItemsRepo,
		suggestedActionsRepo,
		opportunitiesRepo,
		reputationRepo,
		agentRepo,
	)

	agent := &models.Agent{
		ID:             "test-agent",
		Specialties:    []string{"go", "postgresql"},
		CreatedAt:      now.Add(-30 * 24 * time.Hour),
		LastBriefingAt: &lastBriefing,
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify inbox
	if briefing.Inbox == nil {
		t.Fatal("expected Inbox to be populated")
	}
	if briefing.Inbox.UnreadCount != 3 {
		t.Errorf("expected UnreadCount=3, got %d", briefing.Inbox.UnreadCount)
	}
	if len(briefing.Inbox.Items) != 1 {
		t.Errorf("expected 1 inbox item, got %d", len(briefing.Inbox.Items))
	}

	// Verify open items
	if briefing.MyOpenItems == nil {
		t.Fatal("expected MyOpenItems to be populated")
	}
	if briefing.MyOpenItems.ProblemsNoApproaches != 1 {
		t.Errorf("expected ProblemsNoApproaches=1, got %d", briefing.MyOpenItems.ProblemsNoApproaches)
	}

	// Verify suggested actions
	if briefing.SuggestedActions == nil {
		t.Fatal("expected SuggestedActions to be populated")
	}
	if len(briefing.SuggestedActions) != 1 {
		t.Errorf("expected 1 suggested action, got %d", len(briefing.SuggestedActions))
	}

	// Verify opportunities
	if briefing.Opportunities == nil {
		t.Fatal("expected Opportunities to be populated")
	}
	if briefing.Opportunities.ProblemsInMyDomain != 5 {
		t.Errorf("expected ProblemsInMyDomain=5, got %d", briefing.Opportunities.ProblemsInMyDomain)
	}

	// Verify reputation changes
	if briefing.ReputationChanges == nil {
		t.Fatal("expected ReputationChanges to be populated")
	}
	if briefing.ReputationChanges.SinceLastCheck != "+20" {
		t.Errorf("expected SinceLastCheck='+20', got %q", briefing.ReputationChanges.SinceLastCheck)
	}
}

// TestBriefingService_GracefulDegradation verifies that if one repo errors,
// the other 4 sections are still populated (nil for the failed one).
func TestBriefingService_GracefulDegradation(t *testing.T) {
	now := time.Now()
	lastBriefing := now.Add(-4 * time.Hour)

	// Inbox repo returns error
	inboxRepo := &mockInboxRepo{err: errors.New("db connection failed")}

	openItemsRepo := &mockOpenItemsRepo{
		result: &models.OpenItemsResult{
			ProblemsNoApproaches: 2,
			Items:                []models.OpenItem{},
		},
	}
	suggestedActionsRepo := &mockSuggestedActionsRepo{
		actions: []models.SuggestedAction{},
	}
	opportunitiesRepo := &mockOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 3,
			Items:              []models.Opportunity{},
		},
	}
	reputationRepo := &mockReputationRepo{
		result: &models.ReputationChangesResult{
			SinceLastCheck: "+0",
			Breakdown:      []models.ReputationEvent{},
		},
	}
	agentRepo := &mockAgentBriefingRepo{}

	svc := NewBriefingService(
		inboxRepo,
		openItemsRepo,
		suggestedActionsRepo,
		opportunitiesRepo,
		reputationRepo,
		agentRepo,
	)

	agent := &models.Agent{
		ID:             "test-agent",
		Specialties:    []string{"go"},
		CreatedAt:      now.Add(-30 * 24 * time.Hour),
		LastBriefingAt: &lastBriefing,
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error (should not fail even with one repo error): %v", err)
	}

	// Inbox should be nil (error)
	if briefing.Inbox != nil {
		t.Error("expected Inbox to be nil when repo errors")
	}

	// Other 4 sections should be populated
	if briefing.MyOpenItems == nil {
		t.Error("expected MyOpenItems to be populated despite inbox error")
	}
	if briefing.SuggestedActions == nil {
		t.Error("expected SuggestedActions to be populated despite inbox error")
	}
	if briefing.Opportunities == nil {
		t.Error("expected Opportunities to be populated despite inbox error")
	}
	if briefing.ReputationChanges == nil {
		t.Error("expected ReputationChanges to be populated despite inbox error")
	}
}

// TestBriefingService_UpdatesLastBriefingAt verifies that after assembling
// the briefing, UpdateLastBriefingAt is called with the agent's ID.
func TestBriefingService_UpdatesLastBriefingAt(t *testing.T) {
	now := time.Now()

	inboxRepo := &mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0}
	openItemsRepo := &mockOpenItemsRepo{result: &models.OpenItemsResult{Items: []models.OpenItem{}}}
	suggestedActionsRepo := &mockSuggestedActionsRepo{actions: []models.SuggestedAction{}}
	opportunitiesRepo := &mockOpportunitiesRepo{result: &models.OpportunitiesSection{Items: []models.Opportunity{}}}
	reputationRepo := &mockReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}}
	agentRepo := &mockAgentBriefingRepo{}

	svc := NewBriefingService(
		inboxRepo,
		openItemsRepo,
		suggestedActionsRepo,
		opportunitiesRepo,
		reputationRepo,
		agentRepo,
	)

	agent := &models.Agent{
		ID:        "agent-xyz",
		CreatedAt: now.Add(-7 * 24 * time.Hour),
	}

	_, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if agentRepo.calledWith != "agent-xyz" {
		t.Errorf("expected UpdateLastBriefingAt called with 'agent-xyz', got %q", agentRepo.calledWith)
	}
}

// TestBriefingService_EmptyAgent verifies that a new agent with no activity
// gets all sections present but empty/zero values.
func TestBriefingService_EmptyAgent(t *testing.T) {
	now := time.Now()

	inboxRepo := &mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0}
	openItemsRepo := &mockOpenItemsRepo{result: &models.OpenItemsResult{
		ProblemsNoApproaches: 0,
		QuestionsNoAnswers:   0,
		ApproachesStale:      0,
		Items:                []models.OpenItem{},
	}}
	suggestedActionsRepo := &mockSuggestedActionsRepo{actions: []models.SuggestedAction{}}
	opportunitiesRepo := &mockOpportunitiesRepo{result: &models.OpportunitiesSection{
		ProblemsInMyDomain: 0,
		Items:              []models.Opportunity{},
	}}
	reputationRepo := &mockReputationRepo{result: &models.ReputationChangesResult{
		SinceLastCheck: "+0",
		Breakdown:      []models.ReputationEvent{},
	}}
	agentRepo := &mockAgentBriefingRepo{}

	svc := NewBriefingService(
		inboxRepo,
		openItemsRepo,
		suggestedActionsRepo,
		opportunitiesRepo,
		reputationRepo,
		agentRepo,
	)

	agent := &models.Agent{
		ID:          "new-agent",
		Specialties: []string{"python"},
		CreatedAt:   now.Add(-1 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// All sections should be present
	if briefing.Inbox == nil {
		t.Fatal("expected Inbox to be present for empty agent")
	}
	if briefing.Inbox.UnreadCount != 0 {
		t.Errorf("expected 0 unread, got %d", briefing.Inbox.UnreadCount)
	}
	if len(briefing.Inbox.Items) != 0 {
		t.Errorf("expected 0 inbox items, got %d", len(briefing.Inbox.Items))
	}

	if briefing.MyOpenItems == nil {
		t.Fatal("expected MyOpenItems to be present")
	}
	if briefing.MyOpenItems.ProblemsNoApproaches != 0 {
		t.Errorf("expected 0 problems_no_approaches, got %d", briefing.MyOpenItems.ProblemsNoApproaches)
	}

	if briefing.SuggestedActions == nil {
		t.Fatal("expected SuggestedActions to be present")
	}
	if len(briefing.SuggestedActions) != 0 {
		t.Errorf("expected 0 suggested actions, got %d", len(briefing.SuggestedActions))
	}

	if briefing.Opportunities == nil {
		t.Fatal("expected Opportunities to be present")
	}
	if briefing.Opportunities.ProblemsInMyDomain != 0 {
		t.Errorf("expected 0 problems_in_my_domain, got %d", briefing.Opportunities.ProblemsInMyDomain)
	}

	if briefing.ReputationChanges == nil {
		t.Fatal("expected ReputationChanges to be present")
	}
	if briefing.ReputationChanges.SinceLastCheck != "+0" {
		t.Errorf("expected '+0', got %q", briefing.ReputationChanges.SinceLastCheck)
	}
	if len(briefing.ReputationChanges.Breakdown) != 0 {
		t.Errorf("expected 0 breakdown events, got %d", len(briefing.ReputationChanges.Breakdown))
	}
}
