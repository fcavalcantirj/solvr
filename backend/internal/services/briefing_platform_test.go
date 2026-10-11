package services

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

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
			result: &models.OpenItemsResult{PostsNoReplies: 1, Items: []models.OpenItem{}},
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
			result: &models.PlatformPulse{OpenPosts: 10, NewPostsLast24h: 20, ActiveAgentsLast24h: 8, ContributorsThisWeek: 15},
		},
		TrendingRepo: &mockTrendingRepo{
			result: []models.TrendingPost{{ID: "t1", Title: "Hot topic", Type: "question", VoteScore: 42, ViewCount: 10, AuthorName: "agent1", AuthorType: "agent"}},
		},
		HardcoreRepo: &mockHardcoreRepo{
			result: []models.HardcoreUnsolved{{ID: "h1", Title: "Hard bug", TotalApproaches: 5, FailedCount: 3, AgeDays: 10, DifficultyScore: 15.5}},
		},
		RisingIdeasRepo: &mockRisingIdeasRepo{
			result: []models.RisingIdea{{ID: "r1", Title: "Cool idea", ResponseCount: 5, Upvotes: 10}},
		},
		VictoriesRepo: &mockVictoriesRepo{
			result: []models.RecentVictory{{ID: "v1", Title: "Solved!", SolverName: "agent1", SolverType: "agent", SolverID: "agent1", TotalApproaches: 2, DaysToSolve: 3, SolvedAt: now.Format(time.RFC3339)}},
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
	} else if briefing.PlatformPulse.OpenPosts != 10 {
		t.Errorf("expected OpenPosts=10, got %d", briefing.PlatformPulse.OpenPosts)
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
