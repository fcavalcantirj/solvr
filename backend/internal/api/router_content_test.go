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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/auth"
)

// TestProblemsEndpoints verifies problems endpoints are wired.
func TestProblemsEndpoints(t *testing.T) {
	router := NewRouter(nil)

	t.Run("GET /v1/problems returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/problems", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}

		var resp map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
			t.Fatalf("Failed to decode response: %v", err)
		}

		// Should have data array
		if _, ok := resp["data"]; !ok {
			t.Error("Expected 'data' field in response")
		}
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

	t.Run("POST /v1/problems requires auth", func(t *testing.T) {
		reqBody := `{"title":"Test problem","description":"Test description","success_criteria":["Test passes"]}`
		req := httptest.NewRequest(http.MethodPost, "/v1/problems", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Without auth, should return 401
		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
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
	router := NewRouter(nil)

	t.Run("GET /v1/questions returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/questions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

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

	t.Run("GET /v1/questions/:id returns single question or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/questions/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent question, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/questions requires auth", func(t *testing.T) {
		reqBody := `{"title":"Test question","description":"Test description"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/questions", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/questions/:id/answers requires auth", func(t *testing.T) {
		reqBody := `{"content":"Test answer content"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/questions/test-id/answers", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestIdeasEndpoints verifies ideas endpoints are wired.
func TestIdeasEndpoints(t *testing.T) {
	router := NewRouter(nil)

	t.Run("GET /v1/ideas returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

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

	t.Run("GET /v1/ideas/:id returns single idea or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent idea, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/ideas requires auth", func(t *testing.T) {
		reqBody := `{"title":"Test idea","description":"Test description"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/ideas", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/ideas/:id/responses requires auth", func(t *testing.T) {
		reqBody := `{"content":"Test response","response_type":"build"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/ideas/test-id/responses", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestCommentsEndpoints verifies comments endpoints are wired.
func TestCommentsEndpoints(t *testing.T) {
	router := NewRouter(nil)

	t.Run("GET /v1/approaches/:id/comments returns list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/approaches/test-id/comments", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 200 with empty list (in-memory repo)
		if w.Code != http.StatusOK {
			t.Errorf("Expected status 200, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /v1/approaches/:id/comments requires auth", func(t *testing.T) {
		reqBody := `{"content":"Test comment"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/approaches/test-id/comments", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("DELETE /v1/comments/:id requires auth", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodDelete, "/v1/comments/test-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// TestTypeSpecificListEndpoints verifies that posts created via /v1/posts appear
// in type-specific list endpoints (/v1/problems, /v1/questions, /v1/ideas).
// Per FIX-020: Type-specific list endpoints should return posts of their type.
func TestTypeSpecificListEndpoints(t *testing.T) {
	router := NewRouter(nil)

	// Create a valid JWT token for auth
	token, err := createTestJWTToken("user-123", "testuser", "user")
	if err != nil {
		t.Fatalf("Failed to create test JWT: %v", err)
	}

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

	t.Run("Problem created via /v1/posts appears in /v1/problems", func(t *testing.T) {
		// Create a problem via /v1/posts
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

		// GET /v1/problems should include this problem
		w = authGet("/v1/problems")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/problems failed: %d - %s", w.Code, w.Body.String())
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
			t.Errorf("Problem with ID %s not found in /v1/problems response. Got %d items", postID, len(dataArr))
		}
	})

	t.Run("Question created via /v1/posts appears in /v1/questions", func(t *testing.T) {
		// Create a question via /v1/posts
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

		// GET /v1/questions should include this question
		w = authGet("/v1/questions")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/questions failed: %d - %s", w.Code, w.Body.String())
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
			t.Errorf("Question with ID %s not found in /v1/questions response. Got %d items", postID, len(dataArr))
		}
	})

	t.Run("Idea created via /v1/posts appears in /v1/ideas", func(t *testing.T) {
		// Create an idea via /v1/posts
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

		// GET /v1/ideas should include this idea
		w = authGet("/v1/ideas")
		if w.Code != http.StatusOK {
			t.Fatalf("GET /v1/ideas failed: %d - %s", w.Code, w.Body.String())
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
			t.Errorf("Idea with ID %s not found in /v1/ideas response. Got %d items", postID, len(dataArr))
		}
	})
}

// TestPostCommentsEndpoints verifies /v1/posts/:id/comments endpoints per FIX-019.
func TestPostCommentsEndpoints(t *testing.T) {
	router := NewRouter(nil)

	t.Run("GET /v1/posts/:id/comments returns list (no auth required)", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/posts/test-post-id/comments", nil)
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

	t.Run("POST /v1/posts/:id/comments requires auth", func(t *testing.T) {
		reqBody := `{"content":"Test comment on post"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/posts/test-post-id/comments", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusUnauthorized {
			t.Errorf("Expected status 401 without auth, got %d: %s", w.Code, w.Body.String())
		}
	})
}

// createTestJWTToken creates a test JWT token for router content tests.
// Uses the same secret as in router.go ("test-jwt-secret").
func createTestJWTToken(userID, username, role string) (string, error) {
	secret := "test-jwt-secret"
	return auth.GenerateJWT(secret, userID, username+"@example.com", role, time.Hour)
}
