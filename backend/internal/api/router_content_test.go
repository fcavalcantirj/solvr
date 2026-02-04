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
