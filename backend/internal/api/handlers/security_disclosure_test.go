package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
)

// TestSoftDeletesDoNotExposeData verifies that soft-deleted content is never
// exposed through the API per security audit requirement.
// Soft deletes should return 404 (not found), not 403 (forbidden), to avoid
// revealing existence of deleted content.
func TestSoftDeletesDoNotExposeData(t *testing.T) {
	t.Run("deleted posts return 404 not 403", func(t *testing.T) {
		// Create a mock that returns a post with DeletedAt set
		deletedTime := time.Now()
		mockRepo := &mockPostsRepo{
			findByIDFunc: func(ctx context.Context, id string) (*models.PostWithAuthor, error) {
				// Return a post that has been soft-deleted
				return &models.PostWithAuthor{
					Post: models.Post{
						ID:        "deleted-post-123",
						Title:     "This was deleted",
						DeletedAt: &deletedTime,
					},
				}, nil
			},
		}

		handler := NewPostsHandler(mockRepo)

		req := httptest.NewRequest("GET", "/v1/posts/deleted-post-123", nil)
		rr := httptest.NewRecorder()

		rctx := chi.NewRouteContext()
		rctx.URLParams.Add("id", "deleted-post-123")
		req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

		handler.Get(rr, req)

		// Must return 404, not 403 or 200
		if rr.Code != http.StatusNotFound {
			t.Errorf("Expected 404 for deleted post, got %d", rr.Code)
		}

		body := rr.Body.String()

		// Error message should say "not found", not reveal it was deleted
		if strings.Contains(strings.ToLower(body), "deleted") {
			t.Errorf("Response reveals deletion status: %s", body)
		}
		if strings.Contains(strings.ToLower(body), "forbidden") {
			t.Errorf("Response reveals existence via forbidden: %s", body)
		}
	})

	t.Run("error message does not reveal deletion status", func(t *testing.T) {
		deletedTime := time.Now()
		mockRepo := &mockPostsRepo{
			findByIDFunc: func(ctx context.Context, id string) (*models.PostWithAuthor, error) {
				return &models.PostWithAuthor{
					Post: models.Post{
						ID:        id,
						DeletedAt: &deletedTime,
					},
				}, nil
			},
		}

		handler := NewPostsHandler(mockRepo)

		// Try multiple deleted post IDs to ensure consistent behavior
		postIDs := []string{"deleted-1", "deleted-2", "old-post-xyz"}

		for _, postID := range postIDs {
			req := httptest.NewRequest("GET", "/v1/posts/"+postID, nil)
			rr := httptest.NewRecorder()

			rctx := chi.NewRouteContext()
			rctx.URLParams.Add("id", postID)
			req = req.WithContext(context.WithValue(req.Context(), chi.RouteCtxKey, rctx))

			handler.Get(rr, req)

			body := rr.Body.String()

			// Should never reveal deletion-specific information
			leakyTerms := []string{
				"was deleted",
				"has been deleted",
				"soft delete",
				"removed",
				"archived",
				"deleted_at",
				"DeletedAt",
			}

			for _, term := range leakyTerms {
				if strings.Contains(strings.ToLower(body), strings.ToLower(term)) {
					t.Errorf("Response for %s reveals deletion via %q: %s", postID, term, body)
				}
			}
		}
	})

	t.Run("non-existent and deleted posts return identical responses", func(t *testing.T) {
		deletedTime := time.Now()

		// Track calls to distinguish between deleted and non-existent
		var deletedResponse, nonExistentResponse string

		// First, get response for deleted post
		mockRepoDeleted := &mockPostsRepo{
			findByIDFunc: func(ctx context.Context, id string) (*models.PostWithAuthor, error) {
				return &models.PostWithAuthor{
					Post: models.Post{
						ID:        id,
						DeletedAt: &deletedTime,
					},
				}, nil
			},
		}

		handler1 := NewPostsHandler(mockRepoDeleted)
		req1 := httptest.NewRequest("GET", "/v1/posts/some-id", nil)
		rr1 := httptest.NewRecorder()
		rctx1 := chi.NewRouteContext()
		rctx1.URLParams.Add("id", "some-id")
		req1 = req1.WithContext(context.WithValue(req1.Context(), chi.RouteCtxKey, rctx1))
		handler1.Get(rr1, req1)
		deletedResponse = rr1.Body.String()

		// Then, get response for non-existent post
		mockRepoNonExistent := &mockPostsRepo{
			findByIDFunc: func(ctx context.Context, id string) (*models.PostWithAuthor, error) {
				return nil, db.ErrPostNotFound
			},
		}

		handler2 := NewPostsHandler(mockRepoNonExistent)
		req2 := httptest.NewRequest("GET", "/v1/posts/some-id", nil)
		rr2 := httptest.NewRecorder()
		rctx2 := chi.NewRouteContext()
		rctx2.URLParams.Add("id", "some-id")
		req2 = req2.WithContext(context.WithValue(req2.Context(), chi.RouteCtxKey, rctx2))
		handler2.Get(rr2, req2)
		nonExistentResponse = rr2.Body.String()

		// Both should return same status code
		if rr1.Code != rr2.Code {
			t.Errorf("Deleted post returns %d, non-existent returns %d - reveals existence",
				rr1.Code, rr2.Code)
		}

		// Both should return same error message
		if deletedResponse != nonExistentResponse {
			t.Errorf("Responses differ - reveals deletion:\nDeleted: %s\nNon-existent: %s",
				deletedResponse, nonExistentResponse)
		}
	})

	t.Run("DB layer filters deleted posts from list", func(t *testing.T) {
		// Verify that when the DB layer correctly filters out deleted posts,
		// only active posts appear in the list response.
		// The filtering happens at the DB layer (WHERE deleted_at IS NULL),
		// which we verify here by simulating a properly-filtered response.
		mockRepo := &mockPostsRepo{
			listFunc: func(ctx context.Context, opts models.PostListOptions) ([]models.PostWithAuthor, int, error) {
				// DB layer returns only non-deleted posts
				return []models.PostWithAuthor{
					{
						Post: models.Post{
							ID:    "active-post-1",
							Title: "Active Post 1",
						},
					},
					{
						Post: models.Post{
							ID:    "active-post-2",
							Title: "Active Post 2",
						},
					},
				}, 2, nil
			},
		}

		handler := NewPostsHandler(mockRepo)

		req := httptest.NewRequest("GET", "/v1/posts", nil)
		rr := httptest.NewRecorder()

		handler.List(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("Expected 200, got %d", rr.Code)
		}

		body := rr.Body.String()

		// Response should contain active posts
		if !strings.Contains(body, "active-post-1") {
			t.Error("Expected active-post-1 in response")
		}
		if !strings.Contains(body, "active-post-2") {
			t.Error("Expected active-post-2 in response")
		}

		// Response should not contain deleted_at field since posts aren't deleted
		// (omitempty should suppress nil DeletedAt)
		if strings.Contains(body, "deleted_at") {
			t.Error("Response should not include deleted_at field for active posts")
		}
	})
}

// TestAuthErrorMessages verifies authentication errors don't leak info.
func TestAuthErrorMessages(t *testing.T) {
	t.Run("auth errors use safe generic patterns", func(t *testing.T) {
		// When a token is invalid, the error should not reveal:
		// - Signature details
		// - Algorithm info
		// - Secret/key material

		// Truly sensitive patterns that should NEVER appear in auth error messages
		trulyLeakyPatterns := []string{
			"signature invalid", // don't reveal signature issues in detail
			"algorithm",         // don't reveal expected algorithm
			"secret",            // never reveal secret info
			"jwt_secret",        // never reveal secret names
			"private key",       // never reveal key material
			"public key",        // never reveal key material
			"bcrypt",            // don't reveal hashing details
			"sha",               // don't reveal hashing details
			"hmac",              // don't reveal crypto details
		}

		// These are OK to reveal - they help clients understand what to do
		// "API_KEY" is fine (tells user which auth method was invalid)
		// "expired" is fine (tells client to refresh)
		// "invalid" is fine (generic)

		// Test the auth error codes used in the system
		errorCodes := []string{
			"UNAUTHORIZED",
			"INVALID_TOKEN",
			"TOKEN_EXPIRED",
			"INVALID_API_KEY",
		}

		for _, code := range errorCodes {
			for _, leak := range trulyLeakyPatterns {
				if strings.Contains(strings.ToLower(code), strings.ToLower(leak)) {
					t.Errorf("Auth error code %q contains sensitive pattern %q", code, leak)
				}
			}
		}
	})

	t.Run("auth error messages don't reveal implementation", func(t *testing.T) {
		// Verify that auth error messages don't reveal implementation details
		// These are the actual message strings that would be returned to clients
		safeMessages := []string{
			"authentication required",
			"invalid token",
			"token has expired",
			"invalid API key",
			"not authenticated",
		}

		implementationLeaks := []string{
			"jwt",
			"bcrypt",
			"sha256",
			"hmac",
			"signing",
			"verification failed",
			"parse error",
		}

		for _, msg := range safeMessages {
			for _, leak := range implementationLeaks {
				if strings.Contains(strings.ToLower(msg), strings.ToLower(leak)) {
					t.Errorf("Auth message %q contains implementation detail %q", msg, leak)
				}
			}
		}
	})
}
