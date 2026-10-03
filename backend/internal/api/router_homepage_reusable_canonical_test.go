package api

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3: GET /v1/homepage/overview's reusable posts count canonical replies. A
// live human or agent reply makes a post reusable and counts, a child reply too; a system
// verdict, a deleted reply and a legacy approach that was never migrated do not.
func TestHomepageOverviewReusablePosts_CountCanonicalReplies(t *testing.T) {
	t.Setenv("HOMEPAGE_PREVIEW_ROOM_SLUGS", hpoSlug("none"))

	ts, pool, cleanup := setupRoomTestServer(t)
	defer cleanup()
	defer hpoCleanup(t, pool)
	ctx := context.Background()
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err, sql)
	}

	approachOnly := hpoInsertPostWithReply(t, pool, "hpo canonical post with an unmigrated approach", "public")
	exec(`DELETE FROM replies WHERE post_id = $1`, approachOnly)
	exec(`INSERT INTO approaches (problem_id, author_type, author_id, angle, status)
		VALUES ($1, 'agent', 'agent_hpotest', 'unmigrated approach', 'working')`, approachOnly)
	verdictOnly := hpoInsertPostWithReply(t, pool, "hpo canonical post with a verdict only", "public")
	exec(`DELETE FROM replies WHERE post_id = $1`, verdictOnly)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', 'approved')`, verdictOnly)

	reusable := hpoInsertPostWithReply(t, pool, "hpo canonical post with replies", "public")
	var first string
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text FROM replies WHERE post_id = $1`, reusable).Scan(&first))
	human, _ := createLiveTestUser(t, pool, "user")
	exec(`INSERT INTO replies (post_id, parent_reply_id, author_type, author_id, body) VALUES ($1, $2, 'human', $3, 'a child reply')`, reusable, first, human)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body) VALUES ($1, 'system', 'moderation', 'approved')`, reusable)
	exec(`INSERT INTO replies (post_id, author_type, author_id, body, deleted_at) VALUES ($1, 'agent', 'agent_hpotest', 'gone', NOW())`, reusable)

	ov, raw := getHomepageOverview(t, ts.URL)

	assert.Equal(t, "Public posts that already carry at least one reply from a person or an agent, "+
		"most recently worked on first.", ov.Posts.Definition)
	assert.NotContains(t, raw, "hpo canonical post with an unmigrated approach", "a legacy approach is not a reply")
	assert.NotContains(t, raw, "hpo canonical post with a verdict only", "a moderation verdict is not a contribution")
	require.NotEmpty(t, ov.Posts.Items)
	item := ov.Posts.Items[0]
	assert.Equal(t, reusable, item.ID, "the most recently worked-on post leads")
	assert.Equal(t, 2, item.ContributionCount, "the reply and its child; not the verdict or the deleted reply")
	assert.Equal(t, "2 contributions", item.ContributionLabel)
	assert.Equal(t, "/posts/"+reusable, item.URL, "every post links to /posts/{id}")
}
