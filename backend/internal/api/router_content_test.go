package api

/**
 * Tests for content endpoints: problems, questions, ideas, comments.
 *
 * Per PRD-v2 API-CRITICAL requirement:
 * - Wire GET/POST /v1/problems, /v1/problems/{id}/approaches
 * - Wire GET/POST /v1/questions, /v1/questions/{id}/answers
 * - Wire GET/POST /v1/ideas, /v1/ideas/{id}/responses
 * - Wire GET/POST/DELETE /v1/comments
 */

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// TestProblemsEndpoints verifies problems endpoints are wired.
func TestProblemsEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/problems is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		// idx 73: the typed list is retired; GET /v1/posts?type=problem lists these posts
		// (TestTypeSpecificListEndpoints, router_legacy_discovery_test.go).
		req := httptest.NewRequest(http.MethodGet, "/v1/problems", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/problems")
	})

	t.Run("GET /v1/problems/:id returns single problem or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/problems/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 404 for nonexistent, not 500 or route error
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent problem, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/problems is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"title":"Test problem","description":"Test description","success_criteria":["Test passes"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/problems", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/problems")
	})

	t.Run("GET /v1/problems/:id/approaches returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/problems/test-problem-id/approaches", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 404 for nonexistent problem, or 200 with empty list
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("Expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestQuestionsEndpoints verifies questions endpoints are wired.
func TestQuestionsEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/questions is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		// idx 73: the typed list is retired; GET /v1/posts?type=question lists these posts
		// (TestTypeSpecificListEndpoints, router_legacy_discovery_test.go).
		req := httptest.NewRequest(http.MethodGet, "/v1/questions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/questions")
	})

	t.Run("GET /v1/questions/:id returns single question or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/questions/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent question, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/questions is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"title":"Test question","description":"Test description"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/questions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/questions")
	})

	t.Run("POST /v1/questions/:id/answers is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"content":"Test answer content"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/questions/test-id/answers", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/questions/{id}/answers")
	})

	// FIX-022: Test GET /v1/questions/:id/answers endpoint
	t.Run("GET /v1/questions/:id/answers returns list or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/questions/test-question-id/answers", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 404 for nonexistent question, or 200 with list
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("Expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestIdeasEndpoints verifies ideas endpoints are wired.
func TestIdeasEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/ideas is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		// idx 73: the typed list is retired; GET /v1/posts?type=idea lists these posts
		// (TestTypeSpecificListEndpoints, router_legacy_discovery_test.go).
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/ideas")
	})

	t.Run("GET /v1/ideas/:id returns single idea or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent idea, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/ideas is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"title":"Test idea","description":"Test description"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/ideas", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/ideas")
	})

	t.Run("POST /v1/ideas/:id/responses is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"content":"Test response","response_type":"build"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/ideas/test-id/responses", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/ideas/{id}/responses")
	})

	// FIX-024: Test GET /v1/ideas/:id/responses endpoint
	t.Run("GET /v1/ideas/:id/responses returns list or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas/test-idea-id/responses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 404 for nonexistent idea, or 200 with list
		if w.Code != http.StatusOK && w.Code != http.StatusNotFound {
			t.Errorf("Expected status 200 or 404, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// createCommentTargetProblem creates a problem with apiKey and returns its id.
func createCommentTargetProblem(t *testing.T, router http.Handler, apiKey string) string {
	t.Helper()
	body := fmt.Sprintf(`{"type":"problem","title":"Comment list wiring target %d","description":"A problem that exists so the comment list route answers with a list of its comments"}`, time.Now().UnixNano())
	return createCommentTarget(t, router, apiKey, "/v1/posts", body)
}

// createCommentTargetApproach inserts an approach on problemID and returns its id. The legacy
// approach create route is retired (task idx 52), so the row the comment list reads is seeded.
func createCommentTargetApproach(t *testing.T, problemID string) string {
	t.Helper()
	ctx := context.Background()
	pool, err := db.NewPool(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	t.Cleanup(pool.Close)
	var id string
	if err := pool.QueryRow(ctx, `INSERT INTO approaches (problem_id, author_type, author_id, angle)
		VALUES ($1::uuid, 'agent', 'agent_comment_list_seed', 'Comment list wiring approach') RETURNING id::text`, problemID).Scan(&id); err != nil {
		t.Fatalf("seed approach: %v", err)
	}
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM approaches WHERE id = $1::uuid", id) }) //nolint:errcheck
	return id
}

func createCommentTarget(t *testing.T, router http.Handler, apiKey, path, body string) string {
	t.Helper()
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+apiKey)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("POST %s: %d %s", path, w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			ID string `json:"id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil || resp.Data.ID == "" {
		t.Fatalf("POST %s: no id in response (%v)", path, err)
	}
	return resp.Data.ID
}

// TestCommentsEndpoints verifies comments endpoints are wired.
func TestCommentsEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/approaches/:id/comments returns list", func(t *testing.T) {
		// An approach that exists: a missing one is 404 (TestStatusContract_UnknownResourceIDIsNotFound).
		apiKey := testCommentsSetup(t, router)
		approachID := createCommentTargetApproach(t, createCommentTargetProblem(t, router, apiKey))
		req := httptest.NewRequest(http.MethodGet, "/v1/approaches/"+approachID+"/comments", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 with empty list
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/approaches/:id/comments is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"content":"Test comment"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/approaches/00000000-0000-0000-0000-000000000001/comments", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/approaches/{id}/comments")
	})

	t.Run("DELETE /v1/comments/:id is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/comments/test-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "DELETE /v1/comments/{id}")
	})
}

// TestTypeSpecificListEndpoints verifies that posts created via /v1/posts appear
// in the type-specific lists, GET /v1/posts?type=problem|question|idea.
// Per FIX-020: Type-specific list endpoints should return posts of their type. The legacy
// typed lists (/v1/problems, /v1/questions, /v1/ideas) served exactly these queries and are
// retired (idx 73 step 3), so the lists are read where the retirement points.
func TestTypeSpecificListEndpoints(t *testing.T) {
	liftCreateLimits(t)      // many creates by one identity; the hourly limit is not this test's subject
	useRecordingModerator(t) // legacy creates are moderated now (anti-abuse W2); the mock approves
	router := setupTestRouter(t)

	// Create a valid JWT token for auth (its account must exist and be live)
	token := liveTestUserJWT(t, models.UserRoleUser)

	// Helper to make authenticated requests
	authPost := func(path, body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	authGet := func(path string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w
	}

	t.Run("Problem created via /v1/posts appears in /v1/posts?type=problem", func(t *testing.T) {
		// Create a problem via /v1/posts
		groqThrottle(t)
		body := `{
			"type": "problem",
			"title": "Test problem title for listing",
			"description": "This is a test problem description that needs to be long enough to pass validation, so here is some extra text to make it long enough.",
			"success_criteria": ["Test passes"]
		}`
		w := authPost("/v1/posts", body)
		if w.Code != http.StatusCreated {
			t.Fatalf("Failed to create problem: %d - %s", w.Code, w.Body.String())
		}

		// Extract the created post ID
		var createResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}
		data := createResp["data"].(map[string]interface{})
		postID := data["id"].(string)

		// Wait for content moderation to approve the post (async GROQ call).
		if !waitForPostOpen(t, router, postID, "Bearer "+token) {
			t.Skipf("post %s did not become open - GROQ rate limited or slow", postID)
		}

		// GET /v1/posts?type=problem should include this problem
		w = authGet("/v1/posts?type=problem")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/posts?type=problem failed: %d - %s", w.Code, w.Body.String())
		}

		var listResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
			t.Fatalf("Failed to decode list response: %v", err)
		}

		dataArr := listResp["data"].([]interface{})
		found := false
		for _, item := range dataArr {
			post := item.(map[string]interface{})
			if post["id"] == postID {
				found = true
				// Verify it's the right type
				if post["type"] != "problem" {
					t.Errorf("Expected type 'problem', got %v", post["type"])
				}
				break
			}
		}
		if !found {
			t.Errorf("Problem with ID %s not found in /v1/posts?type=problem response. Got %d items", postID, len(dataArr))
		}
	})

	t.Run("Question created via /v1/posts appears in /v1/posts?type=question", func(t *testing.T) {
		// Create a question via /v1/posts
		groqThrottle(t)
		body := `{
			"type": "question",
			"title": "Test question title for listing",
			"description": "This is a test question description that needs to be long enough to pass validation, so here is some extra text to make it long enough."
		}`
		w := authPost("/v1/posts", body)
		if w.Code != http.StatusCreated {
			t.Fatalf("Failed to create question: %d - %s", w.Code, w.Body.String())
		}

		var createResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}
		data := createResp["data"].(map[string]interface{})
		postID := data["id"].(string)

		// Wait for content moderation to approve the post (async GROQ call).
		if !waitForPostOpen(t, router, postID, "Bearer "+token) {
			t.Skipf("post %s did not become open - GROQ rate limited or slow", postID)
		}

		// GET /v1/posts?type=question should include this question
		w = authGet("/v1/posts?type=question")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/posts?type=question failed: %d - %s", w.Code, w.Body.String())
		}

		var listResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
			t.Fatalf("Failed to decode list response: %v", err)
		}

		dataArr := listResp["data"].([]interface{})
		found := false
		for _, item := range dataArr {
			post := item.(map[string]interface{})
			if post["id"] == postID {
				found = true
				if post["type"] != "question" {
					t.Errorf("Expected type 'question', got %v", post["type"])
				}
				break
			}
		}
		if !found {
			t.Errorf("Question with ID %s not found in /v1/posts?type=question response. Got %d items", postID, len(dataArr))
		}
	})

	t.Run("Idea created via /v1/posts appears in /v1/posts?type=idea", func(t *testing.T) {
		// Create an idea via /v1/posts
		groqThrottle(t)
		body := `{
			"type": "idea",
			"title": "Test idea title for listing",
			"description": "This is a test idea description that needs to be long enough to pass validation, so here is some extra text to make it long enough."
		}`
		w := authPost("/v1/posts", body)
		if w.Code != http.StatusCreated {
			t.Fatalf("Failed to create idea: %d - %s", w.Code, w.Body.String())
		}

		var createResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&createResp); err != nil {
			t.Fatalf("Failed to decode create response: %v", err)
		}
		data := createResp["data"].(map[string]interface{})
		postID := data["id"].(string)

		// Wait for content moderation to approve the post (async GROQ call).
		if !waitForPostOpen(t, router, postID, "Bearer "+token) {
			t.Skipf("post %s did not become open - GROQ rate limited or slow", postID)
		}

		// GET /v1/posts?type=idea should include this idea
		w = authGet("/v1/posts?type=idea")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/posts?type=idea failed: %d - %s", w.Code, w.Body.String())
		}

		var listResp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&listResp); err != nil {
			t.Fatalf("Failed to decode list response: %v", err)
		}

		dataArr := listResp["data"].([]interface{})
		found := false
		for _, item := range dataArr {
			post := item.(map[string]interface{})
			if post["id"] == postID {
				found = true
				if post["type"] != "idea" {
					t.Errorf("Expected type 'idea', got %v", post["type"])
				}
				break
			}
		}
		if !found {
			t.Errorf("Idea with ID %s not found in /v1/posts?type=idea response. Got %d items", postID, len(dataArr))
		}
	})
}

// TestPostCommentsEndpoints verifies /v1/posts/:id/comments endpoints per FIX-019.
func TestPostCommentsEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/posts/:id/comments returns list (no auth required)", func(t *testing.T) {
		// A post that exists: a missing one is 404 (TestStatusContract_UnknownResourceIDIsNotFound).
		postID := createCommentTargetProblem(t, router, testCommentsSetup(t, router))
		req := httptest.NewRequest(http.MethodGet, "/v1/posts/"+postID+"/comments", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 with empty list or valid response
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		if _, ok := resp["data"]; !ok {
			t.Error("Expected 'data' field in response")
		}
	})

	t.Run("POST /v1/posts/:id/comments is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"content":"Test comment on post"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/posts/00000000-0000-0000-0000-000000000001/comments", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/posts/{id}/comments")
	})
}
