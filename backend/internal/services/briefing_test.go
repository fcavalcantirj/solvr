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

// --- Mock implementations for new platform repos ---

type mockPlatformPulseRepo struct {
	result *models.PlatformPulse
	err    error
}

func (m *mockPlatformPulseRepo) GetPlatformPulse(_ context.Context) (*models.PlatformPulse, error) {
	return m.result, m.err
}

type mockTrendingRepo struct {
	result []models.TrendingPost
	err    error
}

func (m *mockTrendingRepo) GetTrendingNow(_ context.Context, _ string, _ int) ([]models.TrendingPost, error) {
	return m.result, m.err
}

type mockHardcoreRepo struct {
	result []models.HardcoreUnsolved
	err    error
}

func (m *mockHardcoreRepo) GetHardcoreUnsolved(_ context.Context, _ int) ([]models.HardcoreUnsolved, error) {
	return m.result, m.err
}

type mockRisingIdeasRepo struct {
	result []models.RisingIdea
	err    error
}

func (m *mockRisingIdeasRepo) GetRisingIdeas(_ context.Context, _ int) ([]models.RisingIdea, error) {
	return m.result, m.err
}

type mockVictoriesRepo struct {
	result []models.RecentVictory
	err    error
}

func (m *mockVictoriesRepo) GetRecentVictories(_ context.Context, _ int) ([]models.RecentVictory, error) {
	return m.result, m.err
}

type mockRecommendationsRepo struct {
	result []models.RecommendedPost
	err    error
}

func (m *mockRecommendationsRepo) GetYouMightLike(_ context.Context, _ string, _ []string, _ int) ([]models.RecommendedPost, error) {
	return m.result, m.err
}

// TestNewBriefingServiceWithDeps_AllSections verifies that NewBriefingServiceWithDeps
// populates all 11 briefing sections when all 12 repos are provided.
func TestNewBriefingServiceWithDeps_AllSections(t *testing.T) {
	now := time.Now()
	lastBriefing := now.Add(-4 * time.Hour)

	deps := BriefingDeps{
		InboxRepo: &mockInboxRepo{
			notifications: []models.Notification{
				{ID: "n1", Type: "answer.created", Title: "New answer", Body: "body", Link: "/q/1", CreatedAt: now},
			},
			totalUnread: 1,
		},
		OpenItemsRepo: &mockOpenItemsRepo{
			result: &models.OpenItemsResult{ProblemsNoApproaches: 1, Items: []models.OpenItem{}},
		},
		SuggestedActionsRepo: &mockSuggestedActionsRepo{
			actions: []models.SuggestedAction{{Action: "update", TargetID: "a1", TargetTitle: "Fix", Reason: "Stale"}},
		},
		OpportunitiesRepo: &mockOpportunitiesRepo{
			result: &models.OpportunitiesSection{ProblemsInMyDomain: 2, Items: []models.Opportunity{}},
		},
		ReputationRepo: &mockReputationRepo{
			result: &models.ReputationChangesResult{SinceLastCheck: "+10", Breakdown: []models.ReputationEvent{}},
		},
		AgentRepo: &mockAgentBriefingRepo{},
		PlatformPulseRepo: &mockPlatformPulseRepo{
			result: &models.PlatformPulse{OpenProblems: 10, OpenQuestions: 5, ActiveIdeas: 3, NewPostsLast24h: 20, SolvedLast7d: 2, ActiveAgentsLast24h: 8, ContributorsThisWeek: 15},
		},
		TrendingRepo: &mockTrendingRepo{
			result: []models.TrendingPost{{ID: "t1", Type: "question", Title: "Hot topic", EngagementScore: 42}},
		},
		HardcoreRepo: &mockHardcoreRepo{
			result: []models.HardcoreUnsolved{{ID: "h1", Title: "Hard bug", FailedApproaches: 3, DifficultyScore: 15}},
		},
		RisingIdeasRepo: &mockRisingIdeasRepo{
			result: []models.RisingIdea{{ID: "r1", Title: "Cool idea", ResponseCount: 5, Upvotes: 10}},
		},
		VictoriesRepo: &mockVictoriesRepo{
			result: []models.RecentVictory{{ID: "v1", Title: "Solved!", SolvedBy: "agent1", DaysToSolve: 3, SolvedAt: now}},
		},
		RecommendationsRepo: &mockRecommendationsRepo{
			result: []models.RecommendedPost{{ID: "rec1", Type: "problem", Title: "You might like", MatchReason: "tag_affinity"}},
		},
	}

	svc := NewBriefingServiceWithDeps(deps)

	agent := &models.Agent{
		ID:             "test-agent",
		Specialties:    []string{"go"},
		CreatedAt:      now.Add(-30 * 24 * time.Hour),
		LastBriefingAt: &lastBriefing,
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify all 5 original sections
	if briefing.Inbox == nil {
		t.Error("expected Inbox to be populated")
	}
	if briefing.MyOpenItems == nil {
		t.Error("expected MyOpenItems to be populated")
	}
	if briefing.SuggestedActions == nil {
		t.Error("expected SuggestedActions to be populated")
	}
	if briefing.Opportunities == nil {
		t.Error("expected Opportunities to be populated")
	}
	if briefing.ReputationChanges == nil {
		t.Error("expected ReputationChanges to be populated")
	}

	// Verify all 6 new sections
	if briefing.PlatformPulse == nil {
		t.Error("expected PlatformPulse to be populated")
	} else if briefing.PlatformPulse.OpenProblems != 10 {
		t.Errorf("expected OpenProblems=10, got %d", briefing.PlatformPulse.OpenProblems)
	}
	if briefing.TrendingNow == nil || len(briefing.TrendingNow) != 1 {
		t.Errorf("expected 1 trending post, got %v", briefing.TrendingNow)
	}
	if briefing.HardcoreUnsolved == nil || len(briefing.HardcoreUnsolved) != 1 {
		t.Errorf("expected 1 hardcore unsolved, got %v", briefing.HardcoreUnsolved)
	}
	if briefing.RisingIdeas == nil || len(briefing.RisingIdeas) != 1 {
		t.Errorf("expected 1 rising idea, got %v", briefing.RisingIdeas)
	}
	if briefing.RecentVictories == nil || len(briefing.RecentVictories) != 1 {
		t.Errorf("expected 1 recent victory, got %v", briefing.RecentVictories)
	}
	if briefing.YouMightLike == nil || len(briefing.YouMightLike) != 1 {
		t.Errorf("expected 1 recommendation, got %v", briefing.YouMightLike)
	}
}

// TestNewBriefingServiceWithDeps_NilPlatformRepos verifies that when new repos are nil,
// the original 5 sections work and the new 6 sections are nil (not error).
func TestNewBriefingServiceWithDeps_NilPlatformRepos(t *testing.T) {
	now := time.Now()

	deps := BriefingDeps{
		InboxRepo: &mockInboxRepo{
			notifications: []models.Notification{},
			totalUnread:   0,
		},
		OpenItemsRepo: &mockOpenItemsRepo{
			result: &models.OpenItemsResult{Items: []models.OpenItem{}},
		},
		SuggestedActionsRepo: &mockSuggestedActionsRepo{
			actions: []models.SuggestedAction{},
		},
		OpportunitiesRepo: &mockOpportunitiesRepo{
			result: &models.OpportunitiesSection{Items: []models.Opportunity{}},
		},
		ReputationRepo: &mockReputationRepo{
			result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}},
		},
		AgentRepo: &mockAgentBriefingRepo{},
		// All 6 new repos are nil (zero value)
	}

	svc := NewBriefingServiceWithDeps(deps)

	agent := &models.Agent{
		ID:          "test-agent",
		Specialties: []string{"go"},
		CreatedAt:   now.Add(-7 * 24 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original 5 sections should be populated
	if briefing.Inbox == nil {
		t.Error("expected Inbox to be populated")
	}
	if briefing.MyOpenItems == nil {
		t.Error("expected MyOpenItems to be populated")
	}
	if briefing.ReputationChanges == nil {
		t.Error("expected ReputationChanges to be populated")
	}

	// New 6 sections should be nil (repos not provided)
	if briefing.PlatformPulse != nil {
		t.Error("expected PlatformPulse to be nil when repo not provided")
	}
	if briefing.TrendingNow != nil {
		t.Error("expected TrendingNow to be nil when repo not provided")
	}
	if briefing.HardcoreUnsolved != nil {
		t.Error("expected HardcoreUnsolved to be nil when repo not provided")
	}
	if briefing.RisingIdeas != nil {
		t.Error("expected RisingIdeas to be nil when repo not provided")
	}
	if briefing.RecentVictories != nil {
		t.Error("expected RecentVictories to be nil when repo not provided")
	}
	if briefing.YouMightLike != nil {
		t.Error("expected YouMightLike to be nil when repo not provided")
	}
}

// TestOldConstructor_BackwardsCompat verifies that NewBriefingService() still works,
// populates the original 5 agent-centric sections, and leaves the 6 new sections nil.
func TestOldConstructor_BackwardsCompat(t *testing.T) {
	now := time.Now()

	svc := NewBriefingService(
		&mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0},
		&mockOpenItemsRepo{result: &models.OpenItemsResult{Items: []models.OpenItem{}}},
		&mockSuggestedActionsRepo{actions: []models.SuggestedAction{}},
		&mockOpportunitiesRepo{result: &models.OpportunitiesSection{Items: []models.Opportunity{}}},
		&mockReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}},
		&mockAgentBriefingRepo{},
	)

	agent := &models.Agent{
		ID:          "compat-agent",
		Specialties: []string{"python"},
		CreatedAt:   now.Add(-7 * 24 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Original 5 sections should work
	if briefing.Inbox == nil {
		t.Error("expected Inbox to be populated")
	}
	if briefing.MyOpenItems == nil {
		t.Error("expected MyOpenItems to be populated")
	}
	if briefing.SuggestedActions == nil {
		t.Error("expected SuggestedActions to be populated")
	}
	if briefing.Opportunities == nil {
		t.Error("expected Opportunities to be populated")
	}
	if briefing.ReputationChanges == nil {
		t.Error("expected ReputationChanges to be populated")
	}

	// 6 new sections should be nil (old constructor doesn't set new repos)
	if briefing.PlatformPulse != nil {
		t.Error("expected PlatformPulse to be nil with old constructor")
	}
	if briefing.TrendingNow != nil {
		t.Error("expected TrendingNow to be nil with old constructor")
	}
	if briefing.HardcoreUnsolved != nil {
		t.Error("expected HardcoreUnsolved to be nil with old constructor")
	}
	if briefing.RisingIdeas != nil {
		t.Error("expected RisingIdeas to be nil with old constructor")
	}
	if briefing.RecentVictories != nil {
		t.Error("expected RecentVictories to be nil with old constructor")
	}
	if briefing.YouMightLike != nil {
		t.Error("expected YouMightLike to be nil with old constructor")
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
