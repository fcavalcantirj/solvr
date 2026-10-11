package services

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// --- Mock implementation for inferred specialties ---

type mockInferredSpecialtiesRepo struct {
	specialties []string
	err         error
	calledWith  string
	called      bool
}

func (m *mockInferredSpecialtiesRepo) InferSpecialtiesForAgent(_ context.Context, agentID string) ([]string, error) {
	m.called = true
	m.calledWith = agentID
	return m.specialties, m.err
}

// TestBriefing_Opportunities_InferredSpecialties verifies that when an agent
// has empty specialties and InferredSpecialtiesRepo is set, the service falls
// back to inferred specialties and populates opportunities with inferred_from='inferred'.
func TestBriefing_Opportunities_InferredSpecialties(t *testing.T) {
	now := time.Now()

	inferredRepo := &mockInferredSpecialtiesRepo{
		specialties: []string{"go", "postgresql"},
	}

	opportunitiesRepo := &mockOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 3,
			Items: []models.Opportunity{
				{ID: "opp1", Title: "Go bug", Tags: []string{"go"}, ApproachesCount: 0, PostedBy: "user1", AgeHours: 12},
			},
		},
	}

	deps := BriefingDeps{
		InboxRepo:               &mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0},
		OpenItemsRepo:           &mockOpenItemsRepo{result: &models.OpenItemsResult{Items: []models.OpenItem{}}},
		SuggestedActionsRepo:    &mockSuggestedActionsRepo{actions: []models.SuggestedAction{}},
		OpportunitiesRepo:       opportunitiesRepo,
		ReputationRepo:          &mockReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}},
		AgentRepo:               &mockAgentBriefingRepo{},
		InferredSpecialtiesRepo: inferredRepo,
	}

	svc := NewBriefingServiceWithDeps(deps)

	// Agent with EMPTY specialties — should trigger inference
	agent := &models.Agent{
		ID:          "empty-spec-agent",
		Specialties: []string{},
		CreatedAt:   now.Add(-7 * 24 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inferred specialties repo should have been called
	if !inferredRepo.called {
		t.Fatal("expected InferSpecialtiesForAgent to be called when agent has empty specialties")
	}
	if inferredRepo.calledWith != "empty-spec-agent" {
		t.Errorf("expected InferSpecialtiesForAgent called with %q, got %q", "empty-spec-agent", inferredRepo.calledWith)
	}

	// Opportunities should be populated from inferred specialties
	if briefing.Opportunities == nil {
		t.Fatal("expected Opportunities to be populated via inferred specialties")
	}
	if briefing.Opportunities.ProblemsInMyDomain != 3 {
		t.Errorf("expected ProblemsInMyDomain=3, got %d", briefing.Opportunities.ProblemsInMyDomain)
	}
	if briefing.Opportunities.InferredFrom != "inferred" {
		t.Errorf("expected InferredFrom='inferred', got %q", briefing.Opportunities.InferredFrom)
	}
}

// TestBriefing_Opportunities_ExplicitTakesPrecedence verifies that when an agent
// has explicit specialties set, inferred specialties are NOT called and
// inferred_from is 'explicit'.
func TestBriefing_Opportunities_ExplicitTakesPrecedence(t *testing.T) {
	now := time.Now()

	inferredRepo := &mockInferredSpecialtiesRepo{
		specialties: []string{"python"},
	}

	opportunitiesRepo := &mockOpportunitiesRepo{
		result: &models.OpportunitiesSection{
			ProblemsInMyDomain: 5,
			Items:              []models.Opportunity{},
		},
	}

	deps := BriefingDeps{
		InboxRepo:               &mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0},
		OpenItemsRepo:           &mockOpenItemsRepo{result: &models.OpenItemsResult{Items: []models.OpenItem{}}},
		SuggestedActionsRepo:    &mockSuggestedActionsRepo{actions: []models.SuggestedAction{}},
		OpportunitiesRepo:       opportunitiesRepo,
		ReputationRepo:          &mockReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}},
		AgentRepo:               &mockAgentBriefingRepo{},
		InferredSpecialtiesRepo: inferredRepo,
	}

	svc := NewBriefingServiceWithDeps(deps)

	// Agent with EXPLICIT specialties — inference should NOT be called
	agent := &models.Agent{
		ID:          "explicit-spec-agent",
		Specialties: []string{"go", "rust"},
		CreatedAt:   now.Add(-7 * 24 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Inferred specialties repo should NOT have been called
	if inferredRepo.called {
		t.Error("expected InferSpecialtiesForAgent NOT to be called when agent has explicit specialties")
	}

	// Opportunities should be populated from explicit specialties
	if briefing.Opportunities == nil {
		t.Fatal("expected Opportunities to be populated via explicit specialties")
	}
	if briefing.Opportunities.InferredFrom != "explicit" {
		t.Errorf("expected InferredFrom='explicit', got %q", briefing.Opportunities.InferredFrom)
	}
}

// --- Mock implementation for checkpoint finder ---

type mockCheckpointFinder struct {
	pin *models.Pin
	err error
}

func (m *mockCheckpointFinder) FindLatestCheckpoint(_ context.Context, _ string) (*models.Pin, error) {
	return m.pin, m.err
}

// TestBriefing_IncludesLatestCheckpoint verifies that when an agent has checkpoints,
// the briefing response includes the newest checkpoint as latest_checkpoint.
func TestBriefing_IncludesLatestCheckpoint(t *testing.T) {
	now := time.Now()
	pinnedAt := now.Add(-1 * time.Hour)

	checkpointFinder := &mockCheckpointFinder{
		pin: &models.Pin{
			ID:        "chk-1",
			CID:       "bafyabc123",
			Status:    models.PinStatusPinned,
			Name:      "checkpoint_bafyabc1_20260221",
			Meta:      map[string]string{"type": "amcp_checkpoint", "agent_id": "ckpt-agent"},
			Delegates: []string{},
			OwnerID:   "ckpt-agent",
			OwnerType: "agent",
			CreatedAt: now.Add(-2 * time.Hour),
			PinnedAt:  &pinnedAt,
		},
	}

	deps := BriefingDeps{
		InboxRepo:            &mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0},
		OpenItemsRepo:        &mockOpenItemsRepo{result: &models.OpenItemsResult{Items: []models.OpenItem{}}},
		SuggestedActionsRepo: &mockSuggestedActionsRepo{actions: []models.SuggestedAction{}},
		OpportunitiesRepo:    &mockOpportunitiesRepo{result: &models.OpportunitiesSection{Items: []models.Opportunity{}}},
		ReputationRepo:       &mockReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}},
		AgentRepo:            &mockAgentBriefingRepo{},
		CheckpointFinder:     checkpointFinder,
	}

	svc := NewBriefingServiceWithDeps(deps)

	agent := &models.Agent{
		ID:          "ckpt-agent",
		Specialties: []string{"go"},
		CreatedAt:   now.Add(-30 * 24 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if briefing.LatestCheckpoint == nil {
		t.Fatal("expected LatestCheckpoint to be populated")
	}
	if briefing.LatestCheckpoint.Pin.CID != "bafyabc123" {
		t.Errorf("expected CID='bafyabc123', got %q", briefing.LatestCheckpoint.Pin.CID)
	}
	if briefing.LatestCheckpoint.Pin.Name != "checkpoint_bafyabc1_20260221" {
		t.Errorf("expected Name='checkpoint_bafyabc1_20260221', got %q", briefing.LatestCheckpoint.Pin.Name)
	}
	if briefing.LatestCheckpoint.Pin.Meta["type"] != "amcp_checkpoint" {
		t.Errorf("expected meta type='amcp_checkpoint', got %q", briefing.LatestCheckpoint.Pin.Meta["type"])
	}
}

// TestBriefing_NoCheckpoint verifies that when no checkpoint exists,
// latest_checkpoint is nil in the briefing response.
func TestBriefing_NoCheckpoint(t *testing.T) {
	now := time.Now()

	checkpointFinder := &mockCheckpointFinder{
		pin: nil, // no checkpoint found
		err: nil,
	}

	deps := BriefingDeps{
		InboxRepo:            &mockInboxRepo{notifications: []models.Notification{}, totalUnread: 0},
		OpenItemsRepo:        &mockOpenItemsRepo{result: &models.OpenItemsResult{Items: []models.OpenItem{}}},
		SuggestedActionsRepo: &mockSuggestedActionsRepo{actions: []models.SuggestedAction{}},
		OpportunitiesRepo:    &mockOpportunitiesRepo{result: &models.OpportunitiesSection{Items: []models.Opportunity{}}},
		ReputationRepo:       &mockReputationRepo{result: &models.ReputationChangesResult{SinceLastCheck: "+0", Breakdown: []models.ReputationEvent{}}},
		AgentRepo:            &mockAgentBriefingRepo{},
		CheckpointFinder:     checkpointFinder,
	}

	svc := NewBriefingServiceWithDeps(deps)

	agent := &models.Agent{
		ID:          "no-ckpt-agent",
		Specialties: []string{"go"},
		CreatedAt:   now.Add(-7 * 24 * time.Hour),
	}

	briefing, err := svc.GetBriefingForAgent(context.Background(), agent)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if briefing.LatestCheckpoint != nil {
		t.Error("expected LatestCheckpoint to be nil when no checkpoint exists")
	}
}
