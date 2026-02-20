// Package db provides database connection pool and helper functions.
package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// createInferTestAgent creates a test agent for inferred specialties tests.
func createInferTestAgent(t *testing.T, pool *Pool, suffix string) *models.Agent {
	t.Helper()
	ctx := context.Background()
	agentRepo := NewAgentRepository(pool)

	agentID := "infer_agent_" + suffix + "_" + time.Now().Format("20060102150405.000000000")
	agent := &models.Agent{
		ID:          agentID,
		DisplayName: "Infer Test Agent " + suffix,
		Status:      "active",
	}
	if err := agentRepo.Create(ctx, agent); err != nil {
		t.Fatalf("failed to create test agent: %v", err)
	}
	return agent
}

// createInferTestUser creates a test user for inferred specialties tests.
func createInferTestUser(t *testing.T, pool *Pool, suffix string) *models.User {
	t.Helper()
	ctx := context.Background()
	userRepo := NewUserRepository(pool)

	username := "infer_user_" + suffix + "_" + time.Now().Format("150405.000000000")
	user := &models.User{
		Username:       username,
		DisplayName:    "Infer Test User " + suffix,
		Email:          username + "@test.com",
		AuthProvider:   "github",
		AuthProviderID: "gh_" + username,
	}
	created, err := userRepo.Create(ctx, user)
	if err != nil {
		t.Fatalf("failed to create test user: %v", err)
	}
	return created
}

// createInferTestPost creates a test post with given tags and author, returning the post ID.
func createInferTestPost(t *testing.T, pool *Pool, authorType, authorID string, tags []string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id)
		VALUES ('problem', 'Infer test post', 'A test post for inferred specialties testing',
		        $1, 'open', $2, $3)
		RETURNING id::text
	`, tags, authorType, authorID).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create infer test post: %v", err)
	}
	return id
}

// createInferTestApproach creates an approach on a problem by a given agent, returning the approach ID.
func createInferTestApproach(t *testing.T, pool *Pool, problemID, authorID string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		VALUES ($1, 'agent', $2, 'Test angle for inferred specialties', 'working')
		RETURNING id::text
	`, problemID, authorID).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create infer test approach: %v", err)
	}
	return id
}

// createInferTestAnswer creates an answer on a question by a given author, returning the answer ID.
func createInferTestAnswer(t *testing.T, pool *Pool, questionID, authorType, authorID string) string {
	t.Helper()
	ctx := context.Background()

	var id string
	err := pool.QueryRow(ctx, `
		INSERT INTO answers (question_id, author_type, author_id, content)
		VALUES ($1, $2, $3, 'Test answer for inferred specialties')
		RETURNING id::text
	`, questionID, authorType, authorID).Scan(&id)
	if err != nil {
		t.Fatalf("failed to create infer test answer: %v", err)
	}
	return id
}

// createInferTestVote creates a confirmed upvote on a post by a given voter.
func createInferTestVote(t *testing.T, pool *Pool, postID, voterType, voterID string) {
	t.Helper()
	ctx := context.Background()

	_, err := pool.Exec(ctx, `
		INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		VALUES ('post', $1::uuid, $2, $3, 'up', true)
	`, postID, voterType, voterID)
	if err != nil {
		t.Fatalf("failed to create infer test vote: %v", err)
	}
}

// TestInferSpecialties_AgentWithActivity tests that an agent with posts, approaches,
// and upvotes gets the correct inferred specialties.
// Agent has 3 go-tagged posts, 2 docker-tagged approaches, 1 rust upvote.
// Expected: ['go', 'docker', 'rust'] (ordered by weighted frequency).
func TestInferSpecialties_AgentWithActivity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewInferredSpecialtiesRepository(pool)

	agent := createInferTestAgent(t, pool, "activity")

	// Create 3 go-tagged posts by agent (weight 2 each = 6 total for 'go')
	createInferTestPost(t, pool, "agent", agent.ID, []string{"go", "testing"})
	createInferTestPost(t, pool, "agent", agent.ID, []string{"go", "concurrency"})
	createInferTestPost(t, pool, "agent", agent.ID, []string{"go"})

	// Create 2 docker-tagged problems by another author, then approaches by our agent
	// (approaches inherit tags from parent post, weight 2 each = 4 total for 'docker')
	otherAgent := createInferTestAgent(t, pool, "other")
	dockerProblem1 := createInferTestPost(t, pool, "agent", otherAgent.ID, []string{"docker", "kubernetes"})
	dockerProblem2 := createInferTestPost(t, pool, "agent", otherAgent.ID, []string{"docker"})
	createInferTestApproach(t, pool, dockerProblem1, agent.ID)
	createInferTestApproach(t, pool, dockerProblem2, agent.ID)

	// Create 1 rust-tagged post by someone else, upvoted by agent (weight 1 for 'rust')
	rustPost := createInferTestPost(t, pool, "agent", otherAgent.ID, []string{"rust"})
	createInferTestVote(t, pool, rustPost, "agent", agent.ID)

	// Infer specialties
	specialties, err := repo.InferSpecialtiesForAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("InferSpecialtiesForAgent failed: %v", err)
	}

	// Verify: should contain go, docker, rust
	if len(specialties) < 3 {
		t.Errorf("expected at least 3 inferred specialties, got %d: %v", len(specialties), specialties)
	}

	// 'go' should be first (highest weight: 3 posts * weight 2 = 6)
	if len(specialties) > 0 && specialties[0] != "go" {
		t.Errorf("expected first specialty to be 'go', got '%s' (full: %v)", specialties[0], specialties)
	}

	// Verify all expected tags are present
	tagSet := make(map[string]bool)
	for _, s := range specialties {
		tagSet[s] = true
	}
	for _, expected := range []string{"go", "docker", "rust"} {
		if !tagSet[expected] {
			t.Errorf("expected specialty '%s' to be present in %v", expected, specialties)
		}
	}
}

// TestInferSpecialties_AgentNoActivity tests that an agent with no activity
// returns an empty slice.
func TestInferSpecialties_AgentNoActivity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewInferredSpecialtiesRepository(pool)

	agent := createInferTestAgent(t, pool, "noactivity")

	specialties, err := repo.InferSpecialtiesForAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("InferSpecialtiesForAgent failed: %v", err)
	}

	if len(specialties) != 0 {
		t.Errorf("expected empty specialties for agent with no activity, got %v", specialties)
	}
}

// TestInferSpecialties_UserWithActivity tests that a user with posted problems
// and answers gets the correct inferred specialties.
func TestInferSpecialties_UserWithActivity(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewInferredSpecialtiesRepository(pool)

	user := createInferTestUser(t, pool, "activity")

	// Create go-tagged and python-tagged posts by user
	createInferTestPost(t, pool, "human", user.ID, []string{"go", "api"})
	createInferTestPost(t, pool, "human", user.ID, []string{"python", "ml"})
	createInferTestPost(t, pool, "human", user.ID, []string{"python"})

	// Create a question by another user, answered by our user (question has 'javascript' tag)
	otherAgent := createInferTestAgent(t, pool, "other_for_user")
	jsQuestion := createInferTestPost(t, pool, "agent", otherAgent.ID, []string{"javascript"})
	// Change type to question so answer can reference it
	_, err := pool.Exec(ctx, `UPDATE posts SET type = 'question' WHERE id = $1::uuid`, jsQuestion)
	if err != nil {
		t.Fatalf("failed to update post type: %v", err)
	}
	createInferTestAnswer(t, pool, jsQuestion, "human", user.ID)

	specialties, err := repo.InferSpecialtiesForUser(ctx, user.ID)
	if err != nil {
		t.Fatalf("InferSpecialtiesForUser failed: %v", err)
	}

	if len(specialties) < 2 {
		t.Errorf("expected at least 2 inferred specialties, got %d: %v", len(specialties), specialties)
	}

	// Verify expected tags are present
	tagSet := make(map[string]bool)
	for _, s := range specialties {
		tagSet[s] = true
	}
	for _, expected := range []string{"python", "go"} {
		if !tagSet[expected] {
			t.Errorf("expected specialty '%s' to be present in %v", expected, specialties)
		}
	}
}

// TestInferSpecialties_MaxFiveTags tests that even with activity across many tags,
// only the top 5 are returned.
func TestInferSpecialties_MaxFiveTags(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	repo := NewInferredSpecialtiesRepository(pool)

	agent := createInferTestAgent(t, pool, "maxfive")

	// Create posts with 10 different tags (each post has a unique tag)
	tags := []string{"tag1", "tag2", "tag3", "tag4", "tag5", "tag6", "tag7", "tag8", "tag9", "tag10"}
	for _, tag := range tags {
		createInferTestPost(t, pool, "agent", agent.ID, []string{tag})
	}

	specialties, err := repo.InferSpecialtiesForAgent(ctx, agent.ID)
	if err != nil {
		t.Fatalf("InferSpecialtiesForAgent failed: %v", err)
	}

	if len(specialties) > 5 {
		t.Errorf("expected at most 5 inferred specialties, got %d: %v", len(specialties), specialties)
	}

	if len(specialties) != 5 {
		t.Errorf("expected exactly 5 inferred specialties (top 5 from 10), got %d: %v", len(specialties), specialties)
	}
}
