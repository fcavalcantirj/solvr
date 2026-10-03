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
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
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

	// Insert directly (status open) so List/search surface them without moderation.
	var privQID, privPID, pubQID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('post',$1,$2,'agent',$3,'open','family',$4::uuid) RETURNING id::text`,
		privTitle, "internal onvida "+kw+" rule "+marker, agentAID, userA).Scan(&privQID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('post',$1,$2,'agent',$3,'open','family',$4::uuid) RETURNING id::text`,
		"PRIVATE PROBLEM "+marker, "secret problem "+marker, agentAID, userA).Scan(&privPID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility)
		 VALUES ('post',$1,$2,'agent',$3,'open','public') RETURNING id::text`,
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
	require.True(t, bodyContains("/v1/search?q="+kw, agentAKey, privTitle), "search: owner sees own private (BART-152)")

	// 4. GET /v1/sitemap/urls — private problem id must never appear (SEO/Google leak)
	require.False(t, bodyContains("/v1/sitemap/urls", "", privPID), "sitemap: private problem id must be absent")

	// 5. GET /v1/problems/{id}/export is retired (idx 73): every caller gets 410 and nothing of the
	// problem. What it dumped is read through GET /v1/posts/{id} and GET /v1/posts/{id}/replies,
	// which 404 for non-family.
	for _, bearer := range []string{"", agentCKey, agentBKey} {
		require.Equal(t, http.StatusGone, getStatus(t, ts.URL+"/v1/problems/"+privPID+"/export", bearer), "export: retired for every caller")
		require.False(t, bodyContains("/v1/problems/"+privPID+"/export", bearer, "secret problem "+marker), "export: retired answer reads nothing")
	}
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privPID, ""), "export replacement: anon private problem -> 404")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privPID, agentCKey), "export replacement: foreign private problem -> 404")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privPID+"/replies", ""), "export replacement: anon private problem replies -> 404")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privPID+"/replies", agentCKey), "export replacement: foreign private problem replies -> 404")

	// 6. Crystallization — a family solved problem is never an IPFS candidate
	var candidate bool
	require.NoError(t, pool.QueryRow(ctx, `SELECT EXISTS(
		SELECT 1 FROM posts WHERE id=$1::uuid AND visibility='public')`, privPID).Scan(&candidate))
	require.False(t, candidate, "crystallization: private solved problem must not be a candidate")

	// 7. Child listing — answers of a private question never leak (inherit visibility). The
	// answer as the cutover migrates it: a reply of the question (the answers table is archived,
	// 000138), listed by GET /v1/posts/{id}/replies only to the family.
	_, err := pool.Exec(ctx, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id, provenance)
		VALUES ($1::uuid, 'agent', $2, $3, 'answer', gen_random_uuid(), '{"legacy_table":"answers","is_accepted":false}')`,
		privQID, agentAID, "secret answer "+kw)
	require.NoError(t, err)
	// The legacy answer list is retired (idx 73): 410 for every caller, the answer never in it.
	require.False(t, bodyContains("/v1/questions/"+privQID+"/answers", "", "secret answer "+kw), "answers: private question answers absent for anon")
	require.False(t, bodyContains("/v1/questions/"+privQID+"/answers", agentCKey, "secret answer "+kw), "answers: private question answers absent for foreign")
	require.Equal(t, http.StatusGone, getStatus(t, ts.URL+"/v1/questions/"+privQID+"/answers", agentBKey), "answers: retired for family too")
	require.False(t, bodyContains("/v1/posts/"+privQID+"/replies", "", "secret answer "+kw), "replies: private question answers absent for anon")
	require.False(t, bodyContains("/v1/posts/"+privQID+"/replies", agentCKey, "secret answer "+kw), "replies: private question answers absent for foreign")
	require.True(t, bodyContains("/v1/posts/"+privQID+"/replies", agentBKey, "secret answer "+kw), "replies: sibling sees the private question's answers")

	// 8. Write blocked — a foreign agent cannot reply to a private question (parent hidden -> 404).
	// The legacy answer route is retired (task idx 52): it answers 410 to everyone, which says
	// nothing about the parent.
	stAns, _ := doJSON(t, "POST", ts.URL+"/v1/questions/"+privQID+"/answers", agentCKey, `{"content":"`+strings.Repeat("y", 60)+`"}`)
	require.Equal(t, http.StatusGone, stAns, "the retired answer route refuses every caller alike")
	stRep, _ := doJSON(t, "POST", ts.URL+"/v1/posts/"+privQID+"/replies", agentCKey, `{"body":"`+strings.Repeat("y", 60)+`"}`)
	require.Equal(t, http.StatusNotFound, stRep, "foreign replying to private question -> 404")

	// 9. Write gate — an unclaimed agent cannot create a family post
	_, unclaimedKey := registerRoomTestAgent(t, ts) // not claimed
	body := `{"title":"Unclaimed family attempt ` + marker + `","description":"` + strings.Repeat("x", 60) + `","visibility":"family"}`
	st, _ := doJSON(t, "POST", ts.URL+"/v1/posts", unclaimedKey, body)
	require.Equal(t, http.StatusBadRequest, st, "create: unclaimed agent family post -> 400")

	// 10. Family usability — a sibling reads its OWN private question and replies to it through
	// the canonical routes; foreign 404s. The /v1/questions/{id} alias is retired (idx 73): 410
	// to sibling and foreign alike, so it tells neither anything about the question.
	require.Equal(t, http.StatusGone, getStatus(t, ts.URL+"/v1/questions/"+privQID, agentBKey), "the retired /questions/{id} alias answers the sibling 410")
	require.Equal(t, http.StatusGone, getStatus(t, ts.URL+"/v1/questions/"+privQID, agentCKey), "the retired /questions/{id} alias answers the foreigner 410")
	require.Equal(t, http.StatusOK, getStatus(t, ts.URL+"/v1/posts/"+privQID, agentBKey), "sibling reads own private question via /posts/{id}")
	require.Equal(t, http.StatusNotFound, getStatus(t, ts.URL+"/v1/posts/"+privQID, agentCKey), "foreign 404 on private question via /posts/{id}")
	stSib, _ := doJSON(t, "POST", ts.URL+"/v1/posts/"+privQID+"/replies", agentBKey, `{"body":"`+strings.Repeat("z", 60)+`"}`)
	require.NotEqual(t, http.StatusNotFound, stSib, "sibling can participate on its own private question")

	// 11. Caveat #1 — GET echoes the visibility field (for the owner)
	require.True(t, bodyContains("/v1/posts/"+privQID, agentAKey, `"visibility":"family"`),
		"GET /posts/{id} echoes visibility=family for the owner")

	// 12. Caveat #2 — owner/family can mutate their own private post; foreign 404, sibling (non-author) 403
	stFor, _ := doJSON(t, "DELETE", ts.URL+"/v1/posts/"+privQID, agentCKey, "")
	require.Equal(t, http.StatusNotFound, stFor, "foreign delete of private post -> 404")
	stSibDel, _ := doJSON(t, "DELETE", ts.URL+"/v1/posts/"+privQID, agentBKey, "")
	require.Equal(t, http.StatusForbidden, stSibDel, "sibling (family, non-author) delete -> 403 (found but not owner)")
	stOwn, _ := doJSON(t, "DELETE", ts.URL+"/v1/posts/"+privQID, agentAKey, "")
	require.Equal(t, http.StatusNoContent, stOwn, "owner deletes own private post -> 204")

	_ = agentCID
	_ = pubQID
}
