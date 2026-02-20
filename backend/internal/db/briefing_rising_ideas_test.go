package db

import (
	"context"
	"fmt"
	"testing"
)

// helper: create responses on an idea
func createIdeaResponses(t *testing.T, pool *Pool, ideaID string, count int, authorID string) {
	t.Helper()
	ctx := context.Background()
	for i := 0; i < count; i++ {
		_, err := pool.Exec(ctx,
			`INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
			 VALUES ($1, 'agent', $2, $3, 'build')`,
			ideaID, authorID, fmt.Sprintf("Response %d", i))
		if err != nil {
			t.Fatalf("failed to create response %d: %v", i, err)
		}
	}
}

func TestGetRisingIdeas_RankedByResponses(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	ideaA := createBriefingPost(t, pool, "Rising idea A many responses", "idea", "open", "human", userID, []string{"go"})
	ideaB := createBriefingPost(t, pool, "Rising idea B fewer responses", "idea", "open", "human", userID, []string{"go"})
	createIdeaResponses(t, pool, ideaA, 3, agentID)
	createIdeaResponses(t, pool, ideaB, 1, agentID)

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetRisingIdeas(ctx, 10)
	if err != nil {
		t.Fatalf("GetRisingIdeas failed: %v", err)
	}

	idxA, idxB := -1, -1
	for i, r := range results {
		if r.ID == ideaA {
			idxA = i
			if r.ResponseCount != 3 {
				t.Errorf("expected response_count=3 for ideaA, got %d", r.ResponseCount)
			}
		}
		if r.ID == ideaB {
			idxB = i
		}
	}
	if idxA == -1 || idxB == -1 {
		t.Fatalf("expected both ideas in results, idxA=%d, idxB=%d", idxA, idxB)
	}
	if idxA >= idxB {
		t.Errorf("ideaA (3 responses) should rank before ideaB (1 response), idxA=%d, idxB=%d", idxA, idxB)
	}
}

func TestGetRisingIdeas_IncludesUpvotedNoResponses(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	ideaID := createBriefingPost(t, pool, "Upvoted idea no responses", "idea", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx, "UPDATE posts SET upvotes = 2 WHERE id = $1", ideaID)
	if err != nil {
		t.Fatalf("failed to update upvotes: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetRisingIdeas(ctx, 10)
	if err != nil {
		t.Fatalf("GetRisingIdeas failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.ID == ideaID {
			found = true
			if r.Upvotes != 2 {
				t.Errorf("expected upvotes=2, got %d", r.Upvotes)
			}
		}
	}
	if !found {
		t.Error("idea with 2 upvotes and 0 responses should be included")
	}
}

func TestGetRisingIdeas_ExcludesDormant(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	ideaID := createBriefingPost(t, pool, "Dormant idea with responses", "idea", "dormant", "human", userID, []string{"go"})
	createIdeaResponses(t, pool, ideaID, 3, agentID)

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetRisingIdeas(ctx, 50)
	if err != nil {
		t.Fatalf("GetRisingIdeas failed: %v", err)
	}
	for _, r := range results {
		if r.ID == ideaID {
			t.Error("dormant idea should be excluded from rising_ideas")
		}
	}
}

func TestGetRisingIdeas_EvolvedCount(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	ideaID := createBriefingPost(t, pool, "Evolved idea test", "idea", "open", "human", userID, []string{"go"})
	// Set evolved_into to 2 UUIDs and upvotes > 0 to ensure inclusion
	_, err := pool.Exec(ctx, `UPDATE posts SET evolved_into = ARRAY[$1::uuid, $2::uuid], upvotes = 1 WHERE id = $3`,
		"00000000-0000-0000-0000-000000000001", "00000000-0000-0000-0000-000000000002", ideaID)
	if err != nil {
		t.Fatalf("failed to set evolved_into: %v", err)
	}

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetRisingIdeas(ctx, 10)
	if err != nil {
		t.Fatalf("GetRisingIdeas failed: %v", err)
	}

	found := false
	for _, r := range results {
		if r.ID == ideaID {
			found = true
			if r.EvolvedCount != 2 {
				t.Errorf("expected evolved_count=2, got %d", r.EvolvedCount)
			}
		}
	}
	if !found {
		t.Error("idea with evolved_into should appear in rising_ideas")
	}
}

func TestGetRisingIdeas_ExcludesZeroEngagement(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()
	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create idea with 0 responses and 0 upvotes
	ideaID := createBriefingPost(t, pool, "Zero engagement idea", "idea", "open", "human", userID, []string{"go"})

	repo := NewPlatformBriefingRepository(pool)
	results, err := repo.GetRisingIdeas(ctx, 50)
	if err != nil {
		t.Fatalf("GetRisingIdeas failed: %v", err)
	}
	for _, r := range results {
		if r.ID == ideaID {
			t.Error("idea with 0 responses and 0 upvotes should be excluded (HAVING clause)")
		}
	}
}
