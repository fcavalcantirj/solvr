package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ============================================================================
// Tests for List agents endpoint (API-001)
// GET /v1/agents - list registered agents with pagination and sorting
// ============================================================================

func TestAgentRepository_List_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// List with default options
	opts := models.AgentListOptions{
		Page:    1,
		PerPage: 20,
	}

	agents, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// May have agents from other tests, but should not error
	if agents == nil {
		t.Error("expected non-nil slice, got nil")
	}

	// Total should be >= 0
	if total < 0 {
		t.Errorf("expected total >= 0, got %d", total)
	}
}

func TestAgentRepository_List_WithAgents(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	postsRepo := NewPostRepository(pool)
	ctx := context.Background()

	// Create test agents with unique IDs
	timestamp := time.Now().Format("20060102150405")

	agent1 := &models.Agent{
		ID:          "list_agent1_" + timestamp,
		DisplayName: "List Test Agent 1",
		Bio:         "First test agent for listing",
		Karma:       100,
	}
	agent2 := &models.Agent{
		ID:          "list_agent2_" + timestamp,
		DisplayName: "List Test Agent 2",
		Bio:         "Second test agent for listing",
		Karma:       50,
	}

	err := repo.Create(ctx, agent1)
	if err != nil {
		t.Fatalf("failed to create agent1: %v", err)
	}
	err = repo.Create(ctx, agent2)
	if err != nil {
		t.Fatalf("failed to create agent2: %v", err)
	}

	// Create a post for agent1 to test post_count
	post := &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Test question for agent list",
		Description:  "Testing post count in agent list",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent1.ID,
		Status:       models.PostStatusOpen,
	}
	_, err = postsRepo.Create(ctx, post)
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	// List agents
	opts := models.AgentListOptions{
		Page:    1,
		PerPage: 100,
	}

	agents, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Should have at least 2 agents
	if total < 2 {
		t.Errorf("expected total >= 2, got %d", total)
	}

	// Find our test agents in the results
	var foundAgent1, foundAgent2 bool
	for _, a := range agents {
		if a.ID == agent1.ID {
			foundAgent1 = true
			// Verify post_count is included
			if a.PostCount < 1 {
				t.Errorf("expected agent1 to have post_count >= 1, got %d", a.PostCount)
			}
		}
		if a.ID == agent2.ID {
			foundAgent2 = true
		}
	}

	if !foundAgent1 {
		t.Error("agent1 not found in list results")
	}
	if !foundAgent2 {
		t.Error("agent2 not found in list results")
	}
}

func TestAgentRepository_List_SortByKarma(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create agents with different karma
	timestamp := time.Now().Format("20060102150405")

	lowKarma := &models.Agent{
		ID:          "karma_low_" + timestamp,
		DisplayName: "Low Karma Agent",
	}
	highKarma := &models.Agent{
		ID:          "karma_high_" + timestamp,
		DisplayName: "High Karma Agent",
	}

	err := repo.Create(ctx, lowKarma)
	if err != nil {
		t.Fatalf("failed to create low karma agent: %v", err)
	}
	err = repo.Create(ctx, highKarma)
	if err != nil {
		t.Fatalf("failed to create high karma agent: %v", err)
	}

	// Add karma to differentiate
	repo.AddKarma(ctx, highKarma.ID, 200)
	repo.AddKarma(ctx, lowKarma.ID, 10)

	// List sorted by karma descending
	opts := models.AgentListOptions{
		Page:    1,
		PerPage: 100,
		Sort:    "karma",
	}

	agents, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Find positions of our test agents
	var highPos, lowPos int = -1, -1
	for i, a := range agents {
		if a.ID == highKarma.ID {
			highPos = i
		}
		if a.ID == lowKarma.ID {
			lowPos = i
		}
	}

	if highPos == -1 || lowPos == -1 {
		t.Fatal("test agents not found in results")
	}

	// High karma should come before low karma
	if highPos >= lowPos {
		t.Errorf("expected high karma agent (pos %d) before low karma agent (pos %d)", highPos, lowPos)
	}
}

func TestAgentRepository_List_SortByNewest(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create agents in sequence
	timestamp := time.Now().Format("20060102150405")

	older := &models.Agent{
		ID:          "older_" + timestamp,
		DisplayName: "Older Agent",
	}

	err := repo.Create(ctx, older)
	if err != nil {
		t.Fatalf("failed to create older agent: %v", err)
	}

	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)

	newer := &models.Agent{
		ID:          "newer_" + timestamp,
		DisplayName: "Newer Agent",
	}
	err = repo.Create(ctx, newer)
	if err != nil {
		t.Fatalf("failed to create newer agent: %v", err)
	}

	// List sorted by newest (default)
	opts := models.AgentListOptions{
		Page:    1,
		PerPage: 100,
		Sort:    "newest",
	}

	agents, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// Find positions
	var newerPos, olderPos int = -1, -1
	for i, a := range agents {
		if a.ID == newer.ID {
			newerPos = i
		}
		if a.ID == older.ID {
			olderPos = i
		}
	}

	if newerPos == -1 || olderPos == -1 {
		t.Fatal("test agents not found in results")
	}

	// Newer should come before older
	if newerPos >= olderPos {
		t.Errorf("expected newer agent (pos %d) before older agent (pos %d)", newerPos, olderPos)
	}
}

func TestAgentRepository_List_FilterByStatus(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	timestamp := time.Now().Format("20060102150405")

	activeAgent := &models.Agent{
		ID:          "status_active_" + timestamp,
		DisplayName: "Active Status Agent",
	}

	err := repo.Create(ctx, activeAgent)
	if err != nil {
		t.Fatalf("failed to create active agent: %v", err)
	}

	// List with status filter
	opts := models.AgentListOptions{
		Page:    1,
		PerPage: 100,
		Status:  "active",
	}

	agents, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}

	// All returned agents should be active
	for _, a := range agents {
		if a.Status != "active" {
			t.Errorf("expected status 'active', got '%s' for agent %s", a.Status, a.ID)
		}
	}
}

func TestAgentRepository_List_Pagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewAgentRepository(pool)
	ctx := context.Background()

	// Create some test agents
	timestamp := time.Now().Format("20060102150405")
	for i := 0; i < 5; i++ {
		agent := &models.Agent{
			ID:          "page_agent_" + timestamp + "_" + string(rune('a'+i)),
			DisplayName: "Pagination Test Agent",
		}
		if err := repo.Create(ctx, agent); err != nil {
			t.Fatalf("failed to create agent %d: %v", i, err)
		}
	}

	// First page with limit 2
	opts := models.AgentListOptions{
		Page:    1,
		PerPage: 2,
	}

	page1, total, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List page 1 failed: %v", err)
	}

	if len(page1) != 2 {
		t.Errorf("expected 2 agents on page 1, got %d", len(page1))
	}

	// Total should be >= 5
	if total < 5 {
		t.Errorf("expected total >= 5, got %d", total)
	}

	// Second page
	opts.Page = 2
	page2, _, err := repo.List(ctx, opts)
	if err != nil {
		t.Fatalf("List page 2 failed: %v", err)
	}

	if len(page2) != 2 {
		t.Errorf("expected 2 agents on page 2, got %d", len(page2))
	}

	// Pages should have different agents
	if len(page1) > 0 && len(page2) > 0 && page1[0].ID == page2[0].ID {
		t.Error("page 1 and page 2 should have different agents")
	}
}
