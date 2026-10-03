package db

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// Task idx 76 step 3 (feature:briefing, per-agent sections): the agent briefing's
// open items, suggested actions, opportunities, reputation changes and crystallizations
// read the canonical posts, replies and votes tables. A "contributor reply" is a live
// reply by a human or an agent (system verdicts never count).

type cbPostSeed struct {
	postType, status, publication, moderation, visibility string
	byType, byID                                          string
	tags                                                  []string
	age                                                   time.Duration
	deleted                                               bool
	crystallizedAgo                                       time.Duration // > 0: crystallized that long ago
}

func cbInsertAgent(t *testing.T, pool *Pool, ctx context.Context, specialties []string, lastBriefing *time.Time) string {
	t.Helper()
	id := fmt.Sprintf("cbrf_%d", time.Now().UnixNano())
	_, err := pool.Exec(ctx, `INSERT INTO agents (id, display_name, specialties, status, last_briefing_at)
		VALUES ($1, $2, $3, 'active', $4)`, id, "Canonical Briefing "+id[5:], specialties, lastBriefing)
	require.NoError(t, err)
	t.Cleanup(func() {
		c := context.Background()
		_, _ = pool.Exec(c, `DELETE FROM votes WHERE voter_id = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = $1`, id)
		_, _ = pool.Exec(c, `DELETE FROM agents WHERE id = $1`, id)
	})
	return id
}

func cbInsertPost(t *testing.T, pool *Pool, ctx context.Context, title string, s cbPostSeed) string {
	t.Helper()
	if s.postType == "" {
		s.postType = "post"
	}
	if s.status == "" {
		s.status = "open"
	}
	if s.publication == "" {
		s.publication = "published"
	}
	if s.moderation == "" {
		s.moderation = "approved"
	}
	if s.visibility == "" {
		s.visibility = "public"
	}
	if s.byType == "" {
		s.byType = "agent"
	}
	if s.tags == nil {
		s.tags = []string{"cbrf"}
	}
	created := time.Now().UTC().Add(-s.age)
	var deletedAt, cid, crystallizedAt any
	if s.deleted {
		deletedAt = created
	}
	if s.crystallizedAgo > 0 {
		cid, crystallizedAt = "bafycbrf"+title, time.Now().UTC().Add(-s.crystallizedAgo)
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO posts (type, title, description, tags, status, posted_by_type, posted_by_id,
			publication_state, moderation_state, visibility, created_at, updated_at, deleted_at,
			crystallization_cid, crystallized_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $11, $12, $13, $14)
		RETURNING id::text`,
		s.postType, title, "Canonical briefing fixture body for "+title, s.tags, s.status, s.byType, s.byID,
		s.publication, s.moderation, s.visibility, created, deletedAt, cid, crystallizedAt).Scan(&id))
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM posts WHERE id = $1`, id) })
	return id
}

func cbInsertReply(t *testing.T, pool *Pool, ctx context.Context, postID, authorType, authorID string, ago time.Duration, deleted bool) string {
	t.Helper()
	at := time.Now().UTC().Add(-ago)
	var deletedAt any
	if deleted {
		deletedAt = at
	}
	var id string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO replies (post_id, author_type, author_id, body, created_at, updated_at, deleted_at)
		VALUES ($1, $2, $3, 'canonical briefing reply', $4, $4, $5) RETURNING id::text`,
		postID, authorType, authorID, at, deletedAt).Scan(&id))
	return id
}

func cbInsertVote(t *testing.T, pool *Pool, ctx context.Context, targetType, targetID, voterID, direction string, confirmed bool, ago time.Duration) {
	t.Helper()
	_, err := pool.Exec(ctx, `INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed, created_at)
		VALUES ($1, $2, 'agent', $3, $4, $5, $6)`, targetType, targetID, voterID, direction, confirmed, time.Now().UTC().Add(-ago))
	require.NoError(t, err)
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM votes WHERE target_id = $1 AND voter_id = $2`, targetID, voterID)
	})
}

func TestCanonicalBriefing_OpenItems(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	someone := authorHuman(ctx, t, pool, "someone")
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	mine := func(title string, s cbPostSeed) string {
		s.byID = me
		return cbInsertPost(t, pool, ctx, title, s)
	}

	canonical := mine("canonical post waiting", cbPostSeed{age: 50*time.Hour + 30*time.Minute})
	second := mine("second post waiting", cbPostSeed{age: 40 * time.Hour})
	third := mine("third post waiting", cbPostSeed{age: 30 * time.Hour})
	systemOnly := mine("only a system verdict", cbPostSeed{age: 20 * time.Hour})
	cbInsertReply(t, pool, ctx, systemOnly, "system", "moderation", time.Hour, false)
	deletedOnly := mine("only a deleted reply", cbPostSeed{age: 10 * time.Hour})
	cbInsertReply(t, pool, ctx, deletedOnly, "human", someone, time.Hour, true)
	family := mine("family post waiting", cbPostSeed{visibility: "family", age: 5 * time.Hour})

	humanReplied := mine("a human replied", cbPostSeed{})
	cbInsertReply(t, pool, ctx, humanReplied, "human", someone, time.Hour, false)
	selfReplied := mine("I replied myself", cbPostSeed{})
	cbInsertReply(t, pool, ctx, selfReplied, "agent", me, time.Hour, false)
	mine("draft", cbPostSeed{publication: "draft"})
	mine("archived", cbPostSeed{publication: "archived"})
	mine("pending moderation", cbPostSeed{moderation: "pending"})
	mine("rejected", cbPostSeed{moderation: "rejected"})
	mine("deleted", cbPostSeed{deleted: true})
	mine("closed", cbPostSeed{status: "closed"})
	cbInsertPost(t, pool, ctx, "someone else's post", cbPostSeed{byID: other})

	got, err := NewCanonicalBriefingRepository(pool).GetOpenItemsForAgent(ctx, me)
	require.NoError(t, err)
	require.Equal(t, 6, got.PostsNoReplies, "every live published approved post of mine without a contributor reply")

	var ids []string
	for _, it := range got.Items {
		ids = append(ids, it.ID)
	}
	require.Equal(t, []string{canonical, second, third, systemOnly, deletedOnly, family}, ids, "oldest first")
	require.Equal(t, "post", got.Items[0].Type)
	require.Equal(t, "open", got.Items[0].Status)
	require.Equal(t, 50, got.Items[0].AgeHours)
	for _, it := range got.Items {
		require.Equal(t, "post", it.Type, "every post is type post (idx 68)")
	}
}

func TestCanonicalBriefing_OpenItems_ItemsCappedCountNot(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	for i := 0; i < 12; i++ {
		cbInsertPost(t, pool, ctx, fmt.Sprintf("waiting %02d", i), cbPostSeed{byID: me, age: time.Duration(i+1)*time.Hour + 30*time.Minute})
	}
	got, err := NewCanonicalBriefingRepository(pool).GetOpenItemsForAgent(ctx, me)
	require.NoError(t, err)
	require.Equal(t, 12, got.PostsNoReplies)
	require.Len(t, got.Items, 10)
	require.Equal(t, 12, got.Items[0].AgeHours)

	empty, err := NewCanonicalBriefingRepository(pool).GetOpenItemsForAgent(ctx, "cbrf_nobody")
	require.NoError(t, err)
	require.NotNil(t, empty.Items, "items is never nil")
	require.Empty(t, empty.Items)
}

func TestCanonicalBriefing_SuggestedActions(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	someone := authorHuman(ctx, t, pool, "someone")
	lastBriefing := time.Now().UTC().Add(-time.Hour)
	me := cbInsertAgent(t, pool, ctx, nil, &lastBriefing)
	other := cbInsertAgent(t, pool, ctx, nil, nil)

	post := cbInsertPost(t, pool, ctx, "my post with new replies", cbPostSeed{byID: me})
	human := cbInsertReply(t, pool, ctx, post, "human", someone, 10*time.Minute, false)
	agent := cbInsertReply(t, pool, ctx, post, "agent", other, 5*time.Minute, false)
	cbInsertReply(t, pool, ctx, post, "agent", me, 4*time.Minute, false)            // mine
	cbInsertReply(t, pool, ctx, post, "system", "moderation", 3*time.Minute, false) // a verdict
	cbInsertReply(t, pool, ctx, post, "human", someone, 2*time.Minute, true)        // deleted
	cbInsertReply(t, pool, ctx, post, "human", someone, 2*time.Hour, false)         // before my last briefing
	theirs := cbInsertPost(t, pool, ctx, "someone else's post", cbPostSeed{byID: other})
	cbInsertReply(t, pool, ctx, theirs, "human", someone, time.Minute, false)
	gone := cbInsertPost(t, pool, ctx, "my deleted post", cbPostSeed{byID: me, deleted: true})
	cbInsertReply(t, pool, ctx, gone, "human", someone, time.Minute, false)

	got, err := NewCanonicalBriefingRepository(pool).GetSuggestedActionsForAgent(ctx, me)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, agent, got[0].TargetID, "newest first")
	require.Equal(t, human, got[1].TargetID)
	for _, a := range got {
		require.Equal(t, "respond_to_reply", a.Action)
		require.Equal(t, "my post with new replies", a.TargetTitle)
		require.NotEmpty(t, a.Reason)
	}
}

func TestCanonicalBriefing_SuggestedActions_AtMostFiveAndNeverBriefedMeansAll(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	someone := authorHuman(ctx, t, pool, "someone")
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	post := cbInsertPost(t, pool, ctx, "busy post", cbPostSeed{byID: me})
	for i := 0; i < 7; i++ {
		cbInsertReply(t, pool, ctx, post, "human", someone, time.Duration(i+1)*24*time.Hour, false)
	}
	got, err := NewCanonicalBriefingRepository(pool).GetSuggestedActionsForAgent(ctx, me)
	require.NoError(t, err)
	require.Len(t, got, 5)
}

func TestCanonicalBriefing_Opportunities(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	someone := authorHuman(ctx, t, pool, "someone")
	tag := fmt.Sprintf("cbrf-tag-%d", time.Now().UnixNano())
	me := cbInsertAgent(t, pool, ctx, []string{tag}, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	theirs := func(title string, s cbPostSeed) string {
		s.byID = other
		if s.tags == nil {
			s.tags = []string{tag, "extra"}
		}
		return cbInsertPost(t, pool, ctx, title, s)
	}

	fresh := theirs("fresh canonical post", cbPostSeed{age: 90 * time.Minute})
	replied := theirs("replied canonical post", cbPostSeed{age: 30 * time.Minute})
	cbInsertReply(t, pool, ctx, replied, "human", someone, 10*time.Minute, false)
	cbInsertReply(t, pool, ctx, replied, "agent", me, 5*time.Minute, false)
	cbInsertReply(t, pool, ctx, replied, "system", "moderation", time.Minute, false)
	cbInsertReply(t, pool, ctx, replied, "human", someone, time.Minute, true)
	idea := theirs("open post two hours old", cbPostSeed{age: 2 * time.Hour})
	problem := theirs("open post three hours old", cbPostSeed{age: 3 * time.Hour})

	cbInsertPost(t, pool, ctx, "my own post", cbPostSeed{byID: me, tags: []string{tag}})
	theirs("closed", cbPostSeed{status: "closed"})
	theirs("stale", cbPostSeed{status: "stale"})
	theirs("draft", cbPostSeed{publication: "draft"})
	theirs("pending", cbPostSeed{moderation: "pending"})
	theirs("family", cbPostSeed{visibility: "family"})
	theirs("deleted", cbPostSeed{deleted: true})
	theirs("no tag overlap", cbPostSeed{tags: []string{"unrelated"}})

	repo := NewCanonicalBriefingRepository(pool)
	got, err := repo.GetOpportunitiesForAgent(ctx, me, []string{tag}, 5)
	require.NoError(t, err)
	require.Equal(t, 4, got.ProblemsInMyDomain, "the count uses the same predicate as the items")
	var ids []string
	for _, o := range got.Items {
		ids = append(ids, o.ID)
	}
	require.Equal(t, []string{fresh, idea, problem, replied}, ids, "fewest contributor replies first, then newest")
	require.Equal(t, 2, got.Items[3].ApproachesCount, "live human and agent replies only")
	require.Equal(t, other, got.Items[0].PostedBy)
	require.ElementsMatch(t, []string{tag, "extra"}, got.Items[0].Tags)
	require.Equal(t, 1, got.Items[0].AgeHours)

	limited, err := repo.GetOpportunitiesForAgent(ctx, me, []string{tag}, 2)
	require.NoError(t, err)
	require.Equal(t, 4, limited.ProblemsInMyDomain)
	require.Len(t, limited.Items, 2)
}

func TestCanonicalBriefing_ReputationChanges(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	voters := []string{cbInsertAgent(t, pool, ctx, nil, nil), cbInsertAgent(t, pool, ctx, nil, nil), cbInsertAgent(t, pool, ctx, nil, nil)}

	myPost := cbInsertPost(t, pool, ctx, "my voted post", cbPostSeed{byID: me})
	theirPost := cbInsertPost(t, pool, ctx, "their post I replied to", cbPostSeed{byID: other})
	myReply := cbInsertReply(t, pool, ctx, theirPost, "agent", me, 3*time.Hour, false)
	theirReply := cbInsertReply(t, pool, ctx, theirPost, "agent", other, 3*time.Hour, false)
	since := time.Now().UTC().Add(-2 * time.Hour)

	cbInsertVote(t, pool, ctx, "post", myPost, voters[0], "up", true, 30*time.Minute)
	cbInsertVote(t, pool, ctx, "reply", myReply, voters[1], "down", true, 20*time.Minute)
	cbInsertVote(t, pool, ctx, "reply", myReply, voters[2], "up", true, 10*time.Minute)
	cbInsertVote(t, pool, ctx, "reply", myReply, voters[0], "up", false, 5*time.Minute) // unconfirmed
	cbInsertVote(t, pool, ctx, "post", myPost, voters[1], "up", true, 3*time.Hour)      // before since
	cbInsertVote(t, pool, ctx, "reply", theirReply, voters[0], "up", true, time.Minute) // not mine
	cbInsertVote(t, pool, ctx, "post", theirPost, voters[0], "up", true, time.Minute)   // not mine

	got, err := NewCanonicalBriefingRepository(pool).GetReputationChangesSince(ctx, me, since)
	require.NoError(t, err)
	require.Equal(t, "+19", got.SinceLastCheck)
	require.Len(t, got.Breakdown, 3)
	require.Equal(t, "reply_upvoted", got.Breakdown[0].Reason, "newest first")
	require.Equal(t, theirPost, got.Breakdown[0].PostID, "a reply event names the post it belongs to")
	require.Equal(t, "their post I replied to", got.Breakdown[0].PostTitle)
	require.Equal(t, 10, got.Breakdown[0].Delta)
	require.Equal(t, "reply_downvoted", got.Breakdown[1].Reason)
	require.Equal(t, -1, got.Breakdown[1].Delta)
	require.Equal(t, "post_upvoted", got.Breakdown[2].Reason)
	require.Equal(t, myPost, got.Breakdown[2].PostID)

	none, err := NewCanonicalBriefingRepository(pool).GetReputationChangesSince(ctx, me, time.Now().UTC())
	require.NoError(t, err)
	require.Equal(t, "+0", none.SinceLastCheck)
	require.NotNil(t, none.Breakdown)
	require.Empty(t, none.Breakdown)
}

func TestCanonicalBriefing_Crystallizations(t *testing.T) {
	pool := setupTestDB(t)
	t.Cleanup(pool.Close)
	ctx := context.Background()
	me := cbInsertAgent(t, pool, ctx, nil, nil)
	other := cbInsertAgent(t, pool, ctx, nil, nil)
	since := time.Now().UTC().Add(-time.Hour)

	mine := cbInsertPost(t, pool, ctx, "my crystallized post", cbPostSeed{byID: me, age: 10 * 24 * time.Hour, crystallizedAgo: 20 * time.Minute})
	repliedTo := cbInsertPost(t, pool, ctx, "crystallized with my reply", cbPostSeed{byID: other, age: 10 * 24 * time.Hour, crystallizedAgo: 10 * time.Minute})
	cbInsertReply(t, pool, ctx, repliedTo, "agent", me, 9*24*time.Hour, false)

	late := cbInsertPost(t, pool, ctx, "my reply came after the pin", cbPostSeed{byID: other, age: 10 * 24 * time.Hour, crystallizedAgo: 30 * time.Minute})
	cbInsertReply(t, pool, ctx, late, "agent", me, 5*time.Minute, false)
	deletedReply := cbInsertPost(t, pool, ctx, "my reply was deleted", cbPostSeed{byID: other, age: 10 * 24 * time.Hour, crystallizedAgo: 30 * time.Minute})
	cbInsertReply(t, pool, ctx, deletedReply, "agent", me, 9*24*time.Hour, true)
	cbInsertPost(t, pool, ctx, "crystallized before since", cbPostSeed{byID: me, age: 10 * 24 * time.Hour, crystallizedAgo: 2 * time.Hour})
	cbInsertPost(t, pool, ctx, "my deleted crystallized post", cbPostSeed{byID: me, deleted: true, crystallizedAgo: 20 * time.Minute})
	cbInsertPost(t, pool, ctx, "not crystallized", cbPostSeed{byID: me})
	cbInsertPost(t, pool, ctx, "someone else's crystallized post", cbPostSeed{byID: other, crystallizedAgo: 20 * time.Minute})

	got, err := NewCanonicalBriefingRepository(pool).GetRecentCrystallizations(ctx, me, since)
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, repliedTo, got[0].PostID, "newest crystallization first")
	require.Equal(t, "crystallized with my reply", got[0].PostTitle)
	require.Equal(t, "bafycbrfcrystallized with my reply", got[0].CID)
	require.NotEmpty(t, got[0].CrystallizedAt)
	require.Equal(t, mine, got[1].PostID)

	none, err := NewCanonicalBriefingRepository(pool).GetRecentCrystallizations(ctx, me, time.Now().UTC())
	require.NoError(t, err)
	require.NotNil(t, none)
	require.Empty(t, none)
}
