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
)

// TestProblemsEndpoints verifies problems endpoints are wired.
func TestProblemsEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/problems is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		// idx 73: the typed list is retired; GET /v1/posts lists these posts
		// (TestTypeSpecificListEndpoints, router_legacy_discovery_test.go).
		req := httptest.NewRequest(http.MethodGet, "/v1/problems", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/problems")
	})

	t.Run("GET /v1/problems/:id is retired; GET /v1/posts/:id answers 404 for a nonexistent post", func(t *testing.T) {
		// idx 73: the single-problem read is retired; GET /v1/posts/{id} reads the same post
		// (TestRetiredTypedReads_CanonicalReadsServeWhatTheRouteServed).
		req := httptest.NewRequest(http.MethodGet, "/v1/problems/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requireRetiredRecorder(t, w, "GET /v1/problems/{id}")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/nonexistent-id", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Should return 404 for nonexistent, not 500 or route error
		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent post, got %d: %s", w.Code, w.Body.String())
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

	t.Run("GET /v1/problems/:id/approaches is retired; GET /v1/posts/:id/replies returns list or 404", func(t *testing.T) {
		// idx 73: approaches are replies; GET /v1/posts/{id}/replies lists them.
		req := httptest.NewRequest(http.MethodGet, "/v1/problems/test-problem-id/approaches", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requireRetiredRecorder(t, w, "GET /v1/problems/{id}/approaches")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/test-problem-id/replies", nil)
		w = httptest.NewRecorder()
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
		// idx 73: the typed list is retired; GET /v1/posts lists these posts
		// (TestTypeSpecificListEndpoints, router_legacy_discovery_test.go).
		req := httptest.NewRequest(http.MethodGet, "/v1/questions", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/questions")
	})

	t.Run("GET /v1/questions/:id is retired; GET /v1/posts/:id answers 404 for a nonexistent post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/questions/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requireRetiredRecorder(t, w, "GET /v1/questions/{id}")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/nonexistent-id", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent post, got %d: %s", w.Code, w.Body.String())
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

	// FIX-022 (viewing answers before answering): answers are replies now (idx 73), listed by
	// GET /v1/posts/{id}/replies without authentication.
	t.Run("GET /v1/questions/:id/answers is retired; GET /v1/posts/:id/replies returns list or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/questions/test-question-id/answers", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requireRetiredRecorder(t, w, "GET /v1/questions/{id}/answers")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/test-question-id/replies", nil)
		w = httptest.NewRecorder()
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
		// idx 73: the typed list is retired; GET /v1/posts lists these posts
		// (TestTypeSpecificListEndpoints, router_legacy_discovery_test.go).
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/ideas")
	})

	t.Run("GET /v1/ideas/:id is retired; GET /v1/posts/:id answers 404 for a nonexistent post", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas/nonexistent-id", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requireRetiredRecorder(t, w, "GET /v1/ideas/{id}")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/nonexistent-id", nil)
		w = httptest.NewRecorder()
		router.ServeHTTP(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("Expected status 404 for nonexistent post, got %d: %s", w.Code, w.Body.String())
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

	// FIX-024 (viewing responses before responding): responses are replies now (idx 73), listed
	// by GET /v1/posts/{id}/replies without authentication.
	t.Run("GET /v1/ideas/:id/responses is retired; GET /v1/posts/:id/replies returns list or 404", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/ideas/test-idea-id/responses", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		requireRetiredRecorder(t, w, "GET /v1/ideas/{id}/responses")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/test-idea-id/replies", nil)
		w = httptest.NewRecorder()
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
	body := fmt.Sprintf(`{"title":"Comment list wiring target %d","description":"A problem that exists so the comment list route answers with a list of its comments"}`, time.Now().UnixNano())
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

	t.Run("GET /v1/approaches/:id/comments is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		// An approach that exists still gets the migration error (idx 73 step 3): its comments
		// are replies, listed by TestRetiredCommentLists_RepliesListWhatTheRouteListed.
		apiKey := testCommentsSetup(t, router)
		approachID := createCommentTargetApproach(t, createCommentTargetProblem(t, router, apiKey))
		req := httptest.NewRequest(http.MethodGet, "/v1/approaches/"+approachID+"/comments", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/approaches/{id}/comments")
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

// TestPostCommentsEndpoints verifies /v1/posts/:id/comments endpoints per FIX-019.
func TestPostCommentsEndpoints(t *testing.T) {
	router := setupTestRouter(t)

	t.Run("GET /v1/posts/:id/comments is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		// A post that exists still gets the migration error (idx 73 step 3); its replies list
		// (GET /v1/posts/{id}/replies) answers 200 with a data field.
		postID := createCommentTargetProblem(t, router, testCommentsSetup(t, router))
		req := httptest.NewRequest(http.MethodGet, "/v1/posts/"+postID+"/comments", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "GET /v1/posts/{id}/comments")

		req = httptest.NewRequest(http.MethodGet, "/v1/posts/"+postID+"/replies", nil)
		w = httptest.NewRecorder()
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

	t.Run("POST /v1/posts/:id/comments is retired (410 ENDPOINT_RETIRED)", func(t *testing.T) {
		reqBody := `{"content":"Test comment on post"}`
		req := httptest.NewRequest(http.MethodPost, "/v1/posts/00000000-0000-0000-0000-000000000001/comments", strings.NewReader(reqBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		requireRetiredRecorder(t, w, "POST /v1/posts/{id}/comments")
	})
}
