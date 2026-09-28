package db

import (
	"context"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/reputation"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type frozenEvent struct {
	Owner  string
	PostID *string
	Points int
}

// frozenEvents returns the reputation_history rows recorded for the given legacy row ids,
// keyed by "<source>/<source_id>".
func frozenEvents(t *testing.T, pool *Pool, ctx context.Context, ids ...string) map[string]frozenEvent {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT source, source_id::text, owner_type || ':' || owner_id, post_id::text, points
		FROM reputation_history WHERE source_id::text = ANY($1)`, ids)
	require.NoError(t, err)
	defer rows.Close()
	out := map[string]frozenEvent{}
	for rows.Next() {
		var source, id string
		var ev frozenEvent
		require.NoError(t, rows.Scan(&source, &id, &ev.Owner, &ev.PostID, &ev.Points))
		out[source+"/"+id] = ev
	}
	require.NoError(t, rows.Err())
	return out
}

// Task idx 76 steps 3-4: the cutover freezes every event the legacy reputation rules scored
// and the canonical model cannot score again (post types and statuses, contribution tables,
// votes on contributions that get retargeted to replies) into reputation_history, with the
// legacy points, owner and post. What the legacy rules skipped (questions, deleted rows,
// system comments, unconfirmed votes) is not recorded; votes on approaches are recorded
// with 0 points so their retargeted rows are never scored; votes on posts stay live and are
// not frozen. A second run records nothing new.
func TestFreezeLegacyReputation_RecordsEachEarnedEventOnce(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close) // registered first, so it runs after the fixture cleanups
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	agent := "agent_frz_" + sfx
	voter := "agent_frzv_" + sfx
	human := "human_frz_" + sfx
	insertRemapAgent(t, pool, ctx, agent)
	insertRemapAgent(t, pool, ctx, voter)
	tags := []string{"frz" + sfx}

	solved := insertTestPostWithAuthor(t, pool, ctx, "problem", "frz solved "+sfx, "body", tags, "solved", "agent", agent)
	open := insertTestPostWithAuthor(t, pool, ctx, "problem", "frz open "+sfx, "body", tags, "open", "agent", agent)
	idea := insertTestPostWithAuthor(t, pool, ctx, "idea", "frz idea "+sfx, "body", tags, "open", "agent", agent)
	question := insertTestPostWithAuthor(t, pool, ctx, "question", "frz question "+sfx, "body", tags, "open", "human", human)
	canonical := insertTestPostWithAuthor(t, pool, ctx, "post", "frz post "+sfx, "body", tags, "open", "agent", agent)
	deletedProblem := insertTestPostWithAuthor(t, pool, ctx, "problem", "frz deleted "+sfx, "body", tags, "solved", "agent", agent)
	_, err := pool.Exec(ctx, `UPDATE posts SET deleted_at = NOW() WHERE id = $1`, deletedProblem)
	require.NoError(t, err)

	answer := func(author, authorType string, accepted, deleted bool) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO answers (question_id, author_type, author_id, content, is_accepted, deleted_at)
			VALUES ($1, $2, $3, 'frz answer', $4, CASE WHEN $5 THEN NOW() END) RETURNING id::text`,
			question, authorType, author, accepted, deleted).Scan(&id))
		return id
	}
	accepted := answer(agent, "agent", true, false)
	plain := answer(human, "human", false, false)
	deletedAnswer := answer(agent, "agent", false, true)
	var response string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'frz response', 'support') RETURNING id::text`, idea, agent).Scan(&response))
	approach := insertApproach(t, pool, ctx, solved, agent, "succeeded", false)
	comment := insertComment(t, pool, ctx, "post", solved, agent, "frz comment")
	deletedComment := insertComment(t, pool, ctx, "post", solved, agent, "frz deleted comment")
	_, err = pool.Exec(ctx, `UPDATE comments SET deleted_at = NOW() WHERE id = $1`, deletedComment)
	require.NoError(t, err)
	var systemComment string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ('post', $1, 'system', 'solvr-moderation', 'frz verdict') RETURNING id::text`, solved).Scan(&systemComment))

	vote := func(targetType, targetID, direction string, confirmed bool) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed)
			VALUES ($1, $2, 'agent', $3, $4, $5) RETURNING id::text`,
			targetType, targetID, voter+"_"+direction+targetType, direction, confirmed).Scan(&id))
		return id
	}
	answerUp := vote("answer", accepted, "up", true)
	answerDown := vote("answer", plain, "down", true)
	answerUnconfirmed := vote("answer", accepted, "down", false)
	responseUp := vote("response", response, "up", true)
	responseDown := vote("response", response, "down", true)
	approachUp := vote("approach", approach, "up", true)
	postUp := vote("post", solved, "up", true)

	t.Cleanup(func() {
		c := context.Background()
		ids := []string{solved, open, idea, question, canonical, deletedProblem, accepted, plain, deletedAnswer,
			response, comment, deletedComment, systemComment, answerUp, answerDown, answerUnconfirmed,
			responseUp, responseDown, approachUp, postUp}
		pool.Exec(c, `DELETE FROM reputation_history WHERE source_id::text = ANY($1)`, ids) //nolint:errcheck
		pool.Exec(c, `DELETE FROM votes WHERE id::text = ANY($1)`, ids)                     //nolint:errcheck
		pool.Exec(c, `DELETE FROM comments WHERE target_id = $1`, solved)                   //nolint:errcheck
		pool.Exec(c, `DELETE FROM responses WHERE id = $1`, response)                       //nolint:errcheck
		pool.Exec(c, `DELETE FROM approaches WHERE id = $1`, approach)                      //nolint:errcheck
		pool.Exec(c, `DELETE FROM answers WHERE question_id = $1`, question)                //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id IN ($1, $2)`, agent, human)      //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id IN ($1, $2)`, agent, voter)               //nolint:errcheck
	})

	recorded, err := FreezeLegacyReputation(ctx, pool)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, recorded, int64(12), "at least this fixture's events are recorded")

	ag, hu := "agent:"+agent, "human:"+human
	want := map[string]frozenEvent{
		"problem_solved/" + solved:          {ag, &solved, reputation.PointsProblemSolved},
		"problem_contributed/" + solved:     {ag, &solved, reputation.PointsProblemContributed},
		"problem_contributed/" + open:       {ag, &open, reputation.PointsProblemContributed},
		"idea_posted/" + idea:               {ag, &idea, reputation.PointsIdeaPosted},
		"answer_given/" + accepted:          {ag, &question, reputation.PointsAnswerGiven},
		"answer_accepted/" + accepted:       {ag, &question, reputation.PointsAnswerAccepted},
		"answer_given/" + plain:             {hu, &question, reputation.PointsAnswerGiven},
		"response_given/" + response:        {ag, &idea, reputation.PointsResponseGiven},
		"comment_given/" + comment:          {ag, nil, reputation.PointsCommentGiven},
		"answer_upvote/" + answerUp:         {ag, &question, reputation.PointsUpvoteReceived},
		"answer_downvote/" + answerDown:     {hu, &question, reputation.PointsDownvoteReceived},
		"response_upvote/" + responseUp:     {ag, &idea, reputation.PointsUpvoteReceived},
		"response_downvote/" + responseDown: {ag, &idea, reputation.PointsDownvoteReceived},
		"approach_vote/" + approachUp:       {ag, &solved, 0},
	}
	all := []string{solved, open, idea, question, canonical, deletedProblem, accepted, plain, deletedAnswer,
		response, approach, comment, deletedComment, systemComment,
		answerUp, answerDown, answerUnconfirmed, responseUp, responseDown, approachUp, postUp}
	assert.Equal(t, want, frozenEvents(t, pool, ctx, all...),
		"exactly the scored legacy events, nothing for questions, canonical posts, deleted rows, system comments, unconfirmed votes or post votes")

	var earnedAt, createdAt time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT h.earned_at, v.created_at FROM reputation_history h
		JOIN votes v ON v.id = h.source_id WHERE h.source = 'answer_upvote' AND h.source_id = $1`, answerUp).
		Scan(&earnedAt, &createdAt))
	assert.True(t, earnedAt.Equal(createdAt), "earned when the legacy row was created")

	again, err := FreezeLegacyReputation(ctx, pool)
	require.NoError(t, err)
	assert.Equal(t, want, frozenEvents(t, pool, ctx, all...), "a second run records nothing new")
	assert.Zero(t, again, "a second run over the same legacy rows records nothing")
}
