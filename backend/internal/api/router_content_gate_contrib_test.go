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

// The NaoParis case: the same answer body on a different question, then as a reply elsewhere.
func TestContentGate_SameAuthorBodyOnADifferentPost(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, key := contribAgent(t, ts, pool)
	qa, qb, post := seedOpenPost(t, pool, "question"), seedOpenPost(t, pool, "question"), seedOpenPost(t, pool, "post")
	text := "Great question! Check out my service at example.dev for the full fix " + uuid.NewString()[:8]
	body := fmt.Sprintf(`{"content":%q}`, text)

	first := gateCall(t, ts, key, "/v1/questions/"+qa+"/answers", body)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/questions/"+qb+"/answers", body),
		http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)

	// Cross-kind: the same text as a canonical reply on another post.
	requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+post+"/replies", fmt.Sprintf(`{"body":%q}`, text)),
		http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)

	unique := gateCall(t, ts, key, "/v1/questions/"+qb+"/answers", fmt.Sprintf(`{"content":"Set a context deadline on the client %s."}`, uuid.NewString()))
	require.Equal(t, http.StatusCreated, unique.status, "a unique answer passes: %s", unique.body)
}

func TestContentGate_ReplyApproachResponseProgressRoutes(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, key := contribAgent(t, ts, pool)
	marker := uuid.NewString()[:8]

	// POST /v1/posts/{id}/replies
	p1, p2 := seedOpenPost(t, pool, "post"), seedOpenPost(t, pool, "post")
	reply := fmt.Sprintf(`{"body":"Pin the dependency and rebuild the lockfile %s."}`, marker)
	first := gateCall(t, ts, key, "/v1/posts/"+p1+"/replies", reply)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/posts/"+p2+"/replies", reply), http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)

	// POST /v1/problems/{id}/approaches
	pr1, pr2 := seedOpenPost(t, pool, "problem"), seedOpenPost(t, pool, "problem")
	approach := fmt.Sprintf(`{"angle":"Bound the retries","method":"Wrap the client in a retry budget %s."}`, marker)
	first = gateCall(t, ts, key, "/v1/problems/"+pr1+"/approaches", approach)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/problems/"+pr2+"/approaches", approach), http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)

	// POST /v1/approaches/{id}/progress (on the approach just created)
	note := fmt.Sprintf(`{"content":"Budget of three retries holds under load %s."}`, marker)
	firstNote := gateCall(t, ts, key, "/v1/approaches/"+first.id+"/progress", note)
	require.Equal(t, http.StatusCreated, firstNote.status, firstNote.body)
	refusedNote := gateCall(t, ts, key, "/v1/approaches/"+first.id+"/progress", note)
	require.Equal(t, http.StatusConflict, refusedNote.status, refusedNote.body)
	require.Equal(t, "DUPLICATE_CONTENT", refusedNote.code)

	// POST /v1/ideas/{id}/responses
	i1, i2 := seedOpenPost(t, pool, "idea"), seedOpenPost(t, pool, "idea")
	response := fmt.Sprintf(`{"content":"This would pair well with a shared retry budget %s.","response_type":"build"}`, marker)
	first = gateCall(t, ts, key, "/v1/ideas/"+i1+"/responses", response)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	requireRefused(t, gateCall(t, ts, key, "/v1/ideas/"+i2+"/responses", response), http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
}

// All four comment routes run the gate.
func TestContentGate_CommentRoutes(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	_, key := contribAgent(t, ts, pool)
	q, pr, idea, post := seedOpenPost(t, pool, "question"), seedOpenPost(t, pool, "problem"), seedOpenPost(t, pool, "idea"), seedOpenPost(t, pool, "post")
	answer := seedLegacy(t, pool, `INSERT INTO answers (question_id, author_type, author_id, content) VALUES ($1, 'agent', 'agent_gate_seed', 'seed answer') RETURNING id::text`, q)
	approach := seedLegacy(t, pool, `INSERT INTO approaches (problem_id, author_type, author_id, angle) VALUES ($1, 'agent', 'agent_gate_seed', 'seed angle') RETURNING id::text`, pr)
	response := seedLegacy(t, pool, `INSERT INTO responses (idea_id, author_type, author_id, content, response_type) VALUES ($1, 'agent', 'agent_gate_seed', 'seed response', 'build') RETURNING id::text`, idea)
	comment := fmt.Sprintf(`{"content":"+1, same issue here since the upgrade %s"}`, uuid.NewString()[:8])

	first := gateCall(t, ts, key, "/v1/answers/"+answer+"/comments", comment)
	require.Equal(t, http.StatusCreated, first.status, first.body)
	for _, path := range []string{"/v1/approaches/" + approach, "/v1/responses/" + response, "/v1/posts/" + post, "/v1/answers/" + answer} {
		requireRefused(t, gateCall(t, ts, key, path+"/comments", comment), http.StatusConflict, "DUPLICATE_CONTENT", "", first.id)
	}
}
