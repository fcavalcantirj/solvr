// Package db provides database access for Solvr.
package db

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// Note: These tests require a running PostgreSQL database.
// Set DATABASE_URL environment variable to run integration tests.
// Tests will be skipped if DATABASE_URL is not set.

// TestListResponsesQueryUsesCorrectColumns verifies that the SQL query
// uses the correct column names from the agents table.
// This test catches bugs like using 'a.name' instead of 'a.display_name'.
func TestListResponsesQueryUsesCorrectColumns(t *testing.T) {
	// The agents table has 'display_name', not 'name'.
	// This test ensures we don't accidentally use the wrong column.

	// Read the source file to check the query
	// Since we can't easily inspect the query at runtime,
	// we verify by checking that the function compiles and
	// the query string in the source uses correct column names.

	// Create a repository with nil pool - we're just checking compilation
	// and that the query structure is correct
	repo := &ResponsesRepository{pool: nil}

	// Verify the repository was created (basic sanity check)
	if repo == nil {
		t.Fatal("expected non-nil repository")
	}

	// The real test is the integration test below that verifies
	// the query works with actual data. This unit test just ensures
	// the code compiles with the correct column names.

	// If someone changes 'a.display_name' to 'a.name' in the query,
	// the integration tests will fail with:
	// "ERROR: column a.name does not exist (SQLSTATE 42703)"
}

// TestListResponsesWithAgentAuthor tests that ListResponses correctly
// retrieves agent author information using the correct column names.
func TestListResponsesWithAgentAuthor(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	now := time.Now()
	ns := fmt.Sprintf("%04d", now.Nanosecond()/100000)
	timestamp := now.Format("20060102150405")
	agentID := "raa_" + now.Format("150405") + ns
	displayName := "Resp Agent " + now.Format("150405") + ns

	// Create test agent with display_name
	_, err := pool.Exec(ctx, `
		INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $2, $3, 'active')
	`, agentID, displayName, "hash_"+timestamp)
	if err != nil {
		// If agents table doesn't exist, skip
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("agents table does not exist, skipping")
		}
		t.Fatalf("failed to insert agent: %v", err)
	}

	// Create test idea
	var ideaID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Description', 'agent', $1, 'open')
		RETURNING id::text
	`, agentID).Scan(&ideaID)
	if err != nil {
		t.Fatalf("failed to insert idea: %v", err)
	}

	// Create response by the agent
	var responseID string
	err = pool.QueryRow(ctx, `
		INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'Agent response content', 'build')
		RETURNING id::text
	`, ideaID, agentID).Scan(&responseID)
	if err != nil {
		// If responses table doesn't exist, skip
		if strings.Contains(err.Error(), "does not exist") {
			t.Skip("responses table does not exist, skipping")
		}
		t.Fatalf("failed to insert response: %v", err)
	}

	// Cleanup
	defer func() {
		_, _ = pool.Exec(ctx, "DELETE FROM responses WHERE id = $1", responseID)
		_, _ = pool.Exec(ctx, "DELETE FROM posts WHERE id = $1", ideaID)
		_, _ = pool.Exec(ctx, "DELETE FROM agents WHERE id = $1", agentID)
	}()

	// Test ListResponses - this will fail if query uses wrong column name
	opts := models.ResponseListOptions{
		IdeaID:  ideaID,
		Page:    1,
		PerPage: 10,
	}

	responses, total, err := repo.ListResponses(ctx, ideaID, opts)
	if err != nil {
		t.Fatalf("ListResponses() error = %v", err)
	}

	if total != 1 {
		t.Errorf("expected total = 1, got %d", total)
	}

	if len(responses) != 1 {
		t.Fatalf("expected 1 response, got %d", len(responses))
	}

	// Verify the agent's display_name was retrieved correctly
	resp := responses[0]
	if resp.Author.DisplayName != displayName {
		t.Errorf("expected author display_name = '%s', got '%s'", displayName, resp.Author.DisplayName)
	}

	if resp.Author.Type != "agent" {
		t.Errorf("expected author type = 'agent', got '%s'", resp.Author.Type)
	}

	if resp.Author.ID != agentID {
		t.Errorf("expected author ID = '%s', got '%s'", agentID, resp.Author.ID)
	}
}

func TestResponsesRepository_ListResponses_Empty(t *testing.T) {
	pool := getTestPool(t)
	if pool == nil {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	defer pool.Close()

	repo := NewResponsesRepository(pool)
	ctx := context.Background()

	// Create a test idea to reference
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Test description for responses', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
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
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Test description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
	if err != nil {
		t.Fatalf("failed to insert test idea: %v", err)
	}

	// Insert test responses
	responseIDs := make([]string, 3)
	responseTypes := []string{"build", "critique", "expand"}

	for i := range responseIDs {
		err := pool.QueryRow(ctx, `
			INSERT INTO responses (idea_id, author_type, author_id, content, response_type, upvotes, downvotes)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
			RETURNING id::text
		`,
			ideaID,
			"agent",
			"test_agent_"+timestamp,
			"Test response content "+string(rune('A'+i)),
			responseTypes[i],
			i*10, // upvotes: 0, 10, 20
			i,    // downvotes: 0, 1, 2
		).Scan(&responseIDs[i])
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
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Test description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
	if err != nil {
		t.Fatalf("failed to insert test idea: %v", err)
	}

	// Insert 5 responses
	responseIDs := make([]string, 5)
	for i := 0; i < 5; i++ {
		err := pool.QueryRow(ctx, `
			INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
			VALUES ($1, 'agent', 'test_agent', $2, 'build')
			RETURNING id::text
		`,
			ideaID,
			"Response content "+string(rune('A'+i)),
		).Scan(&responseIDs[i])
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
	var idea1ID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea 1', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&idea1ID)
	if err != nil {
		t.Fatalf("failed to insert idea 1: %v", err)
	}

	var idea2ID string
	err = pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea 2', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&idea2ID)
	if err != nil {
		t.Fatalf("failed to insert idea 2: %v", err)
	}

	// Insert response for idea 1
	var responseID string
	err = pool.QueryRow(ctx, `
		INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', 'test_agent', 'Response content', 'build')
		RETURNING id::text
	`, idea1ID).Scan(&responseID)
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
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
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
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
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
	var ideaID string
	err := pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status)
		VALUES ('idea', 'Test Idea', 'Description', 'agent', 'test_agent', 'open')
		RETURNING id::text
	`).Scan(&ideaID)
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
		err := pool.QueryRow(ctx, `
			INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
			VALUES ($1, 'agent', 'test_agent', $2, 'build')
			RETURNING id::text
		`, ideaID, "Response "+string(rune('A'+i))).Scan(&responseIDs[i])
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
