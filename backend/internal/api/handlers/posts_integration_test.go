package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// TestPostsCRUD_Integration tests the full posts lifecycle:
// Create post → Read post → Update post → Delete post
// This verifies all CRUD operations work together correctly.
func TestPostsCRUD_Integration(t *testing.T) {
	repo := NewMockPostsRepositoryForIntegration()
	handler := NewPostsHandler(repo)
	userID := "integration-test-user"

	var createdPostID string

	// Step 1: Create a post
	t.Run("Step1_CreatePost", func(t *testing.T) {
		body := map[string]interface{}{
			"type":        "problem",
			"title":       "Integration Test Problem Title",
			"description": "This is a test description for the integration test. It needs to be at least fifty characters long to pass validation requirements.",
			"tags":        []string{"integration", "testing"},
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req = addAuthContext(req, userID, "user")
		w := httptest.NewRecorder()

		handler.Create(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("Step 1 failed: expected status 201, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 1 failed: failed to decode response: %v", err)
		}

		data := resp["data"].(map[string]interface{})
		createdPostID = data["id"].(string)

		// Verify created post fields
		if data["title"] != "Integration Test Problem Title" {
			t.Errorf("Step 1: expected title 'Integration Test Problem Title', got '%v'", data["title"])
		}
		if data["type"] != "problem" {
			t.Errorf("Step 1: expected type 'problem', got '%v'", data["type"])
		}
		if data["status"] != "open" {
			t.Errorf("Step 1: expected status 'open', got '%v'", data["status"])
		}

		t.Logf("Step 1 passed: Post created with ID=%s", createdPostID)
	})

	// Step 2: Read the created post
	t.Run("Step2_ReadPost", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("Skipping Step 2: no post ID from Step 1")
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/posts/"+createdPostID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler.Get(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 2 failed: expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 2 failed: failed to decode response: %v", err)
		}

		data := resp["data"].(map[string]interface{})

		// Verify post data
		if data["id"] != createdPostID {
			t.Errorf("Step 2: expected id '%s', got '%v'", createdPostID, data["id"])
		}
		if data["title"] != "Integration Test Problem Title" {
			t.Errorf("Step 2: expected title 'Integration Test Problem Title', got '%v'", data["title"])
		}
		if data["type"] != "problem" {
			t.Errorf("Step 2: expected type 'problem', got '%v'", data["type"])
		}

		// Verify author info is included
		author := data["author"].(map[string]interface{})
		if author["id"] != userID {
			t.Errorf("Step 2: expected author id '%s', got '%v'", userID, author["id"])
		}
		if author["type"] != "human" {
			t.Errorf("Step 2: expected author type 'human', got '%v'", author["type"])
		}

		t.Logf("Step 2 passed: Post retrieved with correct data")
	})

	// Step 3: Update the post
	t.Run("Step3_UpdatePost", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("Skipping Step 3: no post ID from Step 1")
		}

		body := map[string]interface{}{
			"title":       "Updated Integration Test Title",
			"description": "This is the updated description for the integration test. It is also at least fifty characters long to pass validation.",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/v1/posts/"+createdPostID, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, userID, "user")
		w := httptest.NewRecorder()

		handler.Update(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 3 failed: expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 3 failed: failed to decode response: %v", err)
		}

		data := resp["data"].(map[string]interface{})

		// Verify updated fields
		if data["title"] != "Updated Integration Test Title" {
			t.Errorf("Step 3: expected updated title 'Updated Integration Test Title', got '%v'", data["title"])
		}

		t.Logf("Step 3 passed: Post updated successfully")
	})

	// Step 4: Verify the update by reading again
	t.Run("Step4_VerifyUpdate", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("Skipping Step 4: no post ID from Step 1")
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/posts/"+createdPostID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler.Get(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("Step 4 failed: expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Step 4 failed: failed to decode response: %v", err)
		}

		data := resp["data"].(map[string]interface{})

		// Verify the updated title persists
		if data["title"] != "Updated Integration Test Title" {
			t.Errorf("Step 4: expected title 'Updated Integration Test Title', got '%v'", data["title"])
		}

		t.Logf("Step 4 passed: Update verified successfully")
	})

	// Step 5: Delete the post
	t.Run("Step5_DeletePost", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("Skipping Step 5: no post ID from Step 1")
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1/posts/"+createdPostID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, userID, "user")
		w := httptest.NewRecorder()

		handler.Delete(w, req)

		if w.Code != http.StatusNoContent {
			t.Fatalf("Step 5 failed: expected status 204, got %d: %s", w.Code, w.Body.String())
		}

		t.Logf("Step 5 passed: Post deleted successfully")
	})

	// Step 6: Verify deletion by trying to read again
	t.Run("Step6_VerifyDeletion", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("Skipping Step 6: no post ID from Step 1")
		}

		req := httptest.NewRequest(http.MethodGet, "/v1/posts/"+createdPostID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		w := httptest.NewRecorder()

		handler.Get(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Step 6: expected status 404 after deletion, got %d", w.Code)
		}

		t.Logf("Step 6 passed: Deletion verified - post not found")
	})
}

// TestPostsCRUD_OwnershipEnforcement tests that non-owners cannot modify posts.
func TestPostsCRUD_OwnershipEnforcement(t *testing.T) {
	repo := NewMockPostsRepositoryForIntegration()
	handler := NewPostsHandler(repo)
	ownerID := "owner-user"
	nonOwnerID := "non-owner-user"

	var createdPostID string

	// Create a post as owner
	t.Run("CreateAsOwner", func(t *testing.T) {
		body := map[string]interface{}{
			"type":        "question",
			"title":       "Ownership Test Question Title",
			"description": "This is a test question to verify ownership enforcement works correctly with at least fifty characters.",
			"tags":        []string{"ownership", "test"},
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req = addAuthContext(req, ownerID, "user")
		w := httptest.NewRecorder()

		handler.Create(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create post: status %d", w.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		data := resp["data"].(map[string]interface{})
		createdPostID = data["id"].(string)
	})

	// Non-owner cannot update
	t.Run("NonOwnerCannotUpdate", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		body := map[string]interface{}{
			"title": "Hacked Title By Non-Owner",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/v1/posts/"+createdPostID, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, nonOwnerID, "user")
		w := httptest.NewRecorder()

		handler.Update(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	// Non-owner cannot delete
	t.Run("NonOwnerCannotDelete", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1/posts/"+createdPostID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, nonOwnerID, "user")
		w := httptest.NewRecorder()

		handler.Delete(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d", w.Code)
		}
	})

	// Owner can still update
	t.Run("OwnerCanUpdate", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		body := map[string]interface{}{
			"title": "Legitimately Updated Title By Owner",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/v1/posts/"+createdPostID, bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, ownerID, "user")
		w := httptest.NewRecorder()

		handler.Update(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	// Admin can delete even if not owner
	t.Run("AdminCanDelete", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		req := httptest.NewRequest(http.MethodDelete, "/v1/posts/"+createdPostID, nil)
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, "admin-user", "admin")
		w := httptest.NewRecorder()

		handler.Delete(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status 204, got %d", w.Code)
		}
	})
}

// TestPostsCRUD_VotingFlow tests the voting lifecycle.
func TestPostsCRUD_VotingFlow(t *testing.T) {
	repo := NewMockPostsRepositoryForIntegration()
	handler := NewPostsHandler(repo)
	postOwnerID := "post-owner"
	voterID := "voter-user"

	var createdPostID string

	// Create a post first
	t.Run("CreatePost", func(t *testing.T) {
		body := map[string]interface{}{
			"type":        "idea",
			"title":       "Voting Flow Test Idea Title",
			"description": "This is a test idea to verify the voting flow works correctly in integration tests.",
			"tags":        []string{"voting", "test"},
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		req = addAuthContext(req, postOwnerID, "user")
		w := httptest.NewRecorder()

		handler.Create(w, req)

		if w.Code != http.StatusCreated {
			t.Fatalf("failed to create post: status %d", w.Code)
		}

		var resp map[string]interface{}
		json.NewDecoder(w.Body).Decode(&resp)
		data := resp["data"].(map[string]interface{})
		createdPostID = data["id"].(string)
	})

	// Vote upvote
	t.Run("Upvote", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		body := map[string]interface{}{
			"direction": "up",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/posts/"+createdPostID+"/vote", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, voterID, "user")
		w := httptest.NewRecorder()

		handler.Vote(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		// Verify vote was recorded
		if repo.vote == nil {
			t.Error("expected vote to be recorded")
		} else if repo.vote.Direction != "up" {
			t.Errorf("expected direction 'up', got '%s'", repo.vote.Direction)
		}
	})

	// Vote downvote (change vote)
	t.Run("ChangeVoteToDownvote", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		// Reset vote for testing
		repo.vote = nil

		body := map[string]interface{}{
			"direction": "down",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/posts/"+createdPostID+"/vote", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		req = addAuthContext(req, voterID, "user")
		w := httptest.NewRecorder()

		handler.Vote(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		if repo.vote == nil {
			t.Error("expected vote to be recorded")
		} else if repo.vote.Direction != "down" {
			t.Errorf("expected direction 'down', got '%s'", repo.vote.Direction)
		}
	})

	// Verify cannot vote on own post
	t.Run("CannotVoteOnOwnPost", func(t *testing.T) {
		if createdPostID == "" {
			t.Skip("no post created")
		}

		body := map[string]interface{}{
			"direction": "up",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/v1/posts/"+createdPostID+"/vote", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", createdPostID)
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))
		// Try to vote on own post
		req = addAuthContext(req, postOwnerID, "user")
		w := httptest.NewRecorder()

		handler.Vote(w, req)

		if w.Code != http.StatusForbidden {
			t.Errorf("expected status 403, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestPostsCRUD_ListAndPagination tests listing with filters and pagination.
func TestPostsCRUD_ListAndPagination(t *testing.T) {
	repo := NewMockPostsRepositoryForIntegration()
	handler := NewPostsHandler(repo)
	userID := "list-test-user"

	// Create multiple posts of different types
	t.Run("CreateMultiplePosts", func(t *testing.T) {
		posts := []map[string]interface{}{
			{
				"type":        "problem",
				"title":       "First Problem for List Test",
				"description": "Description for the first problem that is at least fifty characters long for validation.",
				"tags":        []string{"problem", "test"},
			},
			{
				"type":        "question",
				"title":       "Second Question for List Test",
				"description": "Description for the second question that is at least fifty characters long for validation.",
				"tags":        []string{"question", "test"},
			},
			{
				"type":        "idea",
				"title":       "Third Idea for List Test",
				"description": "Description for the third idea that is at least fifty characters long for validation purposes.",
				"tags":        []string{"idea", "test"},
			},
		}

		for _, postData := range posts {
			jsonBody, _ := json.Marshal(postData)

			req := httptest.NewRequest(http.MethodPost, "/v1/posts", bytes.NewReader(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			req = addAuthContext(req, userID, "user")
			w := httptest.NewRecorder()

			handler.Create(w, req)

			if w.Code != http.StatusCreated {
				t.Fatalf("failed to create post: status %d", w.Code)
			}
		}
	})

	// List all posts
	t.Run("ListAll", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/posts", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		data := resp["data"].([]interface{})
		if len(data) < 3 {
			t.Errorf("expected at least 3 posts, got %d", len(data))
		}
	})

	// Filter by type
	t.Run("FilterByType", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/posts?type=problem", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		// Verify type filter was applied
		if repo.listOpts.Type != models.PostTypeProblem {
			t.Errorf("expected type filter 'problem', got '%s'", repo.listOpts.Type)
		}
	})

	// Pagination
	t.Run("Pagination", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/posts?page=1&per_page=2", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		if repo.listOpts.Page != 1 {
			t.Errorf("expected page 1, got %d", repo.listOpts.Page)
		}
		if repo.listOpts.PerPage != 2 {
			t.Errorf("expected per_page 2, got %d", repo.listOpts.PerPage)
		}
	})

	// Per page max enforcement
	t.Run("PerPageMaxEnforced", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/posts?per_page=100", nil)
		w := httptest.NewRecorder()

		handler.List(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", w.Code)
		}

		if repo.listOpts.PerPage != 50 {
			t.Errorf("expected per_page capped at 50, got %d", repo.listOpts.PerPage)
		}
	})
}

// MockPostsRepositoryForIntegration extends MockPostsRepository with storage for integration tests.
type MockPostsRepositoryForIntegration struct {
	posts       map[string]*models.PostWithAuthor
	postsList   []models.PostWithAuthor
	listOpts    models.PostListOptions
	vote        *models.Vote
	voteErr     error
	postCounter int
}

func NewMockPostsRepositoryForIntegration() *MockPostsRepositoryForIntegration {
	return &MockPostsRepositoryForIntegration{
		posts:     make(map[string]*models.PostWithAuthor),
		postsList: []models.PostWithAuthor{},
	}
}

func (m *MockPostsRepositoryForIntegration) List(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
	m.listOpts = opts

	// Return all stored posts
	result := make([]models.PostWithAuthor, 0, len(m.posts))
	for _, p := range m.posts {
		// Apply type filter if specified
		if opts.Type != "" && p.Type != opts.Type {
			continue
		}
		// Apply status filter if specified
		if opts.Status != "" && p.Status != opts.Status {
			continue
		}
		result = append(result, *p)
	}

	return result, len(result), nil
}

func (m *MockPostsRepositoryForIntegration) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	post, exists := m.posts[id]
	if !exists {
		return nil, ErrPostNotFound
	}
	// Check if deleted
	if post.DeletedAt != nil {
		return nil, ErrPostNotFound
	}
	return post, nil
}

func (m *MockPostsRepositoryForIntegration) Create(ctx context.Context, post *models.Post) (*models.Post, error) {
	m.postCounter++
	post.ID = "integration-post-" + string(rune('0'+m.postCounter))
	post.CreatedAt = time.Now()
	post.UpdatedAt = time.Now()
	post.Status = models.PostStatusOpen

	postWithAuthor := &models.PostWithAuthor{
		Post: *post,
		Author: models.PostAuthor{
			Type:        post.PostedByType,
			ID:          post.PostedByID,
			DisplayName: "Test User",
		},
	}
	m.posts[post.ID] = postWithAuthor

	return post, nil
}

func (m *MockPostsRepositoryForIntegration) Update(ctx context.Context, post *models.Post) (*models.Post, error) {
	existing, exists := m.posts[post.ID]
	if !exists {
		return nil, ErrPostNotFound
	}

	// Update fields
	if post.Title != "" {
		existing.Title = post.Title
	}
	if post.Description != "" {
		existing.Description = post.Description
	}
	if len(post.Tags) > 0 {
		existing.Tags = post.Tags
	}
	existing.UpdatedAt = time.Now()

	return &existing.Post, nil
}

func (m *MockPostsRepositoryForIntegration) Delete(ctx context.Context, id string) error {
	post, exists := m.posts[id]
	if !exists {
		return ErrPostNotFound
	}

	// Soft delete
	now := time.Now()
	post.DeletedAt = &now

	return nil
}

func (m *MockPostsRepositoryForIntegration) Vote(ctx context.Context, postID string, voterType models.AuthorType, voterID string, direction string) error {
	if m.voteErr != nil {
		return m.voteErr
	}

	_, exists := m.posts[postID]
	if !exists {
		return ErrPostNotFound
	}

	m.vote = &models.Vote{
		TargetType: "post",
		TargetID:   postID,
		VoterType:  string(voterType),
		VoterID:    voterID,
		Direction:  direction,
	}
	return nil
}
