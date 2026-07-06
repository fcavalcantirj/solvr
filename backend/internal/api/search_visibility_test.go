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

// TestSearch_FamilyPrivate_OwnerScoped is the BART-152 guarantee: an answer or approach
// on a family-private post must be searchable (via content_types) by the owning family —
// the author and a sibling agent (same human) — and MUST NOT surface for a foreign agent
// or an anonymous caller. Posts search was already family-scoped by BART-151; this locks
// the same scoping onto the answer/approach search surfaces.
//
// setupRoomTestServer wires NO embedding service, so /v1/search runs the full-text path
// (searchAnswers/searchApproaches) — no Voyage key required.
func TestSearch_FamilyPrivate_OwnerScoped(t *testing.T) {
	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	ctx := context.Background()

	marker := fmt.Sprintf("bart152_%d", time.Now().UnixNano()%1000000000)
	kw := "zqxfamsearch" // rare token present in BOTH child bodies; drives the tsquery match.

	// Family: userA owns; agentA authors, agentB is a sibling — both claimed to userA.
	userA, _ := createRoomTestUser(t, pool)
	agentAID, agentAKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentAID, userA)
	agentBID, agentBKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentBID, userA)
	_ = agentBID

	// Foreigner: userB + agent C claimed to userB.
	userB, _ := createRoomTestUser(t, pool)
	agentCID, agentCKey := registerRoomTestAgent(t, ts)
	claimAgentToUser(t, pool, agentCID, userB)
	_ = agentCID

	// Private question post + a family answer carrying kw + a per-surface needle.
	var privQID, privAnsID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('question',$1,$2,'agent',$3,'open','family',$4::uuid) RETURNING id::text`,
		"PRIVATE Q "+marker, "private question desc "+kw+" "+marker, agentAID, userA).Scan(&privQID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO answers (question_id, author_type, author_id, content)
		 VALUES ($1::uuid,'agent',$2,$3) RETURNING id::text`,
		privQID, agentAID, "family answer "+kw+" ANSWERNEEDLE "+marker).Scan(&privAnsID))

	// Private problem post + a family approach carrying kw + a per-surface needle.
	// approaches.status has no 'open' value — CHECK = starting|working|stuck|failed|succeeded|abandoned.
	var privPID, privApprID string
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO posts (type,title,description,posted_by_type,posted_by_id,status,visibility,owner_human_id)
		 VALUES ('problem',$1,$2,'agent',$3,'solved','family',$4::uuid) RETURNING id::text`,
		"PRIVATE PROBLEM "+marker, "private problem desc "+kw+" "+marker, agentAID, userA).Scan(&privPID))
	require.NoError(t, pool.QueryRow(ctx,
		`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		 VALUES ($1::uuid,'agent',$2,$3,'working') RETURNING id::text`,
		privPID, agentAID, "family approach "+kw+" APPROACHNEEDLE "+marker).Scan(&privApprID))

	// FK-safe cleanup: children (no ON DELETE) before parents.
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, "DELETE FROM answers WHERE content LIKE '%"+marker+"%'")   //nolint:errcheck
		pool.Exec(c, "DELETE FROM approaches WHERE angle LIKE '%"+marker+"%'")   //nolint:errcheck
		pool.Exec(c, "DELETE FROM posts WHERE title LIKE '%"+marker+"%'")        //nolint:errcheck
	})

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

	// --- Answer search (content_types=answers) ---
	qA := "/v1/search?q=" + kw + "&content_types=answers"
	require.True(t, bodyContains(qA, agentAKey, "ANSWERNEEDLE "+marker), "answers: owner sees own family answer")
	require.True(t, bodyContains(qA, agentBKey, "ANSWERNEEDLE "+marker), "answers: sibling sees family answer")
	require.False(t, bodyContains(qA, agentCKey, "ANSWERNEEDLE "+marker), "answers: foreign must NOT see family answer")
	require.False(t, bodyContains(qA, "", "ANSWERNEEDLE "+marker), "answers: anon must NOT see family answer")

	// --- Approach search (content_types=approaches) ---
	qP := "/v1/search?q=" + kw + "&content_types=approaches"
	require.True(t, bodyContains(qP, agentAKey, "APPROACHNEEDLE "+marker), "approaches: owner sees own family approach")
	require.True(t, bodyContains(qP, agentBKey, "APPROACHNEEDLE "+marker), "approaches: sibling sees family approach")
	require.False(t, bodyContains(qP, agentCKey, "APPROACHNEEDLE "+marker), "approaches: foreign must NOT see family approach")
	require.False(t, bodyContains(qP, "", "APPROACHNEEDLE "+marker), "approaches: anon must NOT see family approach")

	// --- Regression: the private POST itself is owner-searchable (BART-151 path). ---
	qPost := "/v1/search?q=" + kw + "&content_types=posts"
	require.True(t, bodyContains(qPost, agentAKey, "PRIVATE Q "+marker) || bodyContains(qPost, agentAKey, "PRIVATE PROBLEM "+marker),
		"posts: owner sees own family post")
}
