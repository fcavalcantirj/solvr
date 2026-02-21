package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// testCommentsSetup creates an agent and returns the API key.
// Reuses the setupTestRouter pattern — no new setup code.
func testCommentsSetup(t *testing.T, router interface{ ServeHTTP(http.ResponseWriter, *http.Request) }) string {
	t.Helper()
	name := fmt.Sprintf("cmt_test_%d", time.Now().UnixNano()%1000000)
	body := fmt.Sprintf(`{"name":%q,"description":"comments count test agent"}`, name)
	req := httptest.NewRequest(http.MethodPost, "/v1/agents/register", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("agent registration failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("decode registration response: %v", err)
	}
	apiKey, _ := resp["api_key"].(string)
	if apiKey == "" {
		t.Fatal("expected api_key in registration response")
	}
	return apiKey
}

// TestCommentsCount_ProblemsShowCountAfterComment verifies:
// 1. Create a problem
// 2. Add a comment via POST /v1/posts/{id}/comments
// 3. GET /v1/problems → the specific problem shows comments_count == 1
func TestCommentsCount_ProblemsShowCountAfterComment(t *testing.T) {
	router := setupTestRouter(t)
	apiKey := testCommentsSetup(t, router)

	// Create problem
	groqThrottle(t)
	title := fmt.Sprintf("Test problem for comments count %d", time.Now().UnixNano()%100000)
	postBody := fmt.Sprintf(`{"type":"problem","title":%q,"description":"E2E test verifying that comments_count is correctly returned for problems in the listing endpoint"}`, title)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(postBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+apiKey)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create problem failed: %d %s", createW.Code, createW.Body.String())
	}
	var createResp map[string]any
	json.NewDecoder(createW.Body).Decode(&createResp)
	data, _ := createResp["data"].(map[string]any)
	postID, _ := data["id"].(string)
	if postID == "" {
		t.Fatal("expected post id in response")
	}

	// Wait for content moderation to approve the post (async GROQ call).
	// Posts start as pending_review; listings only show open posts.
	if !waitForPostOpen(t, router, postID, "Bearer "+apiKey) {
		t.Skip("post did not become open within 35s - GROQ rate limited or slow")
	}

	// Add a comment
	cmtURL := fmt.Sprintf("/v1/posts/%s/comments", postID)
	cmtReq := httptest.NewRequest(http.MethodPost, cmtURL, strings.NewReader(`{"content":"test comment on problem"}`))
	cmtReq.Header.Set("Content-Type", "application/json")
	cmtReq.Header.Set("Authorization", "Bearer "+apiKey)
	cmtW := httptest.NewRecorder()
	router.ServeHTTP(cmtW, cmtReq)
	if cmtW.Code != http.StatusCreated {
		t.Fatalf("create comment failed: %d %s", cmtW.Code, cmtW.Body.String())
	}

	// GET /v1/problems and find our post
	listReq := httptest.NewRequest(http.MethodGet, "/v1/problems?sort=newest&per_page=50", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list problems failed: %d %s", listW.Code, listW.Body.String())
	}

	var listResp map[string]any
	json.NewDecoder(listW.Body).Decode(&listResp)
	items, _ := listResp["data"].([]any)

	found := false
	for _, item := range items {
		p, _ := item.(map[string]any)
		if p["id"] == postID {
			found = true
			cnt, _ := p["comments_count"].(float64)
			if int(cnt) != 1 {
				t.Errorf("expected comments_count=1 for problem, got %v", p["comments_count"])
			}
			break
		}
	}
	if !found {
		t.Errorf("problem %s not found in /v1/problems listing", postID)
	}
}

// TestCommentsCount_IdeasShowCountAfterComment verifies ideas listing shows correct comments_count.
func TestCommentsCount_IdeasShowCountAfterComment(t *testing.T) {
	router := setupTestRouter(t)
	apiKey := testCommentsSetup(t, router)

	// Create idea
	groqThrottle(t)
	title := fmt.Sprintf("Test idea for comments count %d", time.Now().UnixNano()%100000)
	postBody := fmt.Sprintf(`{"type":"idea","title":%q,"description":"E2E test verifying that comments_count is correctly returned for ideas in the listing endpoint"}`, title)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(postBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+apiKey)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create idea failed: %d %s", createW.Code, createW.Body.String())
	}
	var createResp map[string]any
	json.NewDecoder(createW.Body).Decode(&createResp)
	data, _ := createResp["data"].(map[string]any)
	postID, _ := data["id"].(string)
	if postID == "" {
		t.Fatal("expected post id in response")
	}

	// Wait for content moderation to approve the post (async GROQ call).
	if !waitForPostOpen(t, router, postID, "Bearer "+apiKey) {
		t.Skip("post did not become open within 35s - GROQ rate limited or slow")
	}

	// Add a comment
	cmtURL := fmt.Sprintf("/v1/posts/%s/comments", postID)
	cmtReq := httptest.NewRequest(http.MethodPost, cmtURL, strings.NewReader(`{"content":"test comment on idea"}`))
	cmtReq.Header.Set("Content-Type", "application/json")
	cmtReq.Header.Set("Authorization", "Bearer "+apiKey)
	cmtW := httptest.NewRecorder()
	router.ServeHTTP(cmtW, cmtReq)
	if cmtW.Code != http.StatusCreated {
		t.Fatalf("create comment failed: %d %s", cmtW.Code, cmtW.Body.String())
	}

	// GET /v1/ideas and find our post
	listReq := httptest.NewRequest(http.MethodGet, "/v1/ideas?sort=newest&per_page=50", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list ideas failed: %d %s", listW.Code, listW.Body.String())
	}

	var listResp map[string]any
	json.NewDecoder(listW.Body).Decode(&listResp)
	items, _ := listResp["data"].([]any)

	found := false
	for _, item := range items {
		p, _ := item.(map[string]any)
		if p["id"] == postID {
			found = true
			cnt, _ := p["comments_count"].(float64)
			if int(cnt) != 1 {
				t.Errorf("expected comments_count=1 for idea, got %v", p["comments_count"])
			}
			break
		}
	}
	if !found {
		t.Errorf("idea %s not found in /v1/ideas listing", postID)
	}
}

// TestCommentsCount_FeedShowsCommentCount verifies /v1/posts (feed) shows comments_count.
func TestCommentsCount_FeedShowsCommentCount(t *testing.T) {
	router := setupTestRouter(t)
	apiKey := testCommentsSetup(t, router)

	// Create question
	groqThrottle(t)
	title := fmt.Sprintf("Test question for feed comments %d", time.Now().UnixNano()%100000)
	postBody := fmt.Sprintf(`{"type":"question","title":%q,"description":"E2E test verifying that comments_count is correctly returned for all post types in the feed"}`, title)
	createReq := httptest.NewRequest(http.MethodPost, "/v1/posts", strings.NewReader(postBody))
	createReq.Header.Set("Content-Type", "application/json")
	createReq.Header.Set("Authorization", "Bearer "+apiKey)
	createW := httptest.NewRecorder()
	router.ServeHTTP(createW, createReq)
	if createW.Code != http.StatusCreated {
		t.Fatalf("create question failed: %d %s", createW.Code, createW.Body.String())
	}
	var createResp map[string]any
	json.NewDecoder(createW.Body).Decode(&createResp)
	data, _ := createResp["data"].(map[string]any)
	postID, _ := data["id"].(string)
	if postID == "" {
		t.Fatal("expected post id in response")
	}

	// Wait for content moderation to approve the post (async GROQ call).
	if !waitForPostOpen(t, router, postID, "Bearer "+apiKey) {
		t.Skip("post did not become open within 35s - GROQ rate limited or slow")
	}

	// Add 2 comments
	for i := 0; i < 2; i++ {
		cmtURL := fmt.Sprintf("/v1/posts/%s/comments", postID)
		body := fmt.Sprintf(`{"content":"feed comment %d"}`, i+1)
		cmtReq := httptest.NewRequest(http.MethodPost, cmtURL, strings.NewReader(body))
		cmtReq.Header.Set("Content-Type", "application/json")
		cmtReq.Header.Set("Authorization", "Bearer "+apiKey)
		cmtW := httptest.NewRecorder()
		router.ServeHTTP(cmtW, cmtReq)
		if cmtW.Code != http.StatusCreated {
			t.Fatalf("create comment %d failed: %d %s", i+1, cmtW.Code, cmtW.Body.String())
		}
	}

	// GET /v1/posts and verify comments_count == 2
	listReq := httptest.NewRequest(http.MethodGet, "/v1/posts?sort=newest&per_page=50", nil)
	listW := httptest.NewRecorder()
	router.ServeHTTP(listW, listReq)
	if listW.Code != http.StatusOK {
		t.Fatalf("list posts failed: %d %s", listW.Code, listW.Body.String())
	}

	var listResp map[string]any
	json.NewDecoder(listW.Body).Decode(&listResp)
	items, _ := listResp["data"].([]any)

	found := false
	for _, item := range items {
		p, _ := item.(map[string]any)
		if p["id"] == postID {
			found = true
			cnt, _ := p["comments_count"].(float64)
			if int(cnt) != 2 {
				t.Errorf("expected comments_count=2 in feed, got %v", p["comments_count"])
			}
			break
		}
	}
	if !found {
		t.Errorf("post %s not found in /v1/posts listing", postID)
	}
}
