package db

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestLeaderboard_AgentReputationFromSolvedPost verifies reputation calculation
// when agent posts a solved problem (+100 rep).
// This test exposes the bug where all_time uses stale agents.reputation.
func TestLeaderboard_AgentReputationFromSolvedPost(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	leaderboardRepo := NewLeaderboardRepository(pool)
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test agent
	agent := &models.Agent{
		ID:          "rep_test_agent_" + time.Now().Format("150405.000"),
		DisplayName: "Rep Test Agent",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create solved problem (+100 rep)
	_, err = postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Reputation Test Problem",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Status:       models.PostStatusSolved,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	// Fetch all_time leaderboard
	entries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// Find agent entry
	var agentEntry *models.LeaderboardEntry
	for i := range entries {
		if entries[i].ID == agent.ID {
			agentEntry = &entries[i]
			break
		}
	}

	if agentEntry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	// Verify reputation: problems_solved (100) + problems_contributed (25) = 125
	// Per SPEC.md, ALL problems count as contributed, even solved ones
	expectedReputation := 125
	if agentEntry.Reputation != expectedReputation {
		t.Errorf("expected reputation %d (100 solved + 25 contributed), got %d", expectedReputation, agentEntry.Reputation)
	}

	// Verify stats
	if agentEntry.KeyStats.ProblemsSolved != 1 {
		t.Errorf("expected 1 problem solved, got %d", agentEntry.KeyStats.ProblemsSolved)
	}

	t.Logf("✓ Agent reputation correctly calculated: %d", agentEntry.Reputation)
}

// TestLeaderboard_AgentReputationFromUpvotes verifies +2 rep per upvote.
func TestLeaderboard_AgentReputationFromUpvotes(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	leaderboardRepo := NewLeaderboardRepository(pool)
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test agent
	agent := &models.Agent{
		ID:          "rep_upvote_agent_" + time.Now().Format("150405.000"),
		DisplayName: "Upvote Test Agent",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create a post to receive upvotes
	post, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Upvote Test Problem",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Status:       models.PostStatusOpen,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create post: %v", err)
	}

	// Create 3 confirmed upvotes (+6 rep total)
	// Use direct SQL since VoteRepository doesn't exist yet
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `
			INSERT INTO votes (voter_id, target_type, target_id, direction, voter_type, confirmed)
			VALUES (gen_random_uuid(), 'post', $1, 'up', 'anonymous', true)
		`, post.ID)
		if err != nil {
			t.Fatalf("failed to create vote %d: %v", i, err)
		}
	}

	// Fetch all_time leaderboard
	entries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// Find agent entry
	var agentEntry *models.LeaderboardEntry
	for i := range entries {
		if entries[i].ID == agent.ID {
			agentEntry = &entries[i]
			break
		}
	}

	if agentEntry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	// Verify reputation is exactly 6 (3 upvotes * 2)
	if agentEntry.Reputation != 6 {
		t.Errorf("expected reputation 6 for 3 upvotes, got %d", agentEntry.Reputation)
	}

	// Verify stats
	if agentEntry.KeyStats.UpvotesReceived != 3 {
		t.Errorf("expected 3 upvotes, got %d", agentEntry.KeyStats.UpvotesReceived)
	}

	t.Logf("✓ Agent reputation from upvotes correctly calculated: %d", agentEntry.Reputation)
}

// TestLeaderboard_AgentReputationCombined verifies complex reputation calculation.
func TestLeaderboard_AgentReputationCombined(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	leaderboardRepo := NewLeaderboardRepository(pool)
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	answerRepo := NewAnswersRepository(pool)

	// Create test agent
	agent := &models.Agent{
		ID:          "rep_combined_agent_" + time.Now().Format("150405.000"),
		DisplayName: "Combined Rep Agent",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE target_type IN ('post', 'answer')")
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE author_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create 1 solved post (+100 rep)
	solvedPost, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Combined Test Solved Problem",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Status:       models.PostStatusSolved,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create solved post: %v", err)
	}

	// Create another post to answer
	questionPost, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Question for Answer Test",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "other-agent",
		Status:       models.PostStatusOpen,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create question post: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionPost.ID)
	}()

	// Create 1 accepted answer (+50 rep)
	answer, err := answerRepo.CreateAnswer(ctx, &models.Answer{
		QuestionID: questionPost.ID,
		Content:    "Test answer content",
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   agent.ID,
		IsAccepted: true,
	})
	if err != nil {
		t.Fatalf("failed to create answer: %v", err)
	}

	// Create 3 upvotes on solved post (+6 rep) - using direct SQL
	for i := 0; i < 3; i++ {
		_, err := pool.Exec(ctx, `
			INSERT INTO votes (target_type, target_id, direction, voter_type, confirmed)
			VALUES ('post', $1, 'up', 'anonymous', true)
		`, solvedPost.ID)
		if err != nil {
			t.Fatalf("failed to create upvote %d: %v", i, err)
		}
	}

	// Create 1 downvote on answer (-1 rep) - using direct SQL
	_, err = pool.Exec(ctx, `
		INSERT INTO votes (target_type, target_id, direction, voter_type, confirmed)
		VALUES ('answer', $1, 'down', 'anonymous', true)
	`, answer.ID)
	if err != nil {
		t.Fatalf("failed to create downvote: %v", err)
	}

	// Fetch all_time leaderboard
	entries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// Find agent entry
	var agentEntry *models.LeaderboardEntry
	for i := range entries {
		if entries[i].ID == agent.ID {
			agentEntry = &entries[i]
			break
		}
	}

	if agentEntry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	// Verify reputation: 100 (solved) + 50 (accepted) + 6 (upvotes) - 1 (downvote) = 155
	expectedRep := 155
	if agentEntry.Reputation != expectedRep {
		t.Errorf("expected reputation %d, got %d", expectedRep, agentEntry.Reputation)
		t.Logf("Breakdown: 100 (solved) + 50 (accepted) + 6 (3 upvotes) - 1 (downvote) = 155")
	}

	// Verify stats
	if agentEntry.KeyStats.ProblemsSolved != 1 {
		t.Errorf("expected 1 problem solved, got %d", agentEntry.KeyStats.ProblemsSolved)
	}
	if agentEntry.KeyStats.AnswersAccepted != 1 {
		t.Errorf("expected 1 answer accepted, got %d", agentEntry.KeyStats.AnswersAccepted)
	}
	if agentEntry.KeyStats.UpvotesReceived != 3 {
		t.Errorf("expected 3 upvotes, got %d", agentEntry.KeyStats.UpvotesReceived)
	}

	t.Logf("✓ Combined reputation correctly calculated: %d", agentEntry.Reputation)
}

// TestLeaderboard_AllTimeVsWeeklyConsistency verifies filters match for recent activity.
func TestLeaderboard_AllTimeVsWeeklyConsistency(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	leaderboardRepo := NewLeaderboardRepository(pool)
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)

	// Create test agent
	agent := &models.Agent{
		ID:          "rep_consistency_agent_" + time.Now().Format("150405.000"),
		DisplayName: "Consistency Test Agent",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create activity this week (2 solved posts = 200 rep)
	for i := 0; i < 2; i++ {
		_, err := postRepo.Create(ctx, &models.Post{
			Type:         models.PostTypeProblem,
			Title:        "Consistency Test Problem",
			Description:  "Test body",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   agent.ID,
			Status:       models.PostStatusSolved,
			Tags:         []string{"test"},
		})
		if err != nil {
			t.Fatalf("failed to create post %d: %v", i, err)
		}
	}

	// Fetch all_time reputation
	allTimeEntries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard all_time failed: %v", err)
	}

	// Fetch weekly reputation
	weeklyEntries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "weekly",
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard weekly failed: %v", err)
	}

	// Find agent in both results
	var allTimeRep, weeklyRep int
	for _, entry := range allTimeEntries {
		if entry.ID == agent.ID {
			allTimeRep = entry.Reputation
			break
		}
	}
	for _, entry := range weeklyEntries {
		if entry.ID == agent.ID {
			weeklyRep = entry.Reputation
			break
		}
	}

	// Since all activity is this week, all_time and weekly should match
	if allTimeRep != weeklyRep {
		t.Errorf("INCONSISTENCY: all_time reputation (%d) != weekly reputation (%d)", allTimeRep, weeklyRep)
		t.Logf("This proves the bug: all_time uses stale cache, weekly recalculates")
	} else {
		t.Logf("✓ all_time and weekly reputation consistent: %d", allTimeRep)
	}

	// Both should be 200 (2 solved posts * 100)
	if allTimeRep != 200 {
		t.Errorf("expected all_time reputation 200, got %d", allTimeRep)
	}
	if weeklyRep != 200 {
		t.Errorf("expected weekly reputation 200, got %d", weeklyRep)
	}
}

// TestLeaderboard_RankingOrderAllTime verifies correct ranking by reputation.
func TestLeaderboard_RankingOrderAllTime(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect: %v", err)
	}
	defer pool.Close()

	leaderboardRepo := NewLeaderboardRepository(pool)
	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	answerRepo := NewAnswersRepository(pool)

	suffix := time.Now().Format("150405.000")

	// Create Agent A: 200 rep (2 solved posts)
	agentA := &models.Agent{
		ID:          "rank_agent_a_" + suffix,
		DisplayName: "Agent A - 200 Rep",
		APIKeyHash:  mustHash(t, "test-key-a"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agentA); err != nil {
		t.Fatalf("failed to create agent A: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agentA.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentA.ID)
	}()

	for i := 0; i < 2; i++ {
		_, err := postRepo.Create(ctx, &models.Post{
			Type:         models.PostTypeProblem,
			Title:        "Agent A Problem",
			Description:  "Test body",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   agentA.ID,
			Status:       models.PostStatusSolved,
			Tags:         []string{"test"},
		})
		if err != nil {
			t.Fatalf("failed to create post for agent A: %v", err)
		}
	}

	// Create Agent B: 100 rep (1 solved post)
	agentB := &models.Agent{
		ID:          "rank_agent_b_" + suffix,
		DisplayName: "Agent B - 100 Rep",
		APIKeyHash:  mustHash(t, "test-key-b"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agentB); err != nil {
		t.Fatalf("failed to create agent B: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agentB.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentB.ID)
	}()

	_, err = postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Agent B Problem",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agentB.ID,
		Status:       models.PostStatusSolved,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create post for agent B: %v", err)
	}

	// Create Agent C: 50 rep (1 accepted answer)
	agentC := &models.Agent{
		ID:          "rank_agent_c_" + suffix,
		DisplayName: "Agent C - 50 Rep",
		APIKeyHash:  mustHash(t, "test-key-c"),
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agentC); err != nil {
		t.Fatalf("failed to create agent C: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE author_id = $1", agentC.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentC.ID)
	}()

	// Create a post for agent C to answer
	questionForC, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Question for Agent C",
		Description:  "Test body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "other-agent-c",
		Status:       models.PostStatusOpen,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create question for agent C: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", questionForC.ID)
	}()

	_, err = answerRepo.CreateAnswer(ctx, &models.Answer{
		QuestionID: questionForC.ID,
		Content:    "Agent C answer",
		AuthorType: models.AuthorTypeAgent,
		AuthorID:   agentC.ID,
		IsAccepted: true,
	})
	if err != nil {
		t.Fatalf("failed to create answer for agent C: %v", err)
	}

	// Fetch leaderboard
	entries, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     50,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("GetLeaderboard failed: %v", err)
	}

	// Find ranks for our test agents
	ranks := make(map[string]int)
	reps := make(map[string]int)
	for _, entry := range entries {
		if entry.ID == agentA.ID || entry.ID == agentB.ID || entry.ID == agentC.ID {
			ranks[entry.ID] = entry.Rank
			reps[entry.ID] = entry.Reputation
		}
	}

	// Verify reputation values
	if reps[agentA.ID] != 200 {
		t.Errorf("Agent A: expected reputation 200, got %d", reps[agentA.ID])
	}
	if reps[agentB.ID] != 100 {
		t.Errorf("Agent B: expected reputation 100, got %d", reps[agentB.ID])
	}
	if reps[agentC.ID] != 50 {
		t.Errorf("Agent C: expected reputation 50, got %d", reps[agentC.ID])
	}

	// Verify ranking order: A < B < C (lower rank = better)
	if ranks[agentA.ID] >= ranks[agentB.ID] {
		t.Errorf("Agent A (rank %d, %d rep) should rank higher than Agent B (rank %d, %d rep)",
			ranks[agentA.ID], reps[agentA.ID], ranks[agentB.ID], reps[agentB.ID])
	}
	if ranks[agentB.ID] >= ranks[agentC.ID] {
		t.Errorf("Agent B (rank %d, %d rep) should rank higher than Agent C (rank %d, %d rep)",
			ranks[agentB.ID], reps[agentB.ID], ranks[agentC.ID], reps[agentC.ID])
	}

	t.Logf("✓ Ranking order correct:")
	t.Logf("  Agent A: rank %d, reputation %d", ranks[agentA.ID], reps[agentA.ID])
	t.Logf("  Agent B: rank %d, reputation %d", ranks[agentB.ID], reps[agentB.ID])
	t.Logf("  Agent C: rank %d, reputation %d", ranks[agentC.ID], reps[agentC.ID])
}

// ============================================================================
// NEW TESTS - COMPREHENSIVE REPUTATION CONSISTENCY
// These tests verify leaderboard uses the SAME formula as agent stats
// ============================================================================

// TestLeaderboard_ReputationMatchesAgentStats is the CRITICAL test that verifies
// leaderboard reputation uses the SAME formula as agent stats.
// This test will FAIL until leaderboard.go is fixed to match agents.go formula.
func TestLeaderboard_ReputationMatchesAgentStats(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	// Create test agent with 10 bonus points
	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_rep_consistency_" + suffix,
		DisplayName: "Test Rep Consistency",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
		Reputation:  10, // 10 bonus points
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE author_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1 OR id IN (SELECT id FROM posts WHERE posted_by_id = 'other_agent_reptest')", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create various activities for the agent
	// 1. Problem contributed (not solved): 25 points
	_, err = postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Contributed Problem",
		Description:  "Test problem",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Status:       models.PostStatusOpen, // NOT solved
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	// 2. Idea posted: 15 points
	_, err = postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeIdea,
		Title:        "Test Idea",
		Description:  "Test idea body",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create idea: %v", err)
	}

	// 3. Response given: 5 points (responses are for ideas)
	// First create an idea to respond to
	idea, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeIdea,
		Title:        "Test Idea for Response",
		Description:  "Test",
		Tags:         []string{"test"},
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   agent.ID,
		Status:       models.PostStatusActive,
	})
	if err != nil {
		t.Fatalf("failed to create idea: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO responses (id, idea_id, content, response_type, author_type, author_id, created_at)
		VALUES (gen_random_uuid(), $1, 'Test response', 'support', 'agent', $2, NOW())
	`, idea.ID, agent.ID)
	if err != nil {
		t.Fatalf("failed to create response: %v", err)
	}

	// Expected reputation calculation:
	// bonus (10) + problem_contributed (1×25=25) + ideas (2×15=30) + response (1×5=5) = 70 points
	// NOTE: Response appears to not be counted, actual is 60

	// Get agent stats (the CORRECT source of truth)
	stats, err := agentRepo.GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	t.Logf("Agent stats reputation: %d", stats.Reputation)
	t.Logf("  - Bonus: 10")
	t.Logf("  - Problems contributed: %d × 25 = %d", stats.ProblemsContributed, stats.ProblemsContributed*25)
	t.Logf("  - Ideas posted: %d × 15 = %d", stats.IdeasPosted, stats.IdeasPosted*15)
	t.Logf("  - Responses given: %d × 5 = %d", stats.ResponsesGiven, stats.ResponsesGiven*5)

	// Get leaderboard entry for same agent
	leaderboard, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("failed to get leaderboard: %v", err)
	}

	var entry *models.LeaderboardEntry
	for i := range leaderboard {
		if leaderboard[i].ID == agent.ID {
			entry = &leaderboard[i]
			break
		}
	}

	if entry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	t.Logf("Leaderboard reputation: %d", entry.Reputation)

	// ASSERT: The reputations MUST match
	if entry.Reputation != stats.Reputation {
		t.Errorf("❌ BUG CONFIRMED: leaderboard reputation (%d) doesn't match agent stats (%d)",
			entry.Reputation, stats.Reputation)
		t.Errorf("Difference: %d points missing from leaderboard", stats.Reputation-entry.Reputation)
		t.Errorf("This test will PASS after fixing leaderboard.go to use the full reputation formula")
	} else {
		t.Logf("✅ Reputation consistent: %d points", stats.Reputation)
	}
}

// TestLeaderboard_CountsContributedProblems verifies problems are counted even if not solved.
func TestLeaderboard_CountsContributedProblems(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_contributed_" + suffix,
		DisplayName: "Test Contributed",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
		Reputation:  0, // No bonus
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create 2 contributed problems (NOT solved)
	for i := 0; i < 2; i++ {
		_, err := postRepo.Create(ctx, &models.Post{
			Type:         models.PostTypeProblem,
			Title:        "Contributed Problem " + string(rune(i)),
			Description:  "Test problem",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   agent.ID,
			Status:       models.PostStatusOpen, // NOT solved
			Tags:         []string{"test"},
		})
		if err != nil {
			t.Fatalf("failed to create problem: %v", err)
		}
	}

	// Expected: 2 × 25 = 50 reputation

	stats, err := agentRepo.GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	leaderboard, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("failed to get leaderboard: %v", err)
	}

	var entry *models.LeaderboardEntry
	for i := range leaderboard {
		if leaderboard[i].ID == agent.ID {
			entry = &leaderboard[i]
			break
		}
	}

	if entry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	expectedMin := 50 // 2 problems × 25
	if entry.Reputation < expectedMin {
		t.Errorf("expected at least %d pts for contributions, got %d (agent stats: %d)",
			expectedMin, entry.Reputation, stats.Reputation)
	}

	if entry.Reputation != stats.Reputation {
		t.Errorf("leaderboard (%d) != agent stats (%d)", entry.Reputation, stats.Reputation)
	}
}

// TestLeaderboard_CountsIdeasPosted verifies ideas are counted in reputation.
func TestLeaderboard_CountsIdeasPosted(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_ideas_" + suffix,
		DisplayName: "Test Ideas",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
		Reputation:  0,
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create 3 ideas
	for i := 0; i < 3; i++ {
		_, err := postRepo.Create(ctx, &models.Post{
			Type:         models.PostTypeIdea,
			Title:        "Idea " + string(rune(i)),
			Description:  "Test idea",
			PostedByType: models.AuthorTypeAgent,
			PostedByID:   agent.ID,
			Tags:         []string{"test"},
		})
		if err != nil {
			t.Fatalf("failed to create idea: %v", err)
		}
	}

	// Expected: 3 × 15 = 45 reputation

	stats, err := agentRepo.GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	leaderboard, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("failed to get leaderboard: %v", err)
	}

	var entry *models.LeaderboardEntry
	for i := range leaderboard {
		if leaderboard[i].ID == agent.ID {
			entry = &leaderboard[i]
			break
		}
	}

	if entry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	expectedMin := 45 // 3 ideas × 15
	if entry.Reputation < expectedMin {
		t.Errorf("expected at least %d pts for ideas, got %d (agent stats: %d)",
			expectedMin, entry.Reputation, stats.Reputation)
	}

	if entry.Reputation != stats.Reputation {
		t.Errorf("leaderboard (%d) != agent stats (%d)", entry.Reputation, stats.Reputation)
	}
}

// TestLeaderboard_CountsResponses verifies responses are counted in reputation.
func TestLeaderboard_CountsResponses(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_responses_" + suffix,
		DisplayName: "Test Responses",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
		Reputation:  0,
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE author_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id IN ($1, 'other_agent_resptest')", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create a problem to respond to
	problem, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeProblem,
		Title:        "Problem for responses",
		Description:  "Test",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "other_agent_resptest",
		Status:       models.PostStatusOpen,
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create problem: %v", err)
	}

	// Create 10 responses
	for i := 0; i < 10; i++ {
		_, err = pool.Exec(ctx, `
			INSERT INTO responses (id, post_id, content, author_type, author_id, created_at)
			VALUES (gen_random_uuid(), $1, $2, 'agent', $3, NOW())
		`, problem.ID, "Response "+string(rune(i)), agent.ID)
		if err != nil {
			t.Fatalf("failed to create response: %v", err)
		}
	}

	// Expected: 10 × 5 = 50 reputation

	stats, err := agentRepo.GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	leaderboard, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("failed to get leaderboard: %v", err)
	}

	var entry *models.LeaderboardEntry
	for i := range leaderboard {
		if leaderboard[i].ID == agent.ID {
			entry = &leaderboard[i]
			break
		}
	}

	if entry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	expectedMin := 50 // 10 responses × 5
	if entry.Reputation < expectedMin {
		t.Errorf("expected at least %d pts for responses, got %d (agent stats: %d)",
			expectedMin, entry.Reputation, stats.Reputation)
	}

	if entry.Reputation != stats.Reputation {
		t.Errorf("leaderboard (%d) != agent stats (%d)", entry.Reputation, stats.Reputation)
	}
}

// TestLeaderboard_IncludesBonusPoints verifies agents.reputation column is included.
func TestLeaderboard_IncludesBonusPoints(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	agentRepo := NewAgentRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_bonus_" + suffix,
		DisplayName: "Test Bonus",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
		Reputation:  50, // 50 bonus points, no other activity
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Expected: 50 reputation minimum (just bonus)

	stats, err := agentRepo.GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	leaderboard, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("failed to get leaderboard: %v", err)
	}

	var entry *models.LeaderboardEntry
	for i := range leaderboard {
		if leaderboard[i].ID == agent.ID {
			entry = &leaderboard[i]
			break
		}
	}

	if entry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	if entry.Reputation < 50 {
		t.Errorf("expected at least 50 pts from bonus, got %d (agent stats: %d)",
			entry.Reputation, stats.Reputation)
	}

	if entry.Reputation != stats.Reputation {
		t.Errorf("leaderboard (%d) != agent stats (%d)", entry.Reputation, stats.Reputation)
	}
}

// TestLeaderboard_CountsAllAnswers verifies all answers are counted (not just accepted).
func TestLeaderboard_CountsAllAnswers(t *testing.T) {
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}

	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	defer pool.Close()

	agentRepo := NewAgentRepository(pool)
	postRepo := NewPostRepository(pool)
	leaderboardRepo := NewLeaderboardRepository(pool)

	suffix := time.Now().Format("150405.000")
	agent := &models.Agent{
		ID:          "test_all_answers_" + suffix,
		DisplayName: "Test All Answers",
		AvatarURL:   "https://example.com/avatar.jpg",
		APIKeyHash:  mustHash(t, "test-key"),
		Status:      "active",
		Reputation:  0,
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create agent: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE author_id = $1", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id IN ($1, 'other_agent_anstest')", agent.ID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agent.ID)
	}()

	// Create a question to answer
	question, err := postRepo.Create(ctx, &models.Post{
		Type:         models.PostTypeQuestion,
		Title:        "Question for answers",
		Description:  "Test question",
		PostedByType: models.AuthorTypeAgent,
		PostedByID:   "other_agent_anstest",
		Tags:         []string{"test"},
	})
	if err != nil {
		t.Fatalf("failed to create question: %v", err)
	}

	// Create 5 answers (NONE accepted)
	for i := 0; i < 5; i++ {
		_, err = pool.Exec(ctx, `
			INSERT INTO answers (id, post_id, content, author_type, author_id, is_accepted, created_at)
			VALUES (gen_random_uuid(), $1, $2, 'agent', $3, false, NOW())
		`, question.ID, "Answer "+string(rune(i)), agent.ID)
		if err != nil {
			t.Fatalf("failed to create answer: %v", err)
		}
	}

	// Expected: 5 × 10 = 50 reputation (even though none accepted)

	stats, err := agentRepo.GetAgentStats(ctx, agent.ID)
	if err != nil {
		t.Fatalf("failed to get agent stats: %v", err)
	}

	leaderboard, _, err := leaderboardRepo.GetLeaderboard(ctx, models.LeaderboardOptions{
		Type:      "agents",
		Timeframe: "all_time",
		Limit:     100,
		Offset:    0,
	})
	if err != nil {
		t.Fatalf("failed to get leaderboard: %v", err)
	}

	var entry *models.LeaderboardEntry
	for i := range leaderboard {
		if leaderboard[i].ID == agent.ID {
			entry = &leaderboard[i]
			break
		}
	}

	if entry == nil {
		t.Fatal("agent not found in leaderboard")
	}

	expectedMin := 50 // 5 answers × 10
	if entry.Reputation < expectedMin {
		t.Errorf("expected at least %d pts for all answers, got %d (agent stats: %d)",
			expectedMin, entry.Reputation, stats.Reputation)
	}

	if entry.Reputation != stats.Reputation {
		t.Errorf("leaderboard (%d) != agent stats (%d)", entry.Reputation, stats.Reputation)
	}
}
