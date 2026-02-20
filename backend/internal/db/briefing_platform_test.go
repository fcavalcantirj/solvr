package db

import (
	"context"
	"fmt"
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

// --- GetTrendingNow ---

// helper: create a confirmed vote on a post within the last 7 days
func createRecentVote(t *testing.T, pool *Pool, postID, voterID string, voterType string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed, created_at)
		 VALUES ('post', $1, $2, $3, 'up', true, NOW())`,
		postID, voterType, voterID)
	if err != nil {
		t.Fatalf("failed to create vote on post %s: %v", postID, err)
	}
	// Keep denormalized counter in sync
	_, err = pool.Exec(ctx, `UPDATE posts SET upvotes = upvotes + 1 WHERE id = $1`, postID)
	if err != nil {
		t.Fatalf("failed to increment upvotes for post %s: %v", postID, err)
	}
}

// helper: create a post view within the last 7 days
func createRecentView(t *testing.T, pool *Pool, postID, viewerID string) {
	t.Helper()
	ctx := context.Background()
	_, err := pool.Exec(ctx,
		`INSERT INTO post_views (post_id, viewer_type, viewer_id, viewed_at)
		 VALUES ($1, 'agent', $2, NOW())`,
		postID, viewerID)
	if err != nil {
		t.Fatalf("failed to create view on post %s: %v", postID, err)
	}
	// Keep denormalized counter in sync
	_, err = pool.Exec(ctx, `UPDATE posts SET view_count = view_count + 1 WHERE id = $1`, postID)
	if err != nil {
		t.Fatalf("failed to increment view_count for post %s: %v", postID, err)
	}
}

func TestGetTrendingNow_RankedByEngagement(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create 3 posts by the user (not the agent, so they won't be excluded)
	postA := createBriefingPost(t, pool, "Trending post A high engagement", "problem", "open", "human", userID, []string{"go"})
	postB := createBriefingPost(t, pool, "Trending post B medium engagement", "question", "open", "human", userID, []string{"go"})
	postC := createBriefingPost(t, pool, "Trending post C zero engagement", "idea", "open", "human", userID, []string{"go"})

	// Post A: 3 votes + 2 views = engagement 5
	createRecentVote(t, pool, postA, "voter_a1", "agent")
	createRecentVote(t, pool, postA, "voter_a2", "agent")
	createRecentVote(t, pool, postA, "voter_a3", "agent")
	createRecentView(t, pool, postA, "viewer_a1")
	createRecentView(t, pool, postA, "viewer_a2")

	// Post B: 1 vote = engagement 1
	createRecentVote(t, pool, postB, "voter_b1", "agent")

	// Post C: 0 engagement

	repo := NewPlatformBriefingRepository(pool)
	trending, err := repo.GetTrendingNow(ctx, agentID, 10)
	if err != nil {
		t.Fatalf("GetTrendingNow failed: %v", err)
	}

	// Find positions of our posts
	idxA, idxB := -1, -1
	for i, p := range trending {
		if p.ID == postA {
			idxA = i
		}
		if p.ID == postB {
			idxB = i
		}
	}

	if idxA == -1 {
		t.Fatal("expected postA in trending results")
	}
	if idxB == -1 {
		t.Fatal("expected postB in trending results")
	}
	if idxA >= idxB {
		t.Errorf("postA (engagement=5) should rank before postB (engagement=1), got idxA=%d, idxB=%d", idxA, idxB)
	}

	// Verify postA fields
	for _, p := range trending {
		if p.ID == postA {
			if p.Type != "problem" {
				t.Errorf("expected type='problem', got %q", p.Type)
			}
			if p.VoteScore < 3 {
				t.Errorf("expected vote_score >= 3, got %d", p.VoteScore)
			}
			if p.ViewCount < 2 {
				t.Errorf("expected view_count >= 2, got %d", p.ViewCount)
			}
			if p.AuthorType != "human" {
				t.Errorf("expected author_type='human', got %q", p.AuthorType)
			}
			if p.AuthorName == "" {
				t.Error("expected non-empty author_name")
			}
		}
	}

	// PostC may or may not appear (0 engagement), but if present should be after A and B
	_ = postC
}

func TestGetTrendingNow_ExcludesAgentOwnPosts(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a highly-voted post BY the agent
	agentPost := createBriefingPost(t, pool, "Agent own post should be excluded", "problem", "open", "agent", agentID, []string{"go"})
	createRecentVote(t, pool, agentPost, "voter_x1", "agent")
	createRecentVote(t, pool, agentPost, "voter_x2", "agent")
	createRecentVote(t, pool, agentPost, "voter_x3", "agent")

	repo := NewPlatformBriefingRepository(pool)
	trending, err := repo.GetTrendingNow(ctx, agentID, 50)
	if err != nil {
		t.Fatalf("GetTrendingNow failed: %v", err)
	}

	for _, p := range trending {
		if p.ID == agentPost {
			t.Error("agent's own post should be excluded from trending_now results")
		}
	}
}

func TestGetTrendingNow_ExcludesDraftAndClosed(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a draft post with votes
	draftPost := createBriefingPost(t, pool, "Draft post with votes", "problem", "draft", "human", userID, []string{"go"})
	createRecentVote(t, pool, draftPost, "voter_d1", "agent")
	createRecentVote(t, pool, draftPost, "voter_d2", "agent")

	// Create a closed post with votes
	closedPost := createBriefingPost(t, pool, "Closed post with votes", "question", "closed", "human", userID, []string{"go"})
	createRecentVote(t, pool, closedPost, "voter_c1", "agent")
	createRecentVote(t, pool, closedPost, "voter_c2", "agent")

	repo := NewPlatformBriefingRepository(pool)
	trending, err := repo.GetTrendingNow(ctx, agentID, 50)
	if err != nil {
		t.Fatalf("GetTrendingNow failed: %v", err)
	}

	for _, p := range trending {
		if p.ID == draftPost {
			t.Error("draft post should be excluded from trending results")
		}
		if p.ID == closedPost {
			t.Error("closed post should be excluded from trending results")
		}
	}
}

func TestGetTrendingNow_MixOfTypes(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create one of each type with engagement
	problemPost := createBriefingPost(t, pool, "Trending problem mix test", "problem", "open", "human", userID, []string{"go"})
	questionPost := createBriefingPost(t, pool, "Trending question mix test", "question", "open", "human", userID, []string{"go"})
	ideaPost := createBriefingPost(t, pool, "Trending idea mix test", "idea", "open", "human", userID, []string{"go"})

	createRecentVote(t, pool, problemPost, "voter_mix1", "agent")
	createRecentVote(t, pool, questionPost, "voter_mix2", "agent")
	createRecentVote(t, pool, ideaPost, "voter_mix3", "agent")

	repo := NewPlatformBriefingRepository(pool)
	trending, err := repo.GetTrendingNow(ctx, agentID, 50)
	if err != nil {
		t.Fatalf("GetTrendingNow failed: %v", err)
	}

	typesFound := map[string]bool{}
	for _, p := range trending {
		if p.ID == problemPost || p.ID == questionPost || p.ID == ideaPost {
			typesFound[p.Type] = true
		}
	}

	if !typesFound["problem"] {
		t.Error("expected problem type in trending results")
	}
	if !typesFound["question"] {
		t.Error("expected question type in trending results")
	}
	if !typesFound["idea"] {
		t.Error("expected idea type in trending results")
	}
}

func TestGetTrendingNow_LimitRespected(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create 10 posts with engagement
	for i := 0; i < 10; i++ {
		postID := createBriefingPost(t, pool, fmt.Sprintf("Limit test post %d", i), "problem", "open", "human", userID, []string{"go"})
		voterID := fmt.Sprintf("voter_limit_%d", i)
		createRecentVote(t, pool, postID, voterID, "agent")
	}

	repo := NewPlatformBriefingRepository(pool)
	trending, err := repo.GetTrendingNow(ctx, agentID, 5)
	if err != nil {
		t.Fatalf("GetTrendingNow failed: %v", err)
	}

	if len(trending) > 5 {
		t.Errorf("expected at most 5 results with limit=5, got %d", len(trending))
	}
}

// --- GetHardcoreUnsolved ---

// helper: create failed approaches on a problem
func createFailedApproaches(t *testing.T, pool *Pool, problemID string, count int, authorType, authorID string) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := pool.Exec(ctx,
			`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
			 VALUES ($1, $2, $3, $4, 'failed')`,
			problemID, authorType, authorID, fmt.Sprintf("Failed approach %d", i))
		if err != nil {
			t.Fatalf("failed to create approach %d: %v", i, err)
		}
	}
}

func TestGetHardcoreUnsolved_ManyFailedApproaches(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem with 3 failed approaches (qualifies: failed_count >= 2)
	problemID := createBriefingPost(t, pool, "Hardcore problem many failures", "problem", "open", "human", userID, []string{"go"})
	createFailedApproaches(t, pool, problemID, 3, "agent", agentID)

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetHardcoreUnsolved(ctx, 10)
	if err != nil {
		t.Fatalf("GetHardcoreUnsolved failed: %v", err)
	}

	found := false
	for _, h := range results {
		if h.ID == problemID {
			found = true
			if h.FailedCount != 3 {
				t.Errorf("expected failed_count=3, got %d", h.FailedCount)
			}
			if h.TotalApproaches != 3 {
				t.Errorf("expected total_approaches=3, got %d", h.TotalApproaches)
			}
			if h.DifficultyScore <= 0 {
				t.Errorf("expected positive difficulty_score, got %f", h.DifficultyScore)
			}
		}
	}
	if !found {
		t.Error("expected problem with 3 failed approaches in hardcore_unsolved results")
	}
}

func TestGetHardcoreUnsolved_HighWeight(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem with weight=5 and age > 1 day (qualifies: weight >= 4)
	problemID := createBriefingPost(t, pool, "Hardcore high weight problem", "problem", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET weight = 5, created_at = NOW() - INTERVAL '2 days' WHERE id = $1", problemID)
	if err != nil {
		t.Fatalf("failed to update weight: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetHardcoreUnsolved(ctx, 10)
	if err != nil {
		t.Fatalf("GetHardcoreUnsolved failed: %v", err)
	}

	found := false
	for _, h := range results {
		if h.ID == problemID {
			found = true
			if h.Weight != 5 {
				t.Errorf("expected weight=5, got %d", h.Weight)
			}
		}
	}
	if !found {
		t.Error("expected high-weight problem in hardcore_unsolved results")
	}
}

func TestGetHardcoreUnsolved_ExcludesSolvedAndClosed(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a solved problem with many failed approaches
	solvedProblem := createBriefingPost(t, pool, "Solved hardcore problem", "problem", "open", "human", userID, []string{"go"})
	createFailedApproaches(t, pool, solvedProblem, 5, "agent", agentID)
	_, err := pool.Exec(ctx, "UPDATE posts SET status = 'solved' WHERE id = $1", solvedProblem)
	if err != nil {
		t.Fatalf("failed to mark solved: %v", err)
	}

	// Create a closed problem with many failed approaches
	closedProblem := createBriefingPost(t, pool, "Closed hardcore problem", "problem", "open", "human", userID, []string{"go"})
	createFailedApproaches(t, pool, closedProblem, 5, "agent", agentID)
	_, err = pool.Exec(ctx, "UPDATE posts SET status = 'closed' WHERE id = $1", closedProblem)
	if err != nil {
		t.Fatalf("failed to mark closed: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetHardcoreUnsolved(ctx, 50)
	if err != nil {
		t.Fatalf("GetHardcoreUnsolved failed: %v", err)
	}

	for _, h := range results {
		if h.ID == solvedProblem {
			t.Error("solved problem should be excluded from hardcore_unsolved")
		}
		if h.ID == closedProblem {
			t.Error("closed problem should be excluded from hardcore_unsolved")
		}
	}
}

func TestGetHardcoreUnsolved_DifficultyScoreOrdering(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Problem with 5 failed approaches (higher difficulty)
	hardProblem := createBriefingPost(t, pool, "Very hard problem", "problem", "open", "human", userID, []string{"go"})
	createFailedApproaches(t, pool, hardProblem, 5, "agent", agentID)

	// Problem with 2 failed approaches (lower difficulty)
	easyProblem := createBriefingPost(t, pool, "Less hard problem", "problem", "open", "human", userID, []string{"go"})
	createFailedApproaches(t, pool, easyProblem, 2, "agent", agentID)

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetHardcoreUnsolved(ctx, 50)
	if err != nil {
		t.Fatalf("GetHardcoreUnsolved failed: %v", err)
	}

	idxHard, idxEasy := -1, -1
	for i, h := range results {
		if h.ID == hardProblem {
			idxHard = i
		}
		if h.ID == easyProblem {
			idxEasy = i
		}
	}

	if idxHard == -1 {
		t.Fatal("expected hard problem in results")
	}
	if idxEasy == -1 {
		t.Fatal("expected easy problem in results")
	}
	if idxHard >= idxEasy {
		t.Errorf("harder problem (5 failures, idx=%d) should rank before easier (2 failures, idx=%d)", idxHard, idxEasy)
	}
}

func TestGetHardcoreUnsolved_NoQualifyingProblems(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a simple problem with 0 approaches (doesn't qualify)
	createBriefingPost(t, pool, "Simple easy problem", "problem", "open", "human", userID, []string{"go"})

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetHardcoreUnsolved(ctx, 10)
	if err != nil {
		t.Fatalf("GetHardcoreUnsolved should not error for no results: %v", err)
	}

	// Results may include other test data, but our simple problem should NOT be here
	for _, h := range results {
		if h.Title == "Simple easy problem" {
			t.Error("simple problem with 0 approaches should not appear in hardcore_unsolved")
		}
	}
}

// Rising Ideas tests are in briefing_rising_ideas_test.go
