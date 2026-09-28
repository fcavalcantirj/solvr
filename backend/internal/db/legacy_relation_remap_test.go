package db

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A stored notification link that names a legacy page or anchor is rewritten to the
// canonical post and reply; everything else is left exactly as stored (pure, no DB).
func TestCanonicalNotificationLink_RewritesLegacyDestinations(t *testing.T) {
	const (
		post  = "11111111-1111-1111-1111-111111111111"
		ans   = "22222222-2222-2222-2222-222222222222"
		appr  = "33333333-3333-3333-3333-333333333333"
		cmt   = "44444444-4444-4444-4444-444444444444"
		ghost = "99999999-9999-9999-9999-999999999999"
		rAns  = "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa"
		rAppr = "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb"
		rCmt  = "cccccccc-cccc-cccc-cccc-cccccccccccc"
	)
	replies := map[string][2]string{
		"answer/" + ans:    {post, rAns},
		"approach/" + appr: {post, rAppr},
		"comment/" + cmt:   {post, rCmt},
	}
	resolve := func(legacyType, legacyID string) (string, string, bool) {
		r, ok := replies[legacyType+"/"+legacyID]
		return r[0], r[1], ok
	}

	cases := []struct {
		name, in, want string
		resolved       bool
	}{
		{"answer anchor", "/questions/" + post + "#answer-" + ans, "/posts/" + post + "#" + rAns, true},
		{"approach anchor", "/problems/" + post + "#approach-" + appr, "/posts/" + post + "#" + rAppr, true},
		{"comment on a contribution (the old /approachs/ path)", "/approachs/" + appr + "#comment-" + cmt, "/posts/" + post + "#" + rCmt, true},
		{"comment on a post", "/posts/" + post + "#comment-" + cmt, "/posts/" + post + "#" + rCmt, true},
		{"legacy problem page", "/problems/" + post, "/posts/" + post, true},
		{"legacy idea page", "/ideas/" + post, "/posts/" + post, true},
		{"legacy sub-path", "/questions/" + post + "/edit", "/posts/" + post + "/edit", true},
		{"canonical post", "/posts/" + post, "/posts/" + post, true},
		{"unrelated page", "/rooms/some-room", "/rooms/some-room", true},
		{"empty", "", "", true},
		{"unresolved anchor stays as stored", "/questions/" + post + "#answer-" + ghost, "/questions/" + post + "#answer-" + ghost, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, resolved := CanonicalNotificationLink(c.in, resolve)
			assert.Equal(t, c.want, got)
			assert.Equal(t, c.resolved, resolved)
		})
	}
}

func insertRemapAgent(t *testing.T, pool *Pool, ctx context.Context, id string) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, api_key_hash, status)
		VALUES ($1, $2, $3, 'active')`, id, id, "hash_"+id)
	require.NoError(t, err)
}

func insertRemapNotification(t *testing.T, pool *Pool, ctx context.Context, agentID, link string, read bool) string {
	t.Helper()
	var readAt any
	if read {
		readAt = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO notifications (agent_id, type, title, link, read_at)
		VALUES ($1, 'answer_created', 'legacy', $2, $3) RETURNING id::text`, agentID, link, readAt).Scan(&id))
	return id
}

func replyFor(t *testing.T, pool *Pool, ctx context.Context, legacyType, legacyID string) (id, postID string, parent *string, prov []byte) {
	t.Helper()
	require.NoError(t, pool.QueryRow(ctx, `SELECT id::text, post_id::text, parent_reply_id::text, provenance
		FROM replies WHERE legacy_type=$1 AND legacy_id=$2`, legacyType, legacyID).Scan(&id, &postID, &parent, &prov),
		"no reply for %s %s", legacyType, legacyID)
	return id, postID, parent, prov
}

func targetOf(t *testing.T, pool *Pool, ctx context.Context, table, id string) (string, string) {
	t.Helper()
	var tt, tid string
	require.NoError(t, pool.QueryRow(ctx, `SELECT target_type, target_id::text FROM `+table+` WHERE id=$1`, id).Scan(&tt, &tid))
	return tt, tid
}

func countRows(t *testing.T, pool *Pool, ctx context.Context, query string, args ...any) int {
	t.Helper()
	var n int
	require.NoError(t, pool.QueryRow(ctx, query, args...).Scan(&n))
	return n
}

// The cutover routine retargets every relationship that points at a legacy contribution
// onto its canonical reply without losing the target's identity, sends nothing, reports
// what it could not resolve, and changes nothing on a second run (task idx 76 step 2).
func TestRemapLegacyRelations_RetargetsEveryDependentAndSendsNothing(t *testing.T) {
	pool := setupTestDB(t)
	defer pool.Close()
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	agent := "agent_lrr_" + sfx
	voter := "agent_lrv_" + sfx
	insertRemapAgent(t, pool, ctx, agent)
	insertRemapAgent(t, pool, ctx, voter)
	defer func() {
		_, _ = pool.Exec(ctx, `DELETE FROM notifications WHERE agent_id=$1`, agent)
		_, _ = pool.Exec(ctx, `DELETE FROM agents WHERE id IN ($1,$2)`, agent, voter)
	}()

	probID := insertTestPostWithAuthor(t, pool, ctx, "problem", "lrr problem "+sfx, "body", []string{"lrr"}, "open", "agent", agent)
	qID := insertTestPostWithAuthor(t, pool, ctx, "question", "lrr question "+sfx, "body", []string{"lrr"}, "open", "agent", agent)

	a1 := insertApproach(t, pool, ctx, probID, agent, "succeeded", false)
	a2 := insertApproach(t, pool, ctx, probID, agent, "failed", true) // soft-deleted approach
	_, err := pool.Exec(ctx, `UPDATE approaches SET archived_cid='bafylrr'||$2 WHERE id=$1`, a1, sfx)
	require.NoError(t, err)

	var an1 string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO answers (question_id, author_type, author_id, content, is_accepted)
		VALUES ($1,'agent',$2,'accepted answer',TRUE) RETURNING id`, qID, agent).Scan(&an1))
	_, err = pool.Exec(ctx, `UPDATE posts SET accepted_answer_id=$1 WHERE id=$2`, an1, qID)
	require.NoError(t, err)
	c1 := insertComment(t, pool, ctx, "approach", a1, agent, "comment on approach")

	var rel string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO approach_relationships (from_approach_id, to_approach_id, relation_type)
		VALUES ($1,$2,'extends') RETURNING id`, a1, a2).Scan(&rel))
	noteTime := time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC)
	var n1, n2 string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO progress_notes (approach_id, content, created_at)
		VALUES ($1,'halfway there',$2) RETURNING id`, a1, noteTime).Scan(&n1))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO progress_notes (approach_id, content)
		VALUES ($1,'note on a deleted approach') RETURNING id`, a2).Scan(&n2))

	var vAppr, vAns, rep, flagAns, flagPost string
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction)
		VALUES ('approach',$1,'agent',$2,'up') RETURNING id`, a1, voter).Scan(&vAppr))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction)
		VALUES ('answer',$1,'agent',$2,'down') RETURNING id`, an1, voter).Scan(&vAns))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO reports (target_type, target_id, reporter_type, reporter_id, reason)
		VALUES ('comment',$1,'agent',$2,'spam') RETURNING id`, c1, voter).Scan(&rep))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason)
		VALUES ('answer',$1,'agent',$2,'incorrect') RETURNING id`, an1, voter).Scan(&flagAns))
	require.NoError(t, pool.QueryRow(ctx, `INSERT INTO flags (target_type, target_id, reporter_type, reporter_id, reason)
		VALUES ('post',$1,'agent',$2,'spam') RETURNING id`, probID, voter).Scan(&flagPost))

	ghost := "99999999-9999-9999-9999-99999999" + time.Now().Format("0405")
	nAns := insertRemapNotification(t, pool, ctx, agent, "/questions/"+qID+"#answer-"+an1, true)
	nAppr := insertRemapNotification(t, pool, ctx, agent, "/problems/"+probID+"#approach-"+a1, false)
	nCmt := insertRemapNotification(t, pool, ctx, agent, "/approachs/"+a1+"#comment-"+c1, false)
	nPage := insertRemapNotification(t, pool, ctx, agent, "/problems/"+probID, false)
	nCanon := insertRemapNotification(t, pool, ctx, agent, "/posts/"+probID, false)
	nGhost := insertRemapNotification(t, pool, ctx, agent, "/questions/"+qID+"#answer-"+ghost, false)

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)

	notifTotal := countRows(t, pool, ctx, `SELECT COUNT(*) FROM notifications`)
	readBefore := countRows(t, pool, ctx, `SELECT COUNT(*) FROM notifications WHERE agent_id=$1 AND read_at IS NOT NULL`, agent)

	report, err := RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)

	// Nothing new is sent: no notification row appears and read state is untouched.
	assert.Equal(t, notifTotal, countRows(t, pool, ctx, `SELECT COUNT(*) FROM notifications`))
	assert.Equal(t, readBefore, countRows(t, pool, ctx, `SELECT COUNT(*) FROM notifications WHERE agent_id=$1 AND read_at IS NOT NULL`, agent))

	rA1, _, _, provA1 := replyFor(t, pool, ctx, "approach", a1)
	rA2, _, _, _ := replyFor(t, pool, ctx, "approach", a2)
	rAn1, _, _, provAn1 := replyFor(t, pool, ctx, "answer", an1)
	rC1, _, _, _ := replyFor(t, pool, ctx, "comment", c1)

	// Votes, reports and flags keep their target, now named by the reply.
	tt, tid := targetOf(t, pool, ctx, "votes", vAppr)
	assert.Equal(t, [2]string{"reply", rA1}, [2]string{tt, tid})
	tt, tid = targetOf(t, pool, ctx, "votes", vAns)
	assert.Equal(t, [2]string{"reply", rAn1}, [2]string{tt, tid})
	tt, tid = targetOf(t, pool, ctx, "reports", rep)
	assert.Equal(t, [2]string{"reply", rC1}, [2]string{tt, tid})
	tt, tid = targetOf(t, pool, ctx, "flags", flagAns)
	assert.Equal(t, [2]string{"reply", rAn1}, [2]string{tt, tid})
	tt, tid = targetOf(t, pool, ctx, "flags", flagPost)
	assert.Equal(t, [2]string{"post", probID}, [2]string{tt, tid}, "a post flag already names a canonical post")

	// Accepted-answer provenance: the post points at the reply, the reply keeps is_accepted.
	var accepted string
	require.NoError(t, pool.QueryRow(ctx, `SELECT accepted_answer_id::text FROM posts WHERE id=$1`, qID).Scan(&accepted))
	assert.Equal(t, rAn1, accepted)
	assert.Equal(t, "true", provField(t, provAn1, "is_accepted"))

	// Verification records and archived CIDs travel in the approach reply's provenance.
	assert.Equal(t, "succeeded", provField(t, provA1, "status"))
	assert.Equal(t, "Outcome text", provField(t, provA1, "outcome"))
	assert.Equal(t, "Solution text", provField(t, provA1, "solution"))
	assert.Equal(t, "bafylrr"+sfx, provField(t, provA1, "archived_cid"))

	// Approach relationships are kept, both ends named by reply and by legacy id.
	var prov struct {
		Rels []map[string]string `json:"approach_relationships"`
	}
	require.NoError(t, json.Unmarshal(provA1, &prov))
	require.Len(t, prov.Rels, 1)
	assert.Equal(t, rel, prov.Rels[0]["legacy_id"])
	assert.Equal(t, "extends", prov.Rels[0]["relation_type"])
	assert.Equal(t, rA2, prov.Rels[0]["to_reply_id"])
	assert.Equal(t, a2, prov.Rels[0]["to_approach_id"])
	assert.NotEmpty(t, prov.Rels[0]["created_at"])

	// Progress notes become child replies of the approach's reply, by its author, at their time.
	rN1, postN1, parentN1, provN1 := replyFor(t, pool, ctx, "progress_note", n1)
	require.NotNil(t, parentN1)
	assert.Equal(t, rA1, *parentN1)
	assert.Equal(t, probID, postN1)
	assert.Equal(t, a1, provField(t, provN1, "approach_id"))
	var body, authorType, authorID string
	var createdAt time.Time
	var deletedN1 *time.Time
	require.NoError(t, pool.QueryRow(ctx, `SELECT body, author_type, author_id, created_at, deleted_at FROM replies WHERE id=$1`, rN1).
		Scan(&body, &authorType, &authorID, &createdAt, &deletedN1))
	assert.Equal(t, "halfway there", body)
	assert.Equal(t, [2]string{"agent", agent}, [2]string{authorType, authorID})
	assert.True(t, createdAt.Equal(noteTime), "created_at %v, want %v", createdAt, noteTime)
	assert.Nil(t, deletedN1)
	rN2, _, parentN2, _ := replyFor(t, pool, ctx, "progress_note", n2)
	require.NotNil(t, parentN2)
	assert.Equal(t, rA2, *parentN2)
	assert.Equal(t, 1, countRows(t, pool, ctx, `SELECT COUNT(*) FROM replies WHERE id=$1 AND deleted_at IS NOT NULL`, rN2),
		"a note on a deleted approach must not be revived")

	// Notification links move to canonical destinations; unresolvable ones are reported, not guessed.
	links := map[string]string{
		nAns:   "/posts/" + qID + "#" + rAn1,
		nAppr:  "/posts/" + probID + "#" + rA1,
		nCmt:   "/posts/" + probID + "#" + rC1,
		nPage:  "/posts/" + probID,
		nCanon: "/posts/" + probID,
		nGhost: "/questions/" + qID + "#answer-" + ghost,
	}
	for id, want := range links {
		var got string
		require.NoError(t, pool.QueryRow(ctx, `SELECT link FROM notifications WHERE id=$1`, id).Scan(&got))
		assert.Equal(t, want, got, "notification %s", id)
	}
	ex := report.ExceptionsFor(nGhost)
	require.Len(t, ex, 1)
	assert.Equal(t, "notification", ex[0].LegacyType)
	assert.Equal(t, LegacyRemapExceptionUnresolved, ex[0].Kind)
	assert.Empty(t, report.ExceptionsFor(vAppr, vAns, rep, flagAns, rel, n1, n2, nAns, nAppr, nCmt, nPage))

	assert.GreaterOrEqual(t, report.AcceptedAnswers, int64(1))
	assert.GreaterOrEqual(t, report.Votes, int64(2))
	assert.GreaterOrEqual(t, report.Reports, int64(1))
	assert.GreaterOrEqual(t, report.Flags, int64(1))
	assert.GreaterOrEqual(t, report.ApproachRelationships, int64(1))
	assert.GreaterOrEqual(t, report.ProgressNotes, int64(2))
	assert.GreaterOrEqual(t, report.NotificationLinks, int64(4))

	// A second run changes nothing.
	again, err := RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.Zero(t, again.AcceptedAnswers+again.Votes+again.Reports+again.Flags+
		again.ApproachRelationships+again.ProgressNotes+again.NotificationLinks, "second run: %+v", again)
	_, _, _, provA1Again := replyFor(t, pool, ctx, "approach", a1)
	require.NoError(t, json.Unmarshal(provA1Again, &prov))
	assert.Len(t, prov.Rels, 1, "the relationship is recorded once")
	assert.Equal(t, notifTotal, countRows(t, pool, ctx, `SELECT COUNT(*) FROM notifications`))
}

// The registry marks done exactly the remaps TestRemapLegacyRelations_RetargetsEvery
// DependentAndSendsNothing verifies; everything else that remaps stays open.
func TestLegacyDependencyRegistry_RemapsDoneAreTheVerifiedOnes(t *testing.T) {
	verified := map[string]bool{
		"relation:votes": true, "relation:reports": true, "relation:notifications": true,
		"relation:accepted-answer-provenance": true, "relation:approach-relationships": true,
		"relation:progress-notes": true, "relation:verification-records": true,
		"relation:archived-cids": true, "column:posts.accepted_answer_id": true,
	}
	for key, d := range LegacyDependencyDispositions {
		if d.Action == LegacyActionRemap {
			assert.Equal(t, verified[key], d.Done, "%s done=%v", key, d.Done)
		}
	}
	for key := range verified {
		assert.Contains(t, LegacyDependencyDispositions, key)
	}
}
