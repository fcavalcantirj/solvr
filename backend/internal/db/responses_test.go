// Package db provides database access for Solvr.
package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

func TestResponsesRepository_ListResponses_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea to reference
	timestamp := time.Now().Format("20060102150405")
	ideaID := "responses_list_empty_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Test description for responses', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert test idea: %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// List responses when none exist
	opts := models.ResponseListOptions{
		IdeaID:  ideaID,
		Page:    1,
		PerPage: 10,
	}

	responses, total, err := repo.ListResponses(ctx, ideaID, opts)
	if err != nil {
		t.Fatalf("ListResponses() error = %v", err)
	}

	// Should return empty list, not nil
	if responses == nil {
		t.Error("expected non-nil responses slice")
	}

	if len(responses) != 0 {
		t.Errorf("expected 0 responses, got %d", len(responses))
	}

	if total != 0 {
		t.Errorf("expected total = 0, got %d", total)
	}
}

func TestResponsesRepository_ListResponses_WithResponses(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea
	timestamp := time.Now().Format("20060102150405")
	ideaID := "responses_list_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Test description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert test idea: %v", err)
	}

	// Insert test responses
	responseIDs := []string{
		"responses_list_resp_1_" + timestamp,
		"responses_list_resp_2_" + timestamp,
		"responses_list_resp_3_" + timestamp,
	}
	responseTypes := []string{"build", "critique", "expand"}

	for i, respID := range responseIDs {
		_, err := pool.Exec(ctx, `
			INSERT INTO responses (id, idea_id, author_type, author_id, content, response_type, upvotes, downvotes)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
		`,
			respID,
			ideaID,
			"agent",
			"test_agent_"+timestamp,
			"Test response content "+string(rune('A'+i)),
			responseTypes[i],
			i*10, // upvotes: 0, 10, 20
			i,    // downvotes: 0, 1, 2
		)
		if err != nil {
			t.Fatalf("failed to insert test response: %v", err)
		}
	}

	// Clean up after test
	defer func() {
		for _, respID := range responseIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", respID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// List responses
	opts := models.ResponseListOptions{
		IdeaID:  ideaID,
		Page:    1,
		PerPage: 10,
	}

	responses, total, err := repo.ListResponses(ctx, ideaID, opts)
	if err != nil {
		t.Fatalf("ListResponses() error = %v", err)
	}

	if len(responses) != 3 {
		t.Errorf("expected 3 responses, got %d", len(responses))
	}

	if total != 3 {
		t.Errorf("expected total = 3, got %d", total)
	}

	// Check that responses have author information
	for _, resp := range responses {
		if resp.Author.Type == "" {
			t.Error("expected author type to be set")
		}
		if resp.Author.ID == "" {
			t.Error("expected author ID to be set")
		}
		if resp.Content == "" {
			t.Error("expected content to be set")
		}
		if resp.ResponseType == "" {
			t.Error("expected response_type to be set")
		}
	}
}

func TestResponsesRepository_ListResponses_Pagination(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea
	timestamp := time.Now().Format("20060102150405")
	ideaID := "responses_page_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Test description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert test idea: %v", err)
	}

	// Insert 5 responses
	responseIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		responseIDs[i] = "responses_page_resp_" + timestamp + "_" + string(rune('A'+i))
		_, err := pool.Exec(ctx, `
			INSERT INTO responses (id, idea_id, author_type, author_id, content, response_type)
			VALUES ($1, $2, 'agent', 'test_agent', $3, 'build')
		`,
			responseIDs[i],
			ideaID,
			"Response content "+string(rune('A'+i)),
		)
		if err != nil {
			t.Fatalf("failed to insert test response: %v", err)
		}
	}

	// Clean up after test
	defer func() {
		for _, respID := range responseIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", respID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// Test page 1 with per_page=2
	opts := models.ResponseListOptions{
		IdeaID:  ideaID,
		Page:    1,
		PerPage: 2,
	}

	responses, total, err := repo.ListResponses(ctx, ideaID, opts)
	if err != nil {
		t.Fatalf("ListResponses() page 1 error = %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("page 1: expected 2 responses, got %d", len(responses))
	}

	if total != 5 {
		t.Errorf("expected total = 5, got %d", total)
	}

	// Test page 2
	opts.Page = 2
	responses, total, err = repo.ListResponses(ctx, ideaID, opts)
	if err != nil {
		t.Fatalf("ListResponses() page 2 error = %v", err)
	}

	if len(responses) != 2 {
		t.Errorf("page 2: expected 2 responses, got %d", len(responses))
	}

	// Test page 3 (should have 1 response)
	opts.Page = 3
	responses, total, err = repo.ListResponses(ctx, ideaID, opts)
	if err != nil {
		t.Fatalf("ListResponses() page 3 error = %v", err)
	}

	if len(responses) != 1 {
		t.Errorf("page 3: expected 1 response, got %d", len(responses))
	}
}

func TestResponsesRepository_ListResponses_WrongIdea(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create two test ideas
	timestamp := time.Now().Format("20060102150405")
	idea1ID := "responses_wrong_idea1_" + timestamp
	idea2ID := "responses_wrong_idea2_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea 1', 'Description', 'agent', 'test_agent', 'open')
	`, idea1ID)
	if err != nil {
		t.Fatalf("failed to insert idea 1: %v", err)
	}

	_, err = pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea 2', 'Description', 'agent', 'test_agent', 'open')
	`, idea2ID)
	if err != nil {
		t.Fatalf("failed to insert idea 2: %v", err)
	}

	// Insert response for idea 1
	responseID := "responses_wrong_resp_" + timestamp
	_, err = pool.Exec(ctx, `
		INSERT INTO responses (id, idea_id, author_type, author_id, content, response_type)
		VALUES ($1, $2, 'agent', 'test_agent', 'Response content', 'build')
	`, responseID, idea1ID)
	if err != nil {
		t.Fatalf("failed to insert response: %v", err)
	}

	// Clean up
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", responseID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", idea1ID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", idea2ID)
	}()

	// Query responses for idea 2 - should be empty
	opts := models.ResponseListOptions{
		IdeaID:  idea2ID,
		Page:    1,
		PerPage: 10,
	}

	responses, total, err := repo.ListResponses(ctx, idea2ID, opts)
	if err != nil {
		t.Fatalf("ListResponses() error = %v", err)
	}

	if len(responses) != 0 {
		t.Errorf("expected 0 responses for idea2, got %d", len(responses))
	}

	if total != 0 {
		t.Errorf("expected total = 0 for idea2, got %d", total)
	}
}

func TestResponsesRepository_CreateResponse_Success(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea
	timestamp := time.Now().Format("20060102150405")
	ideaID := "responses_create_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Create response
	response := &models.Response{
		IdeaID:       ideaID,
		AuthorType:   models.AuthorTypeAgent,
		AuthorID:     "test_agent_" + timestamp,
		Content:      "This is a test response with build suggestions.",
		ResponseType: models.ResponseTypeBuild,
	}

	created, err := repo.CreateResponse(ctx, response)

	// Clean up
	defer func() {
		if created != nil && created.ID != "" {
			_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", created.ID)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	if err != nil {
		t.Fatalf("CreateResponse() error = %v", err)
	}

	if created == nil {
		t.Fatal("expected non-nil response")
	}

	if created.ID == "" {
		t.Error("expected ID to be set")
	}

	if created.IdeaID != ideaID {
		t.Errorf("expected idea_id = %s, got %s", ideaID, created.IdeaID)
	}

	if created.AuthorType != models.AuthorTypeAgent {
		t.Errorf("expected author_type = agent, got %s", created.AuthorType)
	}

	if created.Content != response.Content {
		t.Errorf("expected content = %s, got %s", response.Content, created.Content)
	}

	if created.ResponseType != models.ResponseTypeBuild {
		t.Errorf("expected response_type = build, got %s", created.ResponseType)
	}

	if created.Upvotes != 0 {
		t.Errorf("expected upvotes = 0, got %d", created.Upvotes)
	}

	if created.Downvotes != 0 {
		t.Errorf("expected downvotes = 0, got %d", created.Downvotes)
	}

	if created.CreatedAt.IsZero() {
		t.Error("expected created_at to be set")
	}
}

func TestResponsesRepository_CreateResponse_AllTypes(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea
	timestamp := time.Now().Format("20060102150405")
	ideaID := "responses_types_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	var createdIDs []string

	// Clean up
	defer func() {
		for _, id := range createdIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", id)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// Test all response types
	responseTypes := []models.ResponseType{
		models.ResponseTypeBuild,
		models.ResponseTypeCritique,
		models.ResponseTypeExpand,
		models.ResponseTypeQuestion,
		models.ResponseTypeSupport,
	}

	for _, rt := range responseTypes {
		response := &models.Response{
			IdeaID:       ideaID,
			AuthorType:   models.AuthorTypeHuman,
			AuthorID:     "test_user_" + timestamp,
			Content:      "Response of type " + string(rt),
			ResponseType: rt,
		}

		created, err := repo.CreateResponse(ctx, response)
		if err != nil {
			t.Errorf("CreateResponse(%s) error = %v", rt, err)
			continue
		}

		if created.ResponseType != rt {
			t.Errorf("expected response_type = %s, got %s", rt, created.ResponseType)
		}

		createdIDs = append(createdIDs, created.ID)
	}
}

func TestResponsesRepository_GetResponseCount(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea
	timestamp := time.Now().Format("20060102150405")
	ideaID := "responses_count_idea_" + timestamp

	_, err := pool.Exec(ctx, `
		INSERT INTO posts (id, type, title, description, posted_by_type, posted_by_id, status)
		VALUES ($1, 'idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
	`, ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Initially count should be 0
	count, err := repo.GetResponseCount(ctx, ideaID)
	if err != nil {
		t.Fatalf("GetResponseCount() initial error = %v", err)
	}
	if count != 0 {
		t.Errorf("expected initial count = 0, got %d", count)
	}

	// Insert 3 responses
	responseIDs := make([]string, 3)
	for i := 0; i < 3; i++ {
		responseIDs[i] = "responses_count_resp_" + timestamp + "_" + string(rune('A'+i))
		_, err := pool.Exec(ctx, `
			INSERT INTO responses (id, idea_id, author_type, author_id, content, response_type)
			VALUES ($1, $2, 'agent', 'test_agent', $3, 'build')
		`, responseIDs[i], ideaID, "Response "+string(rune('A'+i)))
		if err != nil {
			t.Fatalf("failed to insert response: %v", err)
		}
	}

	// Clean up
	defer func() {
		for _, id := range responseIDs {
			_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", id)
		}
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
	}()

	// Count should now be 3
	count, err = repo.GetResponseCount(ctx, ideaID)
	if err != nil {
		t.Fatalf("GetResponseCount() after insert error = %v", err)
	}
	if count != 3 {
		t.Errorf("expected count = 3, got %d", count)
	}
}
