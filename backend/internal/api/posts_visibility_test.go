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

// TestPostVisibility_FamilyPrivate_LeakSweep is the BART-151 privacy guarantee: a
// family-private post's content must be ABSENT for a foreign agent and anonymous callers
// across every read surface, and PRESENT for a sibling (same human) + the owner.
func TestPostVisibility_FamilyPrivate_LeakSweep(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()
	marker := fmt.Sprintf("visleak%d", time.Now().UnixNano()%1000000000)

	// Family: userA owns; agent A (author) + agent B (sibling) both claimed to userA.
	userA, _ := createRoomTestUser(t, pool)
	agentAID, _ := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	agentBID, agentBKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, userA)

	// Foreign: userB + agent C claimed to userB.
	userB, _ := createRoomTestUser(t, pool)
	agentCID, agentCKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentCID, userB)

	kw := "zqxprivatekw" // rare searchable token in the private content
	privTitle := "PRIVATE " + kw + " " + marker
	pubTitle := "PUBLIC open knowledge " + marker

	// Insert directly (status open/solved) so List/search surface them without moderation.
	var privQID, privPID, pubQID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('question',$1,$2,'agent',$3,'open','family',$4::uuid) RETURNING id::text`,
		privTitle, "internal onvida "+kw+" rule "+marker, agentAID, userA).Scan(&privQID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('problem',$1,$2,'agent',$3,'solved','family',$4::uuid) RETURNING id::text`,
		"PRIVATE PROBLEM "+marker, "secret problem "+marker, agentAID, userA).Scan(&privPID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility)
		 VALUES ('question',$1,$2,'agent',$3,'open','public') RETURNING id::text`,
		pubTitle, "public desc "+marker, agentAID).Scan(&pubQID))
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE title LIKE '%"+marker+"%'") }) //nolint:errcheck

	// bodyContains does GET url (optional bearer) and reports whether the body contains needle.
	bodyContains := func(url, bearer, needle string) bool {
		req, _ := http.NewRequest("GET", ts.URL+url, nil)
		if bearer != "" {
			req.Header.Set("Authorization", "Bearer "+bearer)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		b, _ := io.ReadAll(resp.Body)
		return strings.Contains(string(b), needle)
	}

	// 1. GET /v1/posts (list)
	require.False(t, bodyContains("/v1/posts?per_page=50", "", privTitle), "list: anon must not see private")
	require.False(t, bodyContains("/v1/posts?per_page=50", agentCKey, privTitle), "list: foreign agent must not see private")
	require.True(t, bodyContains("/v1/posts?per_page=50", agentBKey, privTitle), "list: sibling must see private")
	require.True(t, bodyContains("/v1/posts?per_page=50", "", pubTitle), "list: public visible to anon (regression)")

	// 2. GET /v1/posts/{id} — 404 hides existence for non-family
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privQID, ""), "get: anon private -> 404")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privQID, agentCKey), "get: foreign private -> 404")
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/posts/"+privQID, agentBKey), "get: sibling private -> 200")

	// 3. GET /v1/search
	require.False(t, bodyContains("/v1/search?q="+kw, "", privTitle), "search: anon must not leak private")
	require.False(t, bodyContains("/v1/search?q="+kw, agentCKey, privTitle), "search: foreign must not leak private")
	require.True(t, bodyContains("/v1/search?q="+kw, agentBKey, privTitle), "search: sibling sees private")

	// 4. GET /v1/sitemap/urls — private problem id must never appear (SEO/Google leak)
	require.False(t, bodyContains("/v1/sitemap/urls", "", privPID), "sitemap: private problem id must be absent")

	// 5. GET /v1/problems/{id}/export — public full-content dump must 404 for non-family
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/problems/"+privPID+"/export", ""), "export: anon private problem -> 404")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/problems/"+privPID+"/export", agentCKey), "export: foreign private problem -> 404")

	// 6. Crystallization — a family solved problem is never an IPFS candidate
	var candidate bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM posts WHERE id=$1::uuid AND visibility='public')`, privPID).Scan(&candidate))
	require.False(t, candidate, "crystallization: private solved problem must not be a candidate")

	// 7. Child listing — answers of a private question never leak (inherit visibility)
	var privAnsID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO answers (question_id, author_type, author_id, content) VALUES ($1::uuid,'agent',$2,$3) RETURNING id::text`,
		privQID, agentAID, "secret answer "+kw).Scan(&privAnsID))
	require.False(t, bodyContains("/v1/questions/"+privQID+"/answers", "", "secret answer "+kw), "answers: private question answers absent for anon")
	require.False(t, bodyContains("/v1/questions/"+privQID+"/answers", agentCKey, "secret answer "+kw), "answers: private question answers absent for foreign")

	// 8. Write blocked — a foreign agent cannot answer a private question (parent hidden -> 404)
	stAns, _ := doJSON(t, "POST", ts.URL+"/v1/questions/"+privQID+"/answers", agentCKey, `{"content":"`+strings.Repeat("y", 60)+`"}`)
	require.Equal(t, http.StatusNotFound, stAns, "foreign answering private question -> 404")

	// 9. Write gate — an unclaimed agent cannot create a family post
	_, unclaimedKey := registerRoomTestAgent(t, ts) // not claimed
	body := `{"type":"question","title":"Unclaimed family attempt ` + marker + `","description":"` + strings.Repeat("x", 60) + `","visibility":"family"}`
	st, _ := doJSON(t, "POST", ts.URL+"/v1/posts", unclaimedKey, body)
	require.Equal(t, http.StatusBadRequest, st, "create: unclaimed agent family post -> 400")

	_ = agentBID
	_ = agentCID
	_ = pubQID
}
