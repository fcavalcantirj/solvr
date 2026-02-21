package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/go-chi/chi/v5"
)

// groqThrottle sleeps 2 seconds to respect GROQ's 30 RPM rate limit.
// Per GROQ docs: 30 RPM = 1 request every 2 seconds minimum.
// Call before any post creation that triggers content moderation.
func groqThrottle(t *testing.T) {
	t.Helper()
	time.Sleep(2 * time.Second)
}

// setupTestRouter creates a router with a real database connection.
// Skips the test if DATABASE_URL is not set.
func setupTestRouter(t *testing.T) *chi.Mux {
	t.Helper()
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		t.Skip("DATABASE_URL not set, skipping integration test")
	}
	pool, err := db.NewPool(context.Background(), dbURL)
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}
	t.Cleanup(func() { pool.Close() })
	return NewRouter(pool)
}

// waitForPostOpen polls GET /v1/posts/:id until the post status is "open" (moderation approved).
// Required when GROQ content moderation is enabled: posts start as pending_review and become open async.
// Times out after 35 seconds. Returns true if the post became open, false if it timed out.
// Call this after creating a post and before checking listings; use t.Skipf if it returns false.
func waitForPostOpen(t *testing.T, router interface{ ServeHTTP(http.ResponseWriter, *http.Request) }, postID, authHeader string) bool {
	t.Helper()
	deadline := time.Now().Add(35 * time.Second)
	for time.Now().Before(deadline) {
		req := httptest.NewRequest(http.MethodGet, fmt.Sprintf("/v1/posts/%s", postID), nil)
		if authHeader != "" {
			req.Header.Set("Authorization", authHeader)
		}
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		if w.Code == http.StatusOK {
			var resp map[string]interface{}
			if err := json.NewDecoder(w.Body).Decode(&resp); err == nil {
				if data, ok := resp["data"].(map[string]interface{}); ok {
					if data["status"] == "open" {
						return true
					}
				}
			}
		}
		time.Sleep(300 * time.Millisecond)
	}
	t.Logf("waitForPostOpen: post %s did not become open within 35s (GROQ may be slow or GROQ_API_KEY not set)", postID)
	return false
}
