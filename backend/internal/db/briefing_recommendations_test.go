package db

import (
	"context"
	"testing"
)

// --- GetYouMightLike ---

func TestGetYouMightLike_TagAffinity(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a "rust" post by user and have agent upvote it (confirmed)
	votedPostID := createBriefingPost(t, pool, "Rust ownership patterns", "question", "open", "human", userID, []string{"rust"})
	_, err := pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, 'agent', $2, 'up', true)`, votedPostID, agentID)
	if err != nil {
		t.Fatalf("failed to create vote: %v", err)
	}

	// Create another "rust" post by user (not interacted with)
	recommendableID := createBriefingPost(t, pool, "Rust async runtime comparison", "question", "open", "human", userID, []string{"rust"})

	repo := NewRecommendationRepository(pool)
	results, err := repo.GetYouMightLike(ctx, agentID, []string{"go"}, 5)
	if err != nil {
		t.Fatalf("GetYouMightLike failed: %v", err)
	}

	// Should find the second rust post via tag affinity
	found := false
	for _, r := range results {
		if r.ID == recommendableID {
			found = true
			if r.MatchReason != "voted_tags" {
				t.Errorf("expected match_reason='voted_tags', got %q", r.MatchReason)
			}
			if r.Type != "question" {
				t.Errorf("expected type='question', got %q", r.Type)
			}
			if r.Title != "Rust async runtime comparison" {
				t.Errorf("expected title='Rust async runtime comparison', got %q", r.Title)
			}
		}
	}
	if !found {
		t.Errorf("expected to find post %s in recommendations, got %d results: %+v", recommendableID, len(results), results)
	}
}

func TestGetYouMightLike_ExcludesInteracted(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a "rust" post and have agent upvote it (this gives tag affinity to "rust")
	votedPostID := createBriefingPost(t, pool, "Rust basics", "question", "open", "human", userID, []string{"rust"})
	_, err := pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, 'agent', $2, 'up', true)`, votedPostID, agentID)
	if err != nil {
		t.Fatalf("failed to create vote: %v", err)
	}

	// Create another "rust" post and have agent vote on it too (interacted)
	interactedPostID := createBriefingPost(t, pool, "Rust interacted post", "question", "open", "human", userID, []string{"rust"})
	_, err = pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, 'agent', $2, 'up', true)`, interactedPostID, agentID)
	if err != nil {
		t.Fatalf("failed to create vote on interacted post: %v", err)
	}

	repo := NewRecommendationRepository(pool)
	results, err := repo.GetYouMightLike(ctx, agentID, []string{"go"}, 5)
	if err != nil {
		t.Fatalf("GetYouMightLike failed: %v", err)
	}

	// The interacted post should NOT appear in results
	for _, r := range results {
		if r.ID == interactedPostID {
			t.Errorf("interacted post %s should be excluded from recommendations", interactedPostID)
		}
		// The originally voted post should also be excluded
		if r.ID == votedPostID {
			t.Errorf("voted post %s should be excluded from recommendations", votedPostID)
		}
	}
}

func TestGetYouMightLike_ExcludesOwnPosts(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	agentID := createBriefingAgent(t, pool, []string{"rust"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a "rust" post by user and have agent upvote it
	votedPostID := createBriefingPost(t, pool, "Rust ownership", "question", "open", "human", userID, []string{"rust"})
	_, err := pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, 'agent', $2, 'up', true)`, votedPostID, agentID)
	if err != nil {
		t.Fatalf("failed to create vote: %v", err)
	}

	// Create a "rust" post by the agent itself
	ownPostID := createBriefingPost(t, pool, "My own rust post", "question", "open", "agent", agentID, []string{"rust"})

	repo := NewRecommendationRepository(pool)
	results, err := repo.GetYouMightLike(ctx, agentID, []string{"rust"}, 5)
	if err != nil {
		t.Fatalf("GetYouMightLike failed: %v", err)
	}

	// Agent's own post should not appear
	for _, r := range results {
		if r.ID == ownPostID {
			t.Errorf("agent's own post %s should be excluded from recommendations", ownPostID)
		}
	}
}

func TestGetYouMightLike_AdjacentTagsFallback(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	// Agent with specialties=['go'] but NO votes
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a post tagged ['go', 'testing'] — establishes 'testing' as adjacent to 'go'
	createBriefingPost(t, pool, "Go testing patterns", "question", "open", "human", userID, []string{"go", "testing"})

	// Create a post tagged ['testing'] only — should be recommended via adjacent_tags
	adjacentPostID := createBriefingPost(t, pool, "Testing best practices", "question", "open", "human", userID, []string{"testing"})

	repo := NewRecommendationRepository(pool)
	results, err := repo.GetYouMightLike(ctx, agentID, []string{"go"}, 5)
	if err != nil {
		t.Fatalf("GetYouMightLike failed: %v", err)
	}

	// Should find the adjacent-tagged post
	found := false
	for _, r := range results {
		if r.ID == adjacentPostID {
			found = true
			if r.MatchReason != "adjacent_tags" {
				t.Errorf("expected match_reason='adjacent_tags', got %q", r.MatchReason)
			}
		}
	}
	if !found {
		t.Errorf("expected to find adjacent-tag post %s in recommendations, got %d results: %+v", adjacentPostID, len(results), results)
	}
}

func TestGetYouMightLike_NoHistoryNoSpecialties(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	// Agent with NO specialties and NO votes
	agentID := createBriefingAgent(t, pool, []string{})
	defer cleanupBriefingTestData(t, pool, agentID, "")

	repo := NewRecommendationRepository(pool)
	results, err := repo.GetYouMightLike(ctx, agentID, []string{}, 5)
	if err != nil {
		t.Fatalf("GetYouMightLike failed: %v", err)
	}

	// Should return empty slice, not error
	if results == nil {
		t.Error("expected empty slice, got nil")
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results for agent with no history/specialties, got %d", len(results))
	}
}

func TestGetYouMightLike_DeduplicateResults(t *testing.T) {
	pool := briefingTestDB(t)
	defer pool.Close()

	ctx := context.Background()
	// Agent with specialties=['go'] — will try both tag_affinity and adjacent_tags
	agentID := createBriefingAgent(t, pool, []string{"go"})
	userID := createBriefingUser(t, pool)
	defer cleanupBriefingTestData(t, pool, agentID, userID)

	// Create a 'go' post and have agent upvote it (gives tag affinity to 'go')
	votedPostID := createBriefingPost(t, pool, "Go concurrency patterns", "question", "open", "human", userID, []string{"go"})
	_, err := pool.Exec(ctx,
		`INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
		 VALUES ('post', $1, 'agent', $2, 'up', true)`, votedPostID, agentID)
	if err != nil {
		t.Fatalf("failed to create vote: %v", err)
	}

	// Create a post tagged ['go', 'testing'] — matches both tag_affinity (via 'go') AND adjacent_tags
	dualMatchPostID := createBriefingPost(t, pool, "Go integration testing guide", "question", "open", "human", userID, []string{"go", "testing"})

	repo := NewRecommendationRepository(pool)
	results, err := repo.GetYouMightLike(ctx, agentID, []string{"go"}, 10)
	if err != nil {
		t.Fatalf("GetYouMightLike failed: %v", err)
	}

	// Count how many times the dual-match post appears
	count := 0
	for _, r := range results {
		if r.ID == dualMatchPostID {
			count++
		}
	}
	if count > 1 {
		t.Errorf("post %s appears %d times, expected at most 1 (deduplication)", dualMatchPostID, count)
	}
	if count == 0 {
		t.Errorf("expected dual-match post %s to appear in results", dualMatchPostID)
	}
}
