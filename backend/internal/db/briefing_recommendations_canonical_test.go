package db

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:briefing, recommendations, inferred specialties and the
// /me/diff opportunity count): canonical posts, replies and votes only.

func TestCanonicalRecommendations_YouMightLike(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalRecommendationRepository(pool)
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	seq := cpSeq()
	liked, special, adjacent, privateAdjacent := "cbpl-t-"+seq, "cbpl-s-"+seq, "cbpl-a1-"+seq, "cbpl-a2-"+seq
	byOther := func(title string, tags []string, s cbPostSeed) string {
		s.byID, s.tags = other, tags
		return cbInsertPost(t, pool, ctx, title, s)
	}

	upvoted := byOther("rec the post I upvoted", []string{liked}, cbPostSeed{})
	cbInsertVote(t, pool, ctx, "post", upvoted, me, "up", true, time.Hour)
	best := byOther("rec best in my voted tag", []string{liked}, cbPostSeed{})
	cpSetPost(t, pool, ctx, best, `upvotes = 3`)
	next := byOther("rec next in my voted tag", []string{liked}, cbPostSeed{})
	cpSetPost(t, pool, ctx, next, `upvotes = 1`)
	replied := byOther("rec I replied here", []string{liked}, cbPostSeed{})
	cbInsertReply(t, pool, ctx, replied, "agent", me, time.Hour, false)
	votedReply := byOther("rec I voted on a reply here", []string{liked}, cbPostSeed{})
	reply := cbInsertReply(t, pool, ctx, votedReply, "human", authorHuman(ctx, t, pool, "cbpl-someone"), time.Hour, false)
	cbInsertVote(t, pool, ctx, "reply", reply, me, "up", true, time.Hour)
	mine := cbInsertPost(t, pool, ctx, "rec my own post", cbPostSeed{byID: me, tags: []string{liked}})
	excluded := map[string]string{"upvoted": upvoted, "replied": replied, "voted a reply": votedReply, "mine": mine}
	for name, s := range map[string]cbPostSeed{
		"family": {visibility: "family"}, "closed": {status: "closed"}, "draft": {publication: "draft"},
		"pending": {moderation: "pending"}, "deleted": {deleted: true},
	} {
		excluded[name] = byOther("rec "+name, []string{liked}, s)
	}

	source := byOther("rec specialty source", []string{special, adjacent}, cbPostSeed{})
	near := byOther("rec adjacent to my specialty", []string{adjacent}, cbPostSeed{})
	byOther("rec private co-occurrence", []string{special, privateAdjacent}, cbPostSeed{visibility: "family"})
	excluded["tag co-occurring only in a private post"] = byOther("rec behind a private tag", []string{privateAdjacent}, cbPostSeed{})

	got, err := repo.GetYouMightLike(ctx, me, []string{special}, 10)
	require.NoError(t, err)
	ids := make([]string, len(got))
	for i, r := range got {
		ids[i] = r.ID
	}
	require.Len(t, got, 4, "%v", ids)
	require.Equal(t, []string{best, next}, ids[:2], "voted-tag matches first, by score")
	require.ElementsMatch(t, []string{source, near}, ids[2:], "then posts in tags adjacent to the specialties")
	for _, r := range got[:2] {
		require.Equal(t, "voted_tags", r.MatchReason)
	}
	for _, r := range got[2:] {
		require.Equal(t, "adjacent_tags", r.MatchReason)
	}
	require.Equal(t, "post", got[0].Type)
	require.Equal(t, 3, got[0].VoteScore)
	for name, id := range excluded {
		require.NotContains(t, ids, id, "%s must not be recommended", name)
	}

	top, err := repo.GetYouMightLike(ctx, me, []string{special}, 1)
	require.NoError(t, err)
	require.Len(t, top, 1)
	require.Equal(t, best, top[0].ID)

	fresh := cbInsertAgent(t, pool, ctx, nil, nil)
	none, err := repo.GetYouMightLike(ctx, fresh, nil, 10)
	require.NoError(t, err)
	require.NotNil(t, none)
	require.Empty(t, none)
}

func TestCanonicalInferredSpecialties_ForAgent(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewCanonicalInferredSpecialtiesRepository(pool)
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	seq := cpSeq()
	// Names sort a < b < c: a reply must outweigh an upvote (2 > 1) and tie with a post.
	upvoteTag, replyTag, postTag := "cbpl-a-"+seq, "cbpl-b-"+seq, "cbpl-c-"+seq
	deletedReplyTag, deletedPostTag, notMineTag := "cbpl-d-"+seq, "cbpl-e-"+seq, "cbpl-f-"+seq

	cbInsertPost(t, pool, ctx, "infer my post", cbPostSeed{byID: me, tags: []string{postTag}})
	onOther := func(tags []string, s cbPostSeed) string {
		s.byID, s.tags = other, tags
		return cbInsertPost(t, pool, ctx, "infer other post", s)
	}
	cbInsertReply(t, pool, ctx, onOther([]string{replyTag}, cbPostSeed{}), "agent", me, time.Hour, false)
	cbInsertVote(t, pool, ctx, "post", onOther([]string{upvoteTag}, cbPostSeed{}), me, "up", true, time.Hour)
	cbInsertReply(t, pool, ctx, onOther([]string{deletedReplyTag}, cbPostSeed{}), "agent", me, time.Hour, true)
	cbInsertReply(t, pool, ctx, onOther([]string{deletedPostTag}, cbPostSeed{deleted: true}), "agent", me, time.Hour, false)
	cbInsertReply(t, pool, ctx, onOther([]string{notMineTag}, cbPostSeed{}), "agent", authorAgent(ctx, t, pool, other+"x"), time.Hour, false)

	got, err := repo.InferSpecialtiesForAgent(ctx, me)
	require.NoError(t, err)
	require.Equal(t, []string{replyTag, postTag, upvoteTag}, got)

	many := cbInsertAgent(t, pool, ctx, nil, nil)
	for i := 0; i < 7; i++ {
		cbInsertPost(t, pool, ctx, "infer many", cbPostSeed{byID: many, tags: []string{"cbpl-m-" + cpSeq()}})
	}
	five, err := repo.InferSpecialtiesForAgent(ctx, many)
	require.NoError(t, err)
	require.Len(t, five, 5)

	none, err := repo.InferSpecialtiesForAgent(ctx, cbInsertAgent(t, pool, ctx, nil, nil))
	require.NoError(t, err)
	require.NotNil(t, none)
	require.Empty(t, none)
}

func TestBriefingDiff_CountNewOpportunitiesSince_CanonicalPosts(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	repo := NewBriefingDiffRepository(pool)
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	tag := "cbpl-diff-" + cpSeq()
	since := time.Now().UTC().Add(-time.Hour)
	byOther := func(s cbPostSeed) {
		if s.byID == "" {
			s.byID = other
		}
		if s.tags == nil {
			s.tags = []string{tag}
		}
		cbInsertPost(t, pool, ctx, "diff opportunity", s)
	}

	byOther(cbPostSeed{})
	byOther(cbPostSeed{postType: "problem", status: "in_progress"})
	byOther(cbPostSeed{postType: "idea", status: "active"})
	byOther(cbPostSeed{postType: "question"})
	for _, s := range []cbPostSeed{
		{age: 2 * time.Hour}, {byID: me}, {visibility: "family"}, {publication: "draft"},
		{moderation: "pending"}, {deleted: true}, {postType: "problem", status: "solved"},
		{status: "closed"}, {tags: []string{"cbpl-diff-elsewhere"}},
	} {
		byOther(s)
	}

	n, err := repo.CountNewOpportunitiesSince(ctx, me, []string{tag}, since)
	require.NoError(t, err)
	require.Equal(t, 4, n, "the briefing's opportunity rule, created after since, every post type")

	n, err = repo.CountNewOpportunitiesSince(ctx, me, nil, since)
	require.NoError(t, err)
	require.Zero(t, n)
}
