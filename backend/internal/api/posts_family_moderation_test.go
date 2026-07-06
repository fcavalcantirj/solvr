package api

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestCreateFamilyPost_SkipsModeration_InstantSearch is the BART-154 guarantee: a family
// post is created status=open (skips moderation) and is immediately full-text searchable by
// its owner, while a public post is created pending_review and — with no moderation service
// wired (setupRoomTestServer wires none, mirroring the "no GROQ" prod risk) — stays out of
// search. This is read-your-write for the family fast path.
func TestCreateFamilyPost_SkipsModeration_InstantSearch(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()

	n := time.Now().UnixNano() % 1000000000
	marker := fmt.Sprintf("bart154_%d", n)
	famKW := fmt.Sprintf("zqxfammod%d", n) // rare lexeme unique to the family post
	pubKW := fmt.Sprintf("zqxpubmod%d", n)  // rare lexeme unique to the public post

	// userA owns; agentA is claimed to userA so it may create family posts (posts.go:550).
	userA, _ := createRoomTestUser(t, pool)
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM posts WHERE title LIKE '%"+marker+"%'") //nolint:errcheck
	})

	searchContains := func(kw, bearer, needle string) bool {
		req, _ := http.NewRequest("GET", ts.URL+"/v1/search?q="+kw, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return strings.Contains(string(b), needle)
	}

	// --- Family post: 201 + status=open (skips moderation) ---
	famTitle := "Family private problem " + famKW + " " + marker
	famDesc := "Internal onvida runbook detail " + famKW + " " + strings.Repeat("x", 40)
	famBody := fmt.Sprintf(`{"type":"problem","title":%q,"description":%q,"visibility":"family"}`, famTitle, famDesc)
	st, out := doJSON(t, "POST", ts.URL+"/v1/posts", agentAKey, famBody)
	require.Equal(t, http.StatusCreated, st, "family create should 201: %v", out)
	famData, _ := out["data"].(map[string]any)
	require.Equal(t, "open", famData["status"], "family post must be created open (skips moderation)")

	// --- Public post: 201 + status=pending_review (unchanged) ---
	pubTitle := "Public shared problem " + pubKW + " " + marker
	pubDesc := "Public knowledge writeup " + pubKW + " " + strings.Repeat("y", 40)
	pubBody := fmt.Sprintf(`{"type":"problem","title":%q,"description":%q,"visibility":"public"}`, pubTitle, pubDesc)
	st, out = doJSON(t, "POST", ts.URL+"/v1/posts", agentAKey, pubBody)
	require.Equal(t, http.StatusCreated, st, "public create should 201: %v", out)
	pubData, _ := out["data"].(map[string]any)
	require.Equal(t, "pending_review", pubData["status"], "public post must start pending_review")

	// --- Search: family present (open + owner-scoped), public absent (pending_review) ---
	require.True(t, searchContains(famKW, agentAKey, famTitle),
		"family post must be full-text searchable immediately by its owner (read-your-write)")
	require.False(t, searchContains(pubKW, agentAKey, pubTitle),
		"public pending_review post must be absent from search (no moderation service to approve it)")
}
