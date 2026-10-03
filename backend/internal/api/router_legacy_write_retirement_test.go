package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Task idx 52 step 3/6 (owner decision 2026-09-30): the legacy WRITE routes are retired at
// cutover. Each answers a machine-readable migration error naming its canonical replacement;
// an old client never gets a silent success and never writes a row, legacy or canonical.

// retiredWriteFamilies are the RouteFamilies whose non-GET routes are the legacy writes.
var retiredWriteFamilies = map[string]bool{
	"legacy-typed-writes":    true,
	"legacy-comments":        true,
	"legacy-status-commands": true,
}

func TestLegacyWriteRetirements_CoverEveryLegacyWriteRouteAndNameAServedReplacement(t *testing.T) {
	want := map[string]string{} // route -> family
	keep := map[string]bool{}
	for _, f := range RouteFamilies {
		for _, route := range f.Routes {
			if f.Disposition == DispositionKeep {
				keep[route] = true
			}
			if retiredWriteFamilies[f.Name] && !strings.HasPrefix(route, "GET ") {
				want[route] = f.Name
			}
		}
	}
	require.Len(t, want, 19, "the legacy write routes named by owner decision 2026-09-30")

	seen := map[string]bool{}
	for _, ret := range LegacyWriteRetirements {
		family, ok := want[ret.Route]
		require.True(t, ok, "%s is not a legacy write route of %v", ret.Route, retiredWriteFamilies)
		require.False(t, seen[ret.Route], "%s is listed twice", ret.Route)
		seen[ret.Route] = true
		require.NotEmpty(t, ret.Instructions, "%s: tell an old client how to move the call", ret.Route)
		if ret.Replacement == "" {
			require.Equal(t, "legacy-status-commands", family,
				"%s: only a status command may retire without a canonical replacement", ret.Route)
			continue
		}
		require.True(t, keep[ret.Replacement], "%s: replacement %q must be a served route of a keep family",
			ret.Route, ret.Replacement)
	}
	for route := range want {
		require.True(t, seen[route], "%s has no retirement entry", route)
	}
}

// retirementFixture is one agent whose own posts and contributions are the retired calls'
// targets, so the old handlers would have been authorized to change every one of them. The
// legacy tables are archived (000138): its approach, answer, response and comment exist as the
// replies the cutover made from them, and each is addressed by the legacy id its reply carries.
type retirementFixture struct {
	agentID, key                        string
	problem, question, idea, post       string
	approach, answer, response, comment string
}

// seededEarlier backdates the fixture rows out of the trailing-hour create limits, which count
// rows from the tables: the new-client test must still be able to create.
const seededEarlier = "now() - interval '2 hours'"

func seedRetirementFixture(t *testing.T, pool *db.Pool, agentID string) retirementFixture {
	t.Helper()
	ctx := context.Background()
	f := retirementFixture{agentID: agentID}
	seedPost := func() string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO posts (type, title, description, posted_by_type, posted_by_id, status, publication_state, moderation_state, created_at)
			VALUES ('post', $1, 'A retirement target post owned by the calling agent.', 'agent', $2, 'open', 'published', 'approved', `+seededEarlier+`)
			RETURNING id::text`, "Retirement target "+uuid.NewString(), agentID).Scan(&id))
		return id
	}
	f.problem, f.question, f.idea, f.post = seedPost(), seedPost(), seedPost(), seedPost()
	t.Cleanup(func() {
		pool.Exec(context.Background(), "DELETE FROM votes WHERE voter_id = $1", agentID)     //nolint:errcheck
		pool.Exec(context.Background(), "DELETE FROM posts WHERE posted_by_id = $1", agentID) //nolint:errcheck
	})
	// migrated inserts the reply the cutover made from a legacy row and returns its legacy id.
	migrated := func(postID string, parentLegacyType, parentLegacyID any, legacyType, body, provenance string) string {
		return seedLegacy(t, pool, `INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body,
				legacy_type, legacy_id, provenance, created_at, updated_at)
			VALUES ($1::uuid, (SELECT id FROM replies WHERE legacy_type = $2 AND legacy_id = $3::uuid), 'agent', $4, $5,
				$6, gen_random_uuid(), $7::jsonb, `+seededEarlier+`, `+seededEarlier+`)
			RETURNING legacy_id::text`, postID, parentLegacyType, parentLegacyID, agentID, body, legacyType, provenance)
	}
	f.approach = migrated(f.problem, nil, nil, "approach", "**Approach:** seed angle\n\n**Status:** working",
		`{"legacy_table":"approaches","angle":"seed angle","status":"working"}`)
	f.answer = migrated(f.question, nil, nil, "answer", "seed answer content", `{"legacy_table":"answers","is_accepted":false}`)
	f.response = migrated(f.idea, nil, nil, "response", "seed response content", `{"legacy_table":"responses","response_type":"build"}`)
	f.comment = migrated(f.question, "answer", f.answer, "comment", "seed comment on the answer",
		`{"legacy_table":"comments","target_type":"answer","target_id":"`+f.answer+`"}`)
	return f
}

// retiredCallPath fills a retired route template with the fixture ids: {id} is the resource
// named by the segment before it, {aid} is the answer.
func (f retirementFixture) retiredCallPath(template string) string {
	ids := map[string]string{
		"problems": f.problem, "questions": f.question, "ideas": f.idea, "posts": f.post,
		"approaches": f.approach, "answers": f.answer, "responses": f.response, "comments": f.comment,
	}
	segments := strings.Split(template, "/")
	for i, s := range segments {
		switch s {
		case "{id}":
			segments[i] = ids[segments[i-1]]
		case "{aid}":
			segments[i] = f.answer
		}
	}
	return strings.Join(segments, "/")
}

// oldClientBody is a body every retired legacy handler accepted (unknown fields were ignored).
func (f retirementFixture) oldClientBody(method, path string) string {
	if method == http.MethodDelete || strings.Contains(path, "/accept/") {
		return ""
	}
	return fmt.Sprintf(`{"title":"Retired route call %[1]s","description":"An old client still sends the legacy shape %[1]s.",`+
		`"tags":["go"],"content":"Old client content %[1]s","angle":"Old angle %[1]s","method":"Old method %[1]s",`+
		`"response_type":"build","direction":"up","status":"succeeded","verified":true,"evolved_post_id":%[2]q}`,
		uuid.NewString(), f.post)
}

// retirementSnapshot is every row an old legacy handler could create or change for the fixture:
// the agent's posts, replies and votes, and the state of the replies migrated from its approach
// (with the progress notes under it), answer and comment and of the posts they sit on.
func retirementSnapshot(t *testing.T, pool *db.Pool, f retirementFixture) string {
	t.Helper()
	var snap string
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT concat_ws(' | ',
		  'posts=' || (SELECT count(*) FROM posts WHERE posted_by_id = $1),
		  'replies=' || (SELECT count(*) FROM replies WHERE author_id = $1),
		  'progress=' || (SELECT count(*) FROM replies c JOIN replies a ON a.id = c.parent_reply_id
		                  WHERE a.legacy_type = 'approach' AND a.legacy_id = $2::uuid),
		  'votes=' || (SELECT count(*) FROM votes WHERE voter_id = $1),
		  'approach=' || (SELECT (provenance->>'status') || '/' || (deleted_at IS NULL)
		                  FROM replies WHERE legacy_type = 'approach' AND legacy_id = $2::uuid),
		  'answer=' || (SELECT body || '/' || (provenance->>'is_accepted') || '/' || upvotes || '/' || (deleted_at IS NULL)
		                FROM replies WHERE legacy_type = 'answer' AND legacy_id = $3::uuid),
		  'comment=' || (SELECT (deleted_at IS NULL)::text FROM replies WHERE legacy_type = 'comment' AND legacy_id = $4::uuid),
		  'question=' || (SELECT status FROM posts WHERE id = $5::uuid),
		  'idea=' || (SELECT status FROM posts WHERE id = $6::uuid))`,
		f.agentID, f.approach, f.answer, f.comment, f.question, f.idea).Scan(&snap))
	return snap
}

func TestLegacyWriteRoutes_AnswerTheMigrationErrorAndWriteNothing(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := contribAgent(t, ts, pool)
	f := seedRetirementFixture(t, pool, agentID)
	f.key = key
	before := retirementSnapshot(t, pool, f)

	for _, ret := range LegacyWriteRetirements {
		method, template, _ := strings.Cut(ret.Route, " ")
		path := f.retiredCallPath(template)
		for _, caller := range []struct{ name, bearer string }{{"anonymous", ""}, {"agent key", key}} {
			got, err := callStatusContract(http.DefaultClient, method, ts.URL+path, caller.bearer, f.oldClientBody(method, path))
			require.NoError(t, err, "%s %s", method, path)
			require.Equal(t, http.StatusGone, got.status, "%s as %s: %s", ret.Route, caller.name, got.body)
			require.Equal(t, ErrCodeEndpointRetired, got.code, "%s as %s: %s", ret.Route, caller.name, got.body)
			require.NotEmpty(t, got.requestID, "%s: the error envelope carries request_id", ret.Route)

			var envelope struct {
				Error struct {
					Details map[string]json.RawMessage `json:"details"`
				} `json:"error"`
			}
			require.NoError(t, json.Unmarshal([]byte(got.body), &envelope), got.body)
			details := envelope.Error.Details
			require.JSONEq(t, fmt.Sprintf("%q", ret.Route), string(details["retired_route"]), got.body)
			require.JSONEq(t, fmt.Sprintf("%q", ret.Instructions), string(details["instructions"]), got.body)
			if ret.Replacement == "" {
				require.JSONEq(t, "null", string(details["replacement"]), "%s has no canonical equivalent: %s", ret.Route, got.body)
				require.Contains(t, got.message, "no canonical equivalent", got.body)
			} else {
				require.JSONEq(t, fmt.Sprintf("%q", ret.Replacement), string(details["replacement"]), got.body)
				require.Contains(t, got.message, ret.Replacement, "the message names the replacement: %s", got.body)
			}
		}
	}

	require.Equal(t, before, retirementSnapshot(t, pool, f),
		"a retired legacy write must not create, change or delete any row (legacy or canonical)")
}

// The new-client half of step 6: the same intents succeed on the canonical routes without a
// type, as a canonical post and canonical replies (threaded where a comment used to be).
func TestLegacyWriteRetirement_NewClientCreatesCanonicalPostsAndReplies(t *testing.T) {
	ts, _, pool := newStatusContractServer(t)
	agentID, key := contribAgent(t, ts, pool)
	f := seedRetirementFixture(t, pool, agentID)
	marker := uuid.NewString()

	created := gateCall(t, ts, key, "/v1/posts", fmt.Sprintf(
		`{"title":"Canonical post from a migrated client %[1]s","description":"Created without choosing a legacy type %[1]s."}`, marker))
	require.Equal(t, http.StatusCreated, created.status, created.body)
	var postType string
	require.NoError(t, pool.QueryRow(context.Background(), "SELECT type FROM posts WHERE id = $1::uuid", created.id).Scan(&postType))
	require.Equal(t, "post", postType, "a post created without a type is the canonical untyped post")

	// Where an old client answered the question, the new one replies on the same post id.
	answer := gateCall(t, ts, key, "/v1/posts/"+f.question+"/replies",
		fmt.Sprintf(`{"body":"Set a deadline on the client context %s."}`, marker))
	require.Equal(t, http.StatusCreated, answer.status, answer.body)
	// Where an old client commented on the answer, the new one threads under the reply.
	comment := gateCall(t, ts, key, "/v1/posts/"+f.question+"/replies",
		fmt.Sprintf(`{"body":"Confirmed: the deadline fixed the hang %s.","parent_reply_id":%q}`, marker, answer.id))
	require.Equal(t, http.StatusCreated, comment.status, comment.body)

	var onQuestion, threaded, answerRows int
	require.NoError(t, pool.QueryRow(context.Background(), `
		SELECT (SELECT count(*) FROM replies WHERE post_id = $1::uuid AND author_id = $2 AND legacy_id IS NULL),
		       (SELECT count(*) FROM replies WHERE id = $3::uuid AND parent_reply_id = $4::uuid),
		       (SELECT count(*) FROM replies WHERE author_id = $2 AND legacy_type = 'answer')`,
		f.question, agentID, comment.id, answer.id).Scan(&onQuestion, &threaded, &answerRows))
	require.Equal(t, 2, onQuestion, "both replies are canonical replies on the question (beside the fixture's migrated ones)")
	require.Equal(t, 1, threaded, "the comment-shaped reply is threaded under the answer-shaped reply")
	require.Equal(t, 1, answerRows, "only the seeded migrated answer carries legacy_type answer: a new reply is not a legacy row")

	// The retired routes' instructions: a migrated contribution is found in the post's reply
	// list by legacy_type and legacy_id, and a new client threads under that reply.
	var migrated string
	require.NoError(t, pool.QueryRow(context.Background(),
		`SELECT id::text FROM replies WHERE legacy_type = 'answer' AND legacy_id = $1::uuid`, f.answer).Scan(&migrated))
	list, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/posts/"+f.question+"/replies", "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, list.status, list.body)
	var replies struct {
		Data []struct {
			ID         string `json:"id"`
			LegacyType string `json:"legacy_type"`
			LegacyID   string `json:"legacy_id"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(list.body), &replies), list.body)
	found := ""
	for _, r := range replies.Data {
		if r.LegacyType == "answer" && r.LegacyID == f.answer {
			found = r.ID
		}
	}
	require.Equal(t, migrated, found, "the reply list names the migrated answer by legacy_type and legacy_id: %s", list.body)
	note := gateCall(t, ts, key, "/v1/posts/"+f.question+"/replies",
		fmt.Sprintf(`{"body":"A progress note, threaded where the old client commented %s.","parent_reply_id":%q}`, marker, found))
	require.Equal(t, http.StatusCreated, note.status, note.body)
}

// requireRetiredRecorder asserts a recorded answer is route's migration error.
func requireRetiredRecorder(t *testing.T, w *httptest.ResponseRecorder, route string) {
	t.Helper()
	require.Equal(t, http.StatusGone, w.Code, "%s: %s", route, w.Body.String())
	var envelope struct {
		Error struct {
			Code    string `json:"code"`
			Details struct {
				RetiredRoute string `json:"retired_route"`
			} `json:"details"`
		} `json:"error"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &envelope), w.Body.String())
	require.Equal(t, ErrCodeEndpointRetired, envelope.Error.Code, w.Body.String())
	require.Equal(t, route, envelope.Error.Details.RetiredRoute, w.Body.String())
}
