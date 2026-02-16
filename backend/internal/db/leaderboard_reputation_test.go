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

	// Verify reputation is exactly 100
	if agentEntry.Reputation != 100 {
		t.Errorf("expected reputation 100 for solved post, got %d (BUG: using stale agents.reputation)", agentEntry.Reputation)
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
			INSERT INTO votes (target_type, target_id, direction, voter_type, confirmed)
			VALUES ('post', $1, 'up', 'anonymous', true)
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
