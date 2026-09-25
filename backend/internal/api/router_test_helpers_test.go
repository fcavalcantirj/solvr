package api

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

	"github.com/fcavalcantirj/solvr/internal/auth"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
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
	return NewRouter(pool, nil, nil)
}

// createLiveTestUser creates a live account with role through the repository every signup
// uses and returns its id and a JWT for it, removing the account (and its API keys) when the
// test ends. Authentication accepts a JWT only while its account is live, so a JWT for an
// invented user id is refused with 401. Unique fields come from a fresh UUID, so rows other
// tests leave in a shared test database cannot collide.
func createLiveTestUser(t *testing.T, pool *db.Pool, role string) (userID, jwt string) {
	t.Helper()
	suffix := strings.ReplaceAll(uuid.NewString(), "-", "")
	username := "stc_" + suffix[:12]
	user, err := db.NewUserRepository(pool).Create(context.Background(), &models.User{
		Username:       username,
		DisplayName:    "Live Test User",
		Email:          username + "@test.solvr.dev",
		AuthProvider:   models.AuthProviderGitHub,
		AuthProviderID: "stc_" + suffix,
		Role:           role,
	})
	require.NoError(t, err, "create test user")
	jwt, err = auth.GenerateJWT("test-jwt-secret-32-chars-long!!", user.ID, user.Email, role, time.Hour)
	require.NoError(t, err)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM user_api_keys WHERE user_id = $1", user.ID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM users WHERE id = $1", user.ID)              //nolint:errcheck
	})
	return user.ID, jwt
}

// liveTestUserJWT is createLiveTestUser for tests that hold only a router.
func liveTestUserJWT(t *testing.T, role string) string {
	t.Helper()
	pool, err := db.NewPool(context.Background(), os.Getenv("DATABASE_URL"))
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	_, jwt := createLiveTestUser(t, pool, role)
	return jwt
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
