package db

import (
	"context"
	"testing"
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
