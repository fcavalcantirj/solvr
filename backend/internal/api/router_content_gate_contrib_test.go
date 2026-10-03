package api

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W1 on the real router, contribution routes: a body the same author already has
// live — as a reply, answer, approach, response or comment, on any post — is a 409.

// seedOpenPost inserts a published, approved post of postType by a throwaway author, the way
// moderation leaves one, so contributions can target it.
func seedOpenPost(t *testing.T, pool *db.Pool, postType string) string {
	t.Helper()
	// The post names an existing author (000117).
	_, err := pool.Exec(context.Background(), `INSERT INTO agents (id, display_name, status)
		VALUES ('agent_gate_seed', 'agent_gate_seed', 'active') ON CONFLICT (id) DO NOTHING`)
	require.NoError(t, err)
	var id string
	require.NoError(t, pool.QueryRow(context.Background(), `
		INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state)
		VALUES ($1, $2, 'A seeded target post for contribution gate tests.', 'agent', 'agent_gate_seed', 'open', 'published', 'approved')
		RETURNING id::text`, postType, "Gate target "+uuid.NewString()).Scan(&id))
	t.Cleanup(func() { pool.Exec(context.Background(), "DELETE FROM posts WHERE id = $1::uuid", id) }) //nolint:errcheck
	return id
}

func seedLegacy(t *testing.T, pool *db.Pool, sql string, args ...any) string {
	t.Helper()
	var id string
	require.NoError(t, pool.QueryRow(context.Background(), sql, args...).Scan(&id), sql)
	return id
}

func deleteContributionsBy(t *testing.T, pool *db.Pool, authorID string) {
	t.Cleanup(func() {
		ctx := context.Background()
		for _, table := range []string{"replies", "answers", "responses", "comments"} {
			pool.Exec(ctx, "DELETE FROM "+table+" WHERE author_id = $1", authorID) //nolint:errcheck
		}
		pool.Exec(ctx, "DELETE FROM progress_notes WHERE approach_id IN (SELECT id FROM approaches WHERE author_id = $1)", authorID) //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM approaches WHERE author_id = $1", authorID)                                                      //nolint:errcheck
	})
}

// contribAgent registers a fresh agent whose contributions are removed when the test ends.
func contribAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (string, string) {
	t.Helper()
	id, key := uniqueTestAgent(t, ts, pool)
	deleteContributionsBy(t, pool, id)
	return id, key
}

// seedMigratedReply inserts a live reply by authorID on postID shaped like the ones the
// contribution cutover wrote from a legacy row (legacyType and a legacy id of its own): since
// the legacy duplicate branches went with the legacy tables (idx 68), the author's former
// answers, approaches, responses and comments reach the gate only as these replies.
func seedMigratedReply(t *testing.T, pool *db.Pool, postID, legacyType, authorID, body string) string {
	t.Helper()
	return seedLegacy(t, pool, `INSERT INTO replies (post_id, author_type, author_id, body, legacy_type, legacy_id)
		VALUES ($1, 'agent', $2, $3, $4, gen_random_uuid()) RETURNING id::text`, postID, authorID, body, legacyType)
}

// The NaoParis case: an author's answer body repeated as a reply on a different post. The
// legacy answer route is retired (task idx 52) and the answer is a migrated reply now; the
// canonical reply route must still find it.
func TestContentGate_SameAuthorBodyOnADifferentPost(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := contribAgent(t, ts, pool)
	qa, post := seedOpenPost(t, pool, "question"), seedOpenPost(t, pool, "post")
	text := "Great question! Check out my service at example.dev for the full fix " + uuid.NewString()[:8]
	answer := seedMigratedReply(t, pool, qa, "answer", agentID, text)

	// Cross-kind: the same text as a canonical reply on another post.
	requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+post+"/replies", fmt.Sprintf(`{"body":%q}`, text)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", answer)

	unique := gateCall(t, ts, key, "/v1/posts/"+post+"/replies", fmt.Sprintf(`{"body":"Set a context deadline on the client %s."}`, uuid.NewString()))
	require.Equal(t, http.StatusCreated, unique.status, "a unique reply passes: %s", unique.body)
}

// POST /v1/posts/{id}/replies runs the gate against the author's replies, native and migrated
// from legacy approaches and responses alike.
func TestContentGate_ReplyApproachResponseProgressRoutes(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := contribAgent(t, ts, pool)
	marker := uuid.NewString()[:8]

	// POST /v1/posts/{id}/replies
	p1, p2 := seedOpenPost(t, pool, "post"), seedOpenPost(t, pool, "post")
	reply := fmt.Sprintf(`{"body":"Pin the dependency and rebuild the lockfile %s."}`, marker)
	first := gateCall(t, ts, key, "/v1/posts/"+p1+"/replies", reply)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+p2+"/replies", reply), http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)

	// A reply migrated from an approach, repeated as a reply.
	method := "Wrap the client in a retry budget " + marker + "."
	approach := seedMigratedReply(t, pool, seedOpenPost(t, pool, "problem"), "approach", agentID, method)
	requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+p2+"/replies", fmt.Sprintf(`{"body":%q}`, method)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", approach)

	// A reply migrated from a response, repeated as a reply.
	content := "This would pair well with a shared retry budget " + marker + "."
	response := seedMigratedReply(t, pool, seedOpenPost(t, pool, "idea"), "response", agentID, content)
	requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+p2+"/replies", fmt.Sprintf(`{"body":%q}`, content)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", response)
}

// A comment's content, repeated as a reply, is refused wherever the comment was: the four
// comment create routes are retired (task idx 52) and the author's comments are migrated
// replies now.
func TestContentGate_CommentRoutes(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := contribAgent(t, ts, pool)
	q, post := seedOpenPost(t, pool, "question"), seedOpenPost(t, pool, "post")
	for _, target := range []string{q, post} {
		content := fmt.Sprintf("+1, same issue here since the upgrade (%s) %s", target, uuid.NewString()[:8])
		comment := seedMigratedReply(t, pool, target, "comment", agentID, content)
		requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+post+"/replies", fmt.Sprintf(`{"body":%q}`, content)),
			http.StatusConflict, "DUPLICATE_CONTENT", "", comment)
	}
}
