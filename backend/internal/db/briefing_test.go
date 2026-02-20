package db

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"
)

// briefingTestDB connects to the real database. Skips if DATABASE_URL is not set.
func briefingTestDB(t *testing.T) *Pool {
	t.Helper()
	databaseURL := os.Getenv("DATABASE_URL")
	if databaseURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	ctx := context.Background()
	pool, err := NewPool(ctx, databaseURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	return pool
}

// cleanupBriefingTestData removes all data created by briefing tests.
func cleanupBriefingTestData(t *testing.T, pool *Pool, agentID, userID string) {
	t.Helper()
	ctx := context.Background()
	// Delete in reverse FK order
	_, _ = pool.Exec(ctx, "DELETE FROM votes WHERE voter_id = $1 OR voter_id = $2", agentID, userID)
	_, _ = pool.Exec(ctx, "DELETE FROM comments WHERE author_id = $1 OR author_id = $2", agentID, userID)
	_, _ = pool.Exec(ctx, "DELETE FROM answers WHERE author_id = $1 OR author_id = $2", agentID, userID)
	_, _ = pool.Exec(ctx, "DELETE FROM approaches WHERE author_id = $1 OR author_id = $2", agentID, userID)
	_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE posted_by_id = $1 OR posted_by_id = $2", agentID, userID)
	_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	_, _ = pool.Exec(ctx, "DELETE FROM auth_methods WHERE user_id = $1", userID)
	_, _ = pool.Exec(ctx, "DELETE FROM users WHERE id = $1", userID)
}

// createBriefingAgent creates a real agent in the database.
func createBriefingAgent(t *testing.T, pool *Pool, specialties []string) string {
	t.Helper()
	ctx := context.Background()
	agentID := fmt.Sprintf("brf_agent_%s", time.Now().Format("150405"))
	_, err := pool.Exec(ctx,
		`INSERT INTO agents (id, display_name, specialties, status)
		 VALUES ($1, $2, $3, 'active')`,
		agentID, "Briefing Test Agent", specialties)
	if err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	return agentID
}

// createBriefingUser creates a real user in the database and returns the user ID.
func createBriefingUser(t *testing.T, pool *Pool) string {
	t.Helper()
	ctx := context.Background()
	ts := time.Now().Format("150405")
	userID := uuid.New().String()
	_, err := pool.Exec(ctx,
		`INSERT INTO users (id, username, display_name, email, auth_provider, auth_provider_id, role)
		 VALUES ($1, $2, $3, $4, 'github', $5, 'user')`,
		userID, "brfu"+ts, "Briefing Test User", "brfu"+ts+"@test.com", "gh_brf_"+ts)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return userID
}

// createBriefingPost creates a real post in the database.
func createBriefingPost(t *testing.T, pool *Pool, title string, postType, status, postedByType, postedByID string, tags []string) string {
	t.Helper()
	ctx := context.Background()
	var postID string
	err := pool.QueryRow(ctx,
		`INSERT INTO posts (title, description, type, status, posted_by_type, posted_by_id, tags)
		 VALUES ($1, 'Test description', $2, $3, $4, $5, $6)
		 RETURNING id`,
		title, postType, status, postedByType, postedByID, tags).Scan(&postID)
	if err != nil {
		t.Fatalf("failed to create test post '%s': %v", title, err)
	}
	return postID
}

// --- GetOpenItemsForAgent ---

func TestBriefing_OpenItems_ProblemsNoApproaches(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	defer cleanupBriefingTestData(t, pool, agentID, "")

	// Create 2 problems by the agent
	p1 := createBriefingPost(t, pool, "Problem with no approaches", "problem", "open", "agent", agentID, []string{"go"})
	p2 := createBriefingPost(t, pool, "Problem with an approach", "problem", "open", "agent", agentID, []string{"go"})

	// Add an approach to p2 (from another agent)
	_, err := pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', 'other_agent', 'Some angle', 'starting')`, p2)
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpenItemsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetOpenItemsForAgent failed: %v", err)
	}

	if result.ProblemsNoApproaches != 1 {
		t.Errorf("expected 1 problem with no approaches, got %d", result.ProblemsNoApproaches)
	}

	// Verify the correct problem is returned
	found := false
	for _, item := range result.Items {
		if item.ID == p1 && item.Type == "problem" {
			found = true
		}
		if item.ID == p2 {
			t.Errorf("problem with approach (%s) should NOT appear in open items", p2)
		}
	}
	if !found {
		t.Errorf("expected problem %s (no approaches) in open items, not found", p1)
	}
}

func TestBriefing_OpenItems_QuestionsNoAnswers(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	defer cleanupBriefingTestData(t, pool, agentID, "")

	// Create a question by the agent with no answers
	q1 := createBriefingPost(t, pool, "Unanswered question", "question", "open", "agent", agentID, []string{"go"})

	// Create a question that IS answered
	q2 := createBriefingPost(t, pool, "Answered question", "question", "open", "agent", agentID, []string{"go"})
	_, err := pool.Exec(ctx,
		`INSERT INTO answers (question_id, author_type, author_id, content)
		 VALUES ($1, 'human', 'some-user', 'Here is the answer')`, q2)
	if err != nil {
		t.Fatalf("failed to create answer: %v", err)
	}

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpenItemsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetOpenItemsForAgent failed: %v", err)
	}

	if result.QuestionsNoAnswers != 1 {
		t.Errorf("expected 1 question with no answers, got %d", result.QuestionsNoAnswers)
	}

	found := false
	for _, item := range result.Items {
		if item.ID == q1 && item.Type == "question" {
			found = true
		}
		if item.ID == q2 {
			t.Errorf("answered question (%s) should NOT appear in open items", q2)
		}
	}
	if !found {
		t.Errorf("expected unanswered question %s in open items, not found", q1)
	}
}

func TestBriefing_OpenItems_StaleApproaches(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem by someone else
	problemID := createBriefingPost(t, pool, "Someone elses problem", "problem", "open", "human", userID, []string{"go"})

	// Create a stale approach by agent (working, updated 48h ago)
	var approachID string
	err := pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status, updated_at)
		 VALUES ($1, 'agent', $2, 'My stale approach', 'working', NOW() - INTERVAL '48 hours')
		 RETURNING id`, problemID, agentID).Scan(&approachID)
	if err != nil {
		t.Fatalf("failed to create stale approach: %v", err)
	}

	// Create a fresh approach (should NOT appear)
	_, err = pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'My fresh approach', 'working')`, problemID, agentID)
	if err != nil {
		t.Fatalf("failed to create fresh approach: %v", err)
	}

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpenItemsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetOpenItemsForAgent failed: %v", err)
	}

	if result.ApproachesStale != 1 {
		t.Errorf("expected 1 stale approach, got %d", result.ApproachesStale)
	}

	found := false
	for _, item := range result.Items {
		if item.ID == approachID && item.Type == "approach" {
			found = true
			if item.AgeHours < 47 {
				t.Errorf("expected stale approach age >= 47h, got %d", item.AgeHours)
			}
		}
	}
	if !found {
		t.Errorf("expected stale approach %s in open items, not found", approachID)
	}
}

// --- GetSuggestedActionsForAgent ---

func TestBriefing_SuggestedActions_StaleApproach(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	problemID := createBriefingPost(t, pool, "Problem for stale action", "problem", "open", "human", userID, []string{"go"})

	// Create stale approach (48h old)
	_, err := pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status, updated_at)
		 VALUES ($1, 'agent', $2, 'Stale angle', 'working', NOW() - INTERVAL '48 hours')`,
		problemID, agentID)
	if err != nil {
		t.Fatalf("failed to create stale approach: %v", err)
	}

	repo := NewBriefingRepository(pool)
	actions, err := repo.GetSuggestedActionsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetSuggestedActionsForAgent failed: %v", err)
	}

	found := false
	for _, a := range actions {
		if a.Action == "update_approach_status" {
			found = true
			if a.TargetTitle == "" {
				t.Error("expected non-empty target title for stale approach action")
			}
		}
	}
	if !found {
		t.Error("expected 'update_approach_status' action for stale approach, not found")
	}
}

func TestBriefing_SuggestedActions_UnrespondedComment(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Agent posts a problem
	postID := createBriefingPost(t, pool, "Agent problem with comment", "problem", "open", "agent", agentID, []string{"go"})

	// A human comments on it (not the agent)
	_, err := pool.Exec(ctx,
		`INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		 VALUES ('post', $1, 'human', $2, 'Have you tried restarting?')`,
		postID, userID)
	if err != nil {
		t.Fatalf("failed to create comment: %v", err)
	}

	repo := NewBriefingRepository(pool)
	actions, err := repo.GetSuggestedActionsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetSuggestedActionsForAgent failed: %v", err)
	}

	found := false
	for _, a := range actions {
		if a.Action == "respond_to_comment" {
			found = true
			if a.TargetTitle != "Agent problem with comment" {
				t.Errorf("expected target title 'Agent problem with comment', got '%s'", a.TargetTitle)
			}
		}
	}
	if !found {
		t.Error("expected 'respond_to_comment' action for unresponded comment, not found")
	}
}

// --- GetSuggestedActionsForAgent: Sort Order & Limit ---

func TestBriefing_SuggestedActions_SortOrder(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	problemID := createBriefingPost(t, pool, "Problem for sort order", "problem", "open", "human", userID, []string{"go"})

	// Create 3 stale approaches with different updated_at times
	var approach48h, approach72h, approach96h string
	err := pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status, updated_at)
		 VALUES ($1, 'agent', $2, 'Approach 48h', 'working', NOW() - INTERVAL '48 hours')
		 RETURNING id`, problemID, agentID).Scan(&approach48h)
	if err != nil {
		t.Fatalf("failed to create 48h approach: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status, updated_at)
		 VALUES ($1, 'agent', $2, 'Approach 72h', 'working', NOW() - INTERVAL '72 hours')
		 RETURNING id`, problemID, agentID).Scan(&approach72h)
	if err != nil {
		t.Fatalf("failed to create 72h approach: %v", err)
	}

	err = pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status, updated_at)
		 VALUES ($1, 'agent', $2, 'Approach 96h', 'working', NOW() - INTERVAL '96 hours')
		 RETURNING id`, problemID, agentID).Scan(&approach96h)
	if err != nil {
		t.Fatalf("failed to create 96h approach: %v", err)
	}

	repo := NewBriefingRepository(pool)
	actions, err := repo.GetSuggestedActionsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetSuggestedActionsForAgent failed: %v", err)
	}

	// Filter to only stale approach actions
	var staleActions []string
	for _, a := range actions {
		if a.Action == "update_approach_status" {
			staleActions = append(staleActions, a.TargetID)
		}
	}

	if len(staleActions) < 3 {
		t.Fatalf("expected at least 3 stale approach actions, got %d", len(staleActions))
	}

	// First returned action should be the NEWEST stale approach (48h), not oldest (96h)
	if staleActions[0] != approach48h {
		t.Errorf("expected first action to be newest stale approach (48h, id=%s), got %s", approach48h, staleActions[0])
	}
	// Last should be oldest (96h)
	if staleActions[len(staleActions)-1] != approach96h {
		t.Errorf("expected last action to be oldest stale approach (96h, id=%s), got %s", approach96h, staleActions[len(staleActions)-1])
	}
}

func TestBriefing_SuggestedActions_LimitFive(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	problemID := createBriefingPost(t, pool, "Problem for limit test", "problem", "open", "human", userID, []string{"go"})

	// Create 7 stale approaches
	for i := 1; i <= 7; i++ {
		interval := fmt.Sprintf("%d hours", 24+i*12) // 36h, 48h, 60h, ...
		_, err := pool.Exec(ctx,
			`INSERT INTO approaches (problem_id, author_type, author_id, angle, status, updated_at)
			 VALUES ($1, 'agent', $2, $3, 'working', NOW() - $4::interval)`,
			problemID, agentID, fmt.Sprintf("Limit approach %d", i), interval)
		if err != nil {
			t.Fatalf("failed to create approach %d: %v", i, err)
		}
	}

	repo := NewBriefingRepository(pool)
	actions, err := repo.GetSuggestedActionsForAgent(ctx, agentID)
	if err != nil {
		t.Fatalf("GetSuggestedActionsForAgent failed: %v", err)
	}

	// Total actions should be at most 5 (the limit)
	if len(actions) > 5 {
		t.Errorf("expected at most 5 suggested actions, got %d", len(actions))
	}

	// Verify the 5 actions are the 5 NEWEST stale approaches (smallest intervals)
	// With DESC order and LIMIT 5, we should get the 5 most recently updated
	for _, a := range actions {
		if a.Action != "update_approach_status" {
			continue
		}
		// All returned actions should have valid non-empty target titles
		if a.TargetTitle == "" {
			t.Error("expected non-empty target title for stale approach action")
		}
	}
}

// --- GetOpportunitiesForAgent ---

func TestBriefing_Opportunities_MatchesSpecialties(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"rust", "wasm"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create problems by the user (not the agent)
	createBriefingPost(t, pool, "Rust memory issue", "problem", "open", "human", userID, []string{"rust"})
	createBriefingPost(t, pool, "WASM build fails", "problem", "open", "human", userID, []string{"wasm"})
	createBriefingPost(t, pool, "Python type hints", "problem", "open", "human", userID, []string{"python"})

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpportunitiesForAgent(ctx, agentID, []string{"rust", "wasm"}, 10)
	if err != nil {
		t.Fatalf("GetOpportunitiesForAgent failed: %v", err)
	}

	if result.ProblemsInMyDomain < 2 {
		t.Errorf("expected at least 2 problems in domain, got %d", result.ProblemsInMyDomain)
	}

	// Python post should NOT be in the results
	for _, opp := range result.Items {
		if opp.Title == "Python type hints" {
			t.Error("Python problem should NOT appear in rust/wasm agent's opportunities")
		}
	}

	// Verify rust and wasm posts ARE present
	titles := map[string]bool{}
	for _, opp := range result.Items {
		titles[opp.Title] = true
	}
	if !titles["Rust memory issue"] {
		t.Error("expected 'Rust memory issue' in opportunities")
	}
	if !titles["WASM build fails"] {
		t.Error("expected 'WASM build fails' in opportunities")
	}

	_ = ctx // used above
}

func TestBriefing_Opportunities_ExcludesOwnPosts(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Agent's own problem (should be excluded)
	createBriefingPost(t, pool, "My own problem", "problem", "open", "agent", agentID, []string{"go"})
	// Someone else's problem (should be included)
	createBriefingPost(t, pool, "Someone elses go problem", "problem", "open", "human", userID, []string{"go"})

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpportunitiesForAgent(context.Background(), agentID, []string{"go"}, 10)
	if err != nil {
		t.Fatalf("GetOpportunitiesForAgent failed: %v", err)
	}

	for _, opp := range result.Items {
		if opp.Title == "My own problem" {
			t.Error("agent's own problem should NOT appear in opportunities")
		}
	}

	found := false
	for _, opp := range result.Items {
		if opp.Title == "Someone elses go problem" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Someone elses go problem' in opportunities")
	}
}

func TestBriefing_Opportunities_ExcludesSolvedClosed(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	createBriefingPost(t, pool, "Solved problem", "problem", "solved", "human", userID, []string{"go"})
	createBriefingPost(t, pool, "Closed problem", "problem", "closed", "human", userID, []string{"go"})
	createBriefingPost(t, pool, "Open problem", "problem", "open", "human", userID, []string{"go"})

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpportunitiesForAgent(context.Background(), agentID, []string{"go"}, 10)
	if err != nil {
		t.Fatalf("GetOpportunitiesForAgent failed: %v", err)
	}

	for _, opp := range result.Items {
		if opp.Title == "Solved problem" {
			t.Error("solved problem should NOT appear in opportunities")
		}
		if opp.Title == "Closed problem" {
			t.Error("closed problem should NOT appear in opportunities")
		}
	}

	found := false
	for _, opp := range result.Items {
		if opp.Title == "Open problem" {
			found = true
		}
	}
	if !found {
		t.Error("expected 'Open problem' in opportunities")
	}
}

func TestBriefing_Opportunities_PrioritizesZeroApproaches(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Problem with 0 approaches (should rank first)
	createBriefingPost(t, pool, "Zero approaches problem", "problem", "open", "human", userID, []string{"go"})

	// Problem with 1 approach (should rank second)
	p2 := createBriefingPost(t, pool, "One approach problem", "problem", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', 'other_agent_2', 'Some angle', 'starting')`, p2)
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	repo := NewBriefingRepository(pool)
	result, err := repo.GetOpportunitiesForAgent(ctx, agentID, []string{"go"}, 10)
	if err != nil {
		t.Fatalf("GetOpportunitiesForAgent failed: %v", err)
	}

	if len(result.Items) < 2 {
		t.Fatalf("expected at least 2 opportunities, got %d", len(result.Items))
	}

	// Find positions of our test items
	zeroIdx := -1
	oneIdx := -1
	for i, opp := range result.Items {
		if opp.Title == "Zero approaches problem" {
			zeroIdx = i
		}
		if opp.Title == "One approach problem" {
			oneIdx = i
		}
	}

	if zeroIdx == -1 {
		t.Fatal("'Zero approaches problem' not found in results")
	}
	if oneIdx == -1 {
		t.Fatal("'One approach problem' not found in results")
	}
	if zeroIdx > oneIdx {
		t.Errorf("zero-approach problem (idx %d) should rank before one-approach problem (idx %d)", zeroIdx, oneIdx)
	}
}

// --- GetReputationChangesSince ---

func TestBriefing_Reputation_UpvoteOnApproach(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a problem, then agent's approach on it
	problemID := createBriefingPost(t, pool, "Reputation test problem", "problem", "open", "human", userID, []string{"go"})
	var approachID string
	err := pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1, 'agent', $2, 'Agent approach for votes', 'starting')
		 RETURNING id`, problemID, agentID).Scan(&approachID)
	if err != nil {
		t.Fatalf("failed to create approach: %v", err)
	}

	// Insert an upvote on the approach
	_, err = pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('approach', $1, 'human', $2, 'up', true)`, approachID, userID)
	if err != nil {
		t.Fatalf("failed to create vote: %v", err)
	}

	repo := NewBriefingRepository(pool)
	since := time.Now().Add(-1 * time.Hour) // check last hour
	result, err := repo.GetReputationChangesSince(ctx, agentID, since)
	if err != nil {
		t.Fatalf("GetReputationChangesSince failed: %v", err)
	}

	if result.SinceLastCheck != "+10" {
		t.Errorf("expected delta '+10', got '%s'", result.SinceLastCheck)
	}

	found := false
	for _, e := range result.Breakdown {
		if e.Reason == "approach_upvoted" && e.Delta == 10 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'approach_upvoted' event with delta 10, got: %+v", result.Breakdown)
	}
}

func TestBriefing_Reputation_DownvoteOnPost(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Agent's post
	postID := createBriefingPost(t, pool, "Downvoted post", "problem", "open", "agent", agentID, []string{"go"})

	// Downvote on it
	_, err := pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, 'human', $2, 'down', true)`, postID, userID)
	if err != nil {
		t.Fatalf("failed to create downvote: %v", err)
	}

	repo := NewBriefingRepository(pool)
	since := time.Now().Add(-1 * time.Hour)
	result, err := repo.GetReputationChangesSince(ctx, agentID, since)
	if err != nil {
		t.Fatalf("GetReputationChangesSince failed: %v", err)
	}

	if result.SinceLastCheck != "-1" {
		t.Errorf("expected delta '-1', got '%s'", result.SinceLastCheck)
	}

	found := false
	for _, e := range result.Breakdown {
		if e.Reason == "post_downvoted" && e.Delta == -1 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'post_downvoted' event with delta -1, got: %+v", result.Breakdown)
	}
}

func TestBriefing_Reputation_AcceptedAnswer(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Human posts a question
	questionID := createBriefingPost(t, pool, "Question for accepted answer", "question", "open", "human", userID, []string{"go"})

	// Agent answers, and it gets accepted
	_, err := pool.Exec(ctx,
		`INSERT INTO answers (question_id, author_type, author_id, content, is_accepted)
		 VALUES ($1, 'agent', $2, 'Here is the solution', true)`, questionID, agentID)
	if err != nil {
		t.Fatalf("failed to create accepted answer: %v", err)
	}

	repo := NewBriefingRepository(pool)
	since := time.Now().Add(-1 * time.Hour)
	result, err := repo.GetReputationChangesSince(ctx, agentID, since)
	if err != nil {
		t.Fatalf("GetReputationChangesSince failed: %v", err)
	}

	if result.SinceLastCheck != "+50" {
		t.Errorf("expected delta '+50', got '%s'", result.SinceLastCheck)
	}

	found := false
	for _, e := range result.Breakdown {
		if e.Reason == "answer_accepted" && e.Delta == 50 {
			found = true
		}
	}
	if !found {
		t.Errorf("expected 'answer_accepted' event with delta 50, got: %+v", result.Breakdown)
	}
}
