package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

// Anti-abuse W2 through the real router: replies are moderated after they are created. A
// reject hides the reply (soft delete), flags it moderation_rejected and notifies the author;
// an approve leaves the row alone. The legacy contribution create routes (approaches, answers,
// responses, comments, progress notes) are retired (task idx 52): they create and moderate
// nothing.

type contributionCase struct {
	kind, table, path, body string
	hideable                bool
}

// contributionCases builds one create per contribution route that still creates, on seeded
// open targets: the canonical reply route.
func contributionCases(t *testing.T, pool *db.Pool) []contributionCase {
	t.Helper()
	m := uuid.NewString()[:8]
	post := seedOpenPost(t, pool, "post")
	return []contributionCase{
		{"reply", "replies", "/v1/posts/" + post + "/replies", `{"body":"reply ` + m + `"}`, true},
	}
}

// retiredContributionPaths are the retired legacy contribution creates, on seeded open targets.
func retiredContributionPaths(t *testing.T, pool *db.Pool) map[string]string {
	t.Helper()
	post, question, problem, idea := seedOpenPost(t, pool, "post"), seedOpenPost(t, pool, "question"), seedOpenPost(t, pool, "problem"), seedOpenPost(t, pool, "idea")
	answer := seedLegacy(t, pool, `INSERT INTO answers (question_id, author_type, author_id, content) VALUES ($1, 'agent', 'agent_gate_seed', 'seed answer') RETURNING id::text`, question)
	approach := seedLegacy(t, pool, `INSERT INTO approaches (problem_id, author_type, author_id, angle) VALUES ($1, 'agent', 'agent_gate_seed', 'seed angle') RETURNING id::text`, problem)
	response := seedLegacy(t, pool, `INSERT INTO responses (idea_id, author_type, author_id, content, response_type) VALUES ($1, 'agent', 'agent_gate_seed', 'seed response', 'build') RETURNING id::text`, idea)
	return map[string]string{
		"/v1/problems/" + problem + "/approaches":  `{"angle":"approach angle","method":"approach method"}`,
		"/v1/questions/" + question + "/answers":   `{"content":"answer content"}`,
		"/v1/answers/" + answer + "/comments":      `{"content":"answer comment"}`,
		"/v1/approaches/" + approach + "/comments": `{"content":"approach comment"}`,
		"/v1/approaches/" + approach + "/progress": `{"content":"progress note"}`,
		"/v1/responses/" + response + "/comments":  `{"content":"response comment"}`,
		"/v1/posts/" + post + "/comments":          `{"content":"post comment"}`,
		"/v1/ideas/" + idea + "/responses":         `{"content":"response content","response_type":"build"}`,
	}
}

func moderationAgent(t *testing.T, ts *httptest.Server, pool *db.Pool) (string, string) {
	t.Helper()
	agentID, key := contribAgent(t, ts, pool)
	t.Cleanup(func() {
		ctx := context.Background()
		pool.Exec(ctx, "DELETE FROM notifications WHERE agent_id = $1", agentID)                               //nolint:errcheck
		pool.Exec(ctx, "DELETE FROM flags WHERE reporter_id = 'content-moderation' AND details = $1", agentID) //nolint:errcheck
	})
	return agentID, key
}

// T-M3 … T-M6 and T-M8 (responses): a reject on every contribution route. Each case uses its
// own agent, so the hourly contribution limit never interferes.
func TestModeration_RejectedContributions(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)

	for _, c := range contributionCases(t, pool) {
		agentID, key := moderationAgent(t, ts, pool)
		mod.QueueResults(rejected(agentID)) // the explanation doubles as a cleanup marker
		got := gateCall(t, ts, key, c.path, c.body)
		require.Equal(t, http.StatusCreated, got.status, "%s: %s", c.path, got.body)

		waitForValue(t, pool, "1", `SELECT count(*)::text FROM flags
			WHERE target_type = $1 AND target_id = $2::uuid AND reporter_type = 'system' AND reason = 'moderation_rejected'`, c.kind, got.id)
		deleted := "true"
		if !c.hideable {
			deleted = "false"
		}
		waitForValue(t, pool, deleted, `SELECT (to_jsonb(x) ->> 'deleted_at' IS NOT NULL)::text FROM `+c.table+` x WHERE id = $1::uuid`, got.id)
		notifType := "contribution_removed"
		if !c.hideable {
			notifType = "contribution_flagged"
		}
		waitForValue(t, pool, notifType, `SELECT string_agg(type, ',') FROM notifications WHERE agent_id = $1`, agentID)
	}

	// A hidden reply is gone from its post's reply list.
	agentID, key := moderationAgent(t, ts, pool)
	post := seedOpenPost(t, pool, "post")
	mod.QueueResults(rejected(agentID))
	reply := gateCall(t, ts, key, "/v1/posts/"+post+"/replies", `{"body":"hidden reply `+uuid.NewString()[:8]+`"}`)
	require.Equal(t, http.StatusCreated, reply.status, reply.body)
	waitForValue(t, pool, "true", `SELECT (deleted_at IS NOT NULL)::text FROM replies WHERE id = $1::uuid`, reply.id)
	list, err := callStatusContract(http.DefaultClient, http.MethodGet, ts.URL+"/v1/posts/"+post+"/replies", "", "")
	require.NoError(t, err)
	require.Equal(t, http.StatusOK, list.status, list.body)
	require.False(t, strings.Contains(list.body, reply.id), "a hidden reply is not listed: %s", list.body)
}

// T-M7: an approve leaves every contribution live and unflagged.
func TestModeration_ApprovedContributions(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)

	for _, c := range contributionCases(t, pool) {
		_, key := moderationAgent(t, ts, pool)
		calls := mod.GetCalls()
		got := gateCall(t, ts, key, c.path, c.body)
		require.Equal(t, http.StatusCreated, got.status, "%s: %s", c.path, got.body)
		require.Eventually(t, func() bool { return mod.GetCalls() == calls+1 }, waitTimeout, waitTick, "%s: moderated once", c.path)
		require.Equal(t, "false", queryText(t, pool, `SELECT (to_jsonb(x) ->> 'deleted_at' IS NOT NULL)::text FROM `+c.table+` x WHERE id = $1::uuid`, got.id), c.path)
		require.Equal(t, "0", queryText(t, pool, `SELECT count(*)::text FROM flags WHERE target_id = $1::uuid`, got.id), c.path)
	}
}

// T-M3 … T-M8 on the retired legacy contribution routes (approaches, answers, comments,
// responses, progress notes): an old client's create is refused with the migration error before
// anything is stored, so there is nothing to moderate, flag or notify.
func TestModeration_RetiredContributionRoutesModerateNothing(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, _, pool := newStatusContractServer(t)
	agentID, key := moderationAgent(t, ts, pool)
	for path, body := range retiredContributionPaths(t, pool) {
		calls := mod.GetCalls()
		got := gateCall(t, ts, key, path, body)
		require.Equal(t, http.StatusGone, got.status, "%s: %s", path, got.body)
		require.Equal(t, ErrCodeEndpointRetired, got.code, got.body)
		require.Equal(t, calls, mod.GetCalls(), "%s: a retired route moderates nothing", path)
	}
	require.Equal(t, "0", queryText(t, pool, `SELECT count(*)::text FROM notifications WHERE agent_id = $1`, agentID))
	require.Equal(t, "0", queryText(t, pool, `SELECT count(*)::text FROM flags WHERE details = $1`, agentID))
}

func queryText(t *testing.T, pool *db.Pool, query string, args ...any) string {
	t.Helper()
	var v string
	require.NoError(t, pool.QueryRow(context.Background(), query, args...).Scan(&v), query)
	return v
}
