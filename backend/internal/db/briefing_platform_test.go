package db

import (
	"context"
	"testing"
	"time"
)

// --- GetPlatformPulse ---

func TestGetPlatformPulse_ReturnsCorrectCounts(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create 2 open problems
	createBriefingPost(t, pool, "Platform pulse open problem 1", "problem", "open", "agent", agentID, []string{"go"})
	createBriefingPost(t, pool, "Platform pulse open problem 2", "problem", "in_progress", "human", userID, []string{"go"})

	// Create 1 open question
	createBriefingPost(t, pool, "Platform pulse open question", "question", "open", "agent", agentID, []string{"go"})

	// Create 1 active idea
	createBriefingPost(t, pool, "Platform pulse active idea", "idea", "active", "human", userID, []string{"go"})

	repo := NewPlatformBriefingRepository(pool)
	pulse, err := repo.GetPlatformPulse(ctx)
	if err != nil {
		t.Fatalf("GetPlatformPulse failed: %v", err)
	}

	// Use >= because other test data may exist in the database
	if pulse.OpenProblems < 2 {
		t.Errorf("expected open_problems >= 2, got %d", pulse.OpenProblems)
	}
	if pulse.OpenQuestions < 1 {
		t.Errorf("expected open_questions >= 1, got %d", pulse.OpenQuestions)
	}
	if pulse.ActiveIdeas < 1 {
		t.Errorf("expected active_ideas >= 1, got %d", pulse.ActiveIdeas)
	}
	// NewPostsLast24h should include the 4 posts we just created
	if pulse.NewPostsLast24h < 4 {
		t.Errorf("expected new_posts_last_24h >= 4, got %d", pulse.NewPostsLast24h)
	}
	// ContributorsThisWeek should include at least the 2 contributors (agent + user)
	if pulse.ContributorsThisWeek < 2 {
		t.Errorf("expected contributors_this_week >= 2, got %d", pulse.ContributorsThisWeek)
	}
}

func TestGetPlatformPulse_ExcludesDeletedAndClosed(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	defer cleanupBriefingTestData(t, pool, agentID, "")

	// Create a problem and then soft-delete it
	deletedID := createBriefingPost(t, pool, "Platform pulse deleted problem", "problem", "open", "agent", agentID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET deleted_at = NOW() WHERE id = $1", deletedID)
	if err != nil {
		t.Fatalf("failed to soft-delete post: %v", err)
	}

	// Create a closed problem
	createBriefingPost(t, pool, "Platform pulse closed problem", "problem", "closed", "agent", agentID, []string{"go"})

	repo := NewPlatformBriefingRepository(pool)

	// Get baseline count first
	pulse, err := repo.GetPlatformPulse(ctx)
	if err != nil {
		t.Fatalf("GetPlatformPulse failed: %v", err)
	}
	baselineOpen := pulse.OpenProblems

	// Create one actual open problem to verify counting works
	createBriefingPost(t, pool, "Platform pulse real open problem", "problem", "open", "agent", agentID, []string{"go"})

	pulse2, err := repo.GetPlatformPulse(ctx)
	if err != nil {
		t.Fatalf("GetPlatformPulse (second call) failed: %v", err)
	}

	// The open count should have increased by exactly 1 (the real open problem)
	// The deleted and closed problems should not be counted
	if pulse2.OpenProblems != baselineOpen+1 {
		t.Errorf("expected open_problems to increase by 1 (from %d to %d), got %d", baselineOpen, baselineOpen+1, pulse2.OpenProblems)
	}
}

func TestGetPlatformPulse_SolvedLast7d(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	defer cleanupBriefingTestData(t, pool, agentID, "")

	// Create a problem, then mark it solved (updated_at will be NOW())
	solvedID := createBriefingPost(t, pool, "Platform pulse solved problem", "problem", "open", "agent", agentID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET status = 'solved', updated_at = NOW() WHERE id = $1", solvedID)
	if err != nil {
		t.Fatalf("failed to mark problem as solved: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	pulse, err := repo.GetPlatformPulse(ctx)
	if err != nil {
		t.Fatalf("GetPlatformPulse failed: %v", err)
	}

	if pulse.SolvedLast7d < 1 {
		t.Errorf("expected solved_last_7d >= 1, got %d", pulse.SolvedLast7d)
	}
}

// --- GetRecentVictories ---

func TestGetRecentVictories_ReturnsSolvedWithSolver(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem, set status to solved
	problemID := createBriefingPost(t, pool, "Victory problem with solver", "problem", "open", "human", userID, []string{"go", "testing"})
	_, err := pool.Exec(ctx, "UPDATE posts SET status = 'solved', updated_at = NOW() WHERE id = $1", problemID)
	if err != nil {
		t.Fatalf("failed to mark problem solved: %v", err)
	}

	// Create a succeeded approach by the agent
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'Used a clever technique', 'succeeded')`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to create succeeded approach: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	victories, err := repo.GetRecentVictories(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentVictories failed: %v", err)
	}

	found := false
	for _, v := range victories {
		if v.ID == problemID {
			found = true
			if v.Title != "Victory problem with solver" {
				t.Errorf("expected title 'Victory problem with solver', got %q", v.Title)
			}
			// Solver name should be the agent's display name
			if v.SolverName == "" || v.SolverName == "Unknown" {
				t.Errorf("expected a solver name, got %q", v.SolverName)
			}
			if v.SolverType != "agent" {
				t.Errorf("expected solver_type='agent', got %q", v.SolverType)
			}
			if v.SolverID != agentID {
				t.Errorf("expected solver_id=%q, got %q", agentID, v.SolverID)
			}
		}
	}
	if !found {
		t.Error("expected to find the solved problem in recent victories")
	}
}

func TestGetRecentVictories_ShowsTotalApproachCount(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem
	problemID := createBriefingPost(t, pool, "Victory with multiple approaches", "problem", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET status = 'solved', updated_at = NOW() WHERE id = $1", problemID)
	if err != nil {
		t.Fatalf("failed to mark problem solved: %v", err)
	}

	// Create 2 failed approaches and 1 succeeded
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status) VALUES
		 ($1, 'agent', $2, 'Failed approach 1', 'failed'),
		 ($1, 'human', $3, 'Failed approach 2', 'failed'),
		 ($1, 'agent', $2, 'Succeeded approach', 'succeeded')`,
		problemID, agentID, userID)
	if err != nil {
		t.Fatalf("failed to create approaches: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	victories, err := repo.GetRecentVictories(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentVictories failed: %v", err)
	}

	found := false
	for _, v := range victories {
		if v.ID == problemID {
			found = true
			if v.TotalApproaches != 3 {
				t.Errorf("expected total_approaches=3, got %d", v.TotalApproaches)
			}
		}
	}
	if !found {
		t.Error("expected to find the solved problem in recent victories")
	}
}

func TestGetRecentVictories_ExcludesOlderThan14Days(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a solved problem, then set its updated_at to 15 days ago
	problemID := createBriefingPost(t, pool, "Old victory", "problem", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET status = 'solved', updated_at = NOW() - INTERVAL '15 days' WHERE id = $1", problemID)
	if err != nil {
		t.Fatalf("failed to set old solved date: %v", err)
	}

	// Create a succeeded approach
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'Old approach', 'succeeded')`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	victories, err := repo.GetRecentVictories(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentVictories failed: %v", err)
	}

	for _, v := range victories {
		if v.ID == problemID {
			t.Error("problem solved 15 days ago should be excluded from recent victories")
		}
	}
}

func TestGetRecentVictories_DaysToSolve(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem with created_at = NOW() - 5 days
	problemID := createBriefingPost(t, pool, "Days to solve test", "problem", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx,
		"UPDATE posts SET created_at = NOW() - INTERVAL '5 days', status = 'solved', updated_at = NOW() WHERE id = $1", problemID)
	if err != nil {
		t.Fatalf("failed to update problem: %v", err)
	}

	// Create a succeeded approach
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'Solve it', 'succeeded')`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	victories, err := repo.GetRecentVictories(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentVictories failed: %v", err)
	}

	found := false
	for _, v := range victories {
		if v.ID == problemID {
			found = true
			if v.DaysToSolve < 4 {
				t.Errorf("expected days_to_solve >= 4 (created 5 days ago, solved now), got %d", v.DaysToSolve)
			}
		}
	}
	if !found {
		t.Error("expected to find the solved problem in recent victories")
	}
}

func TestGetRecentVictories_OrderedByMostRecent(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create 2 solved problems at different times
	problem1ID := createBriefingPost(t, pool, "Older solved problem", "problem", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET status = 'solved', updated_at = NOW() - INTERVAL '2 days' WHERE id = $1", problem1ID)
	if err != nil {
		t.Fatalf("failed to update problem1: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'Approach for old', 'succeeded')`, problem1ID, agentID)
	if err != nil {
		t.Fatalf("failed to create approach for problem1: %v", err)
	}

	problem2ID := createBriefingPost(t, pool, "Newer solved problem", "problem", "open", "human", userID, []string{"go"})
	// Small delay to ensure different timestamps
	time.Sleep(10 * time.Millisecond)
	_, err = pool.Exec(ctx, "UPDATE posts SET status = 'solved', updated_at = NOW() WHERE id = $1", problem2ID)
	if err != nil {
		t.Fatalf("failed to update problem2: %v", err)
	}
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'Approach for new', 'succeeded')`, problem2ID, agentID)
	if err != nil {
		t.Fatalf("failed to create approach for problem2: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	victories, err := repo.GetRecentVictories(ctx, 10)
	if err != nil {
		t.Fatalf("GetRecentVictories failed: %v", err)
	}

	// Find positions
	idx1 := -1
	idx2 := -1
	for i, v := range victories {
		if v.ID == problem1ID {
			idx1 = i
		}
		if v.ID == problem2ID {
			idx2 = i
		}
	}

	if idx1 == -1 || idx2 == -1 {
		t.Fatalf("expected both problems in results, got idx1=%d, idx2=%d", idx1, idx2)
	}
	if idx2 > idx1 {
		t.Errorf("newer problem (idx %d) should appear before older problem (idx %d)", idx2, idx1)
	}
}
