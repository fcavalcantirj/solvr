package db

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type leaderboardFetch func(opts models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error)

// leaderboardEntriesFor pages through a whole leaderboard view and returns the entries of
// ids, rank cleared: ranks depend on every other row in the shared test database.
func leaderboardEntriesFor(t *testing.T, fetch leaderboardFetch, opts models.LeaderboardOptions, ids ...string) map[string]models.LeaderboardEntry {
	t.Helper()
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	out := map[string]models.LeaderboardEntry{}
	for offset := 0; ; offset += 100 {
		opts.Limit, opts.Offset = 100, offset
		entries, total, err := fetch(opts)
		require.NoError(t, err)
		for _, e := range entries {
			if want[e.ID] {
				e.Rank = 0
				out[e.ID] = e
			}
		}
		if len(entries) == 0 || offset+100 >= total {
			return out
		}
	}
}

// productionSourcesContaining lists the non-test Go files under the backend that contain needle.
func productionSourcesContaining(t *testing.T, needle string) []string {
	t.Helper()
	root := backendRoot(t)
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if name := entry.Name(); name == "vendor" || name == "node_modules" || (strings.HasPrefix(name, ".") && path != root) {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if strings.Contains(string(src), needle) {
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return err
			}
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	require.NoError(t, err)
	return files
}

// Task idx 76 steps 3-4 (feature:leaderboards): the router serves both leaderboards from
// CanonicalLeaderboardRepository; the legacy repository is unwired and goes with the legacy
// tables, as does the cutover freeze that reads them.
func TestLegacyLeaderboards_ServedByTheCanonicalRepository(t *testing.T) {
	assert.Empty(t, productionSourcesContaining(t, "NewLeaderboardRepository("),
		"the legacy leaderboard repository was deleted with the legacy tables (idx 68)")
	assert.Contains(t, productionSourcesContaining(t, "NewCanonicalLeaderboardRepository("), "internal/api/router.go")

	d := LegacyDependencyDispositions["feature:leaderboards"]
	assert.Equal(t, LegacyActionRefactor, d.Action)
	assert.True(t, d.Done, "feature:leaderboards is refactored and verified")
	for _, key := range []string{"code:internal/db/leaderboard.go", "code:internal/db/leaderboard_tags.go"} {
		assertLegacyDependencyGone(t, key)
	}
	d, ok := LegacyDependencyDispositions["code:internal/db/reputation_history.go"]
	require.True(t, ok)
	assert.Equal(t, LegacyActionKeep, d.Action, "the cutover's reputation freeze stays with the pre-archive cutover tool")
}

// Task idx 76 steps 3-4 (feature:leaderboards): the contribution cutover keeps earned
// reputation as history. For every leaderboard view (all/agents/users x all_time/monthly/
// weekly, and per tag), the canonical leaderboard after MigrateContributions and
// RemapLegacyRelations shows each fixture entity with exactly the reputation and key stats
// the legacy leaderboard showed before it. Migrated rows award nothing new: an approach and
// a vote on it (0 points under the legacy rules) still give 0. After the cutover, confirmed
// votes on posts and replies score live, including new votes on migrated replies, while
// writing a post or reply scores nothing by itself. A second cutover run changes nothing.
func TestCanonicalLeaderboard_KeepsEarnedReputationAcrossTheCutover(t *testing.T) {
	pool, archiveLegacy := newPreArchiveScratchDatabase(t)
	t.Cleanup(pool.Close) // registered first, so it runs after the fixture cleanups
	ctx := context.Background()

	sfx := time.Now().Format("150405.000000")
	holder, newcomer := "agent_lbh_"+sfx, "agent_lbn_"+sfx
	voter, late := "agent_lbv_"+sfx, "agent_lbw_"+sfx
	insertRemapAgent(t, pool, ctx, holder)
	insertRemapAgent(t, pool, ctx, newcomer)
	n := time.Now().UnixNano() % 1000000000
	user, err := NewUserRepository(pool).Create(ctx, &models.User{
		Username: fmt.Sprintf("lbc%d", n), DisplayName: "Leaderboard Cutover User",
		Email: fmt.Sprintf("lbc%d@example.com", n), AuthProvider: models.AuthProviderGitHub,
		AuthProviderID: fmt.Sprintf("gh_lbc%d", n), Role: models.UserRoleUser,
	})
	require.NoError(t, err)
	owners := []string{holder, newcomer, user.ID}
	t.Cleanup(func() {
		c := context.Background()
		pool.Exec(c, `DELETE FROM reputation_history WHERE owner_id = ANY($1)`, owners)                   //nolint:errcheck
		pool.Exec(c, `DELETE FROM votes WHERE voter_id LIKE $1 OR voter_id LIKE $2`, voter+"%", late+"%") //nolint:errcheck
		pool.Exec(c, `DELETE FROM comments WHERE author_id = ANY($1)`, owners)                            //nolint:errcheck
		pool.Exec(c, `DELETE FROM responses WHERE author_id = ANY($1)`, owners)                           //nolint:errcheck
		pool.Exec(c, `DELETE FROM approaches WHERE author_id = ANY($1)`, owners)                          //nolint:errcheck
		pool.Exec(c, `DELETE FROM answers WHERE author_id = ANY($1)`, owners)                             //nolint:errcheck
		pool.Exec(c, `DELETE FROM replies WHERE author_id = ANY($1)`, owners)                             //nolint:errcheck
		pool.Exec(c, `DELETE FROM posts WHERE posted_by_id = ANY($1)`, owners)                            //nolint:errcheck
		pool.Exec(c, `DELETE FROM agents WHERE id IN ($1, $2)`, holder, newcomer)                         //nolint:errcheck
		pool.Exec(c, `DELETE FROM users WHERE id = $1`, user.ID)                                          //nolint:errcheck
	})

	tag := "lbc" + strings.ReplaceAll(sfx, ".", "")
	tags := []string{tag}
	old := time.Now().UTC().AddDate(0, 0, -60)
	post := func(postType, status, authorType, author string) string {
		return insertTestPostWithAuthor(t, pool, ctx, postType, "lbc "+postType+" "+sfx, "body", tags, status, authorType, author)
	}
	untagged := func(author string) string {
		return insertTestPostWithAuthor(t, pool, ctx, "post", "lbc untagged "+sfx, "body", []string{"lbc-other"}, "open", "agent", author)
	}
	solved := post("problem", "solved", "agent", holder)
	open := post("problem", "open", "agent", holder)
	idea := post("idea", "open", "agent", holder)
	oldIdea := post("idea", "open", "agent", holder)
	_, err = pool.Exec(ctx, `UPDATE posts SET created_at = $1 WHERE id = $2`, old, oldIdea)
	require.NoError(t, err)
	question := post("question", "open", "human", user.ID)
	userSolved := post("problem", "solved", "human", user.ID)
	holderUntagged := untagged(holder)

	answer := func(authorType, author string, accepted bool) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO answers (question_id, author_type, author_id, content, is_accepted)
			VALUES ($1, $2, $3, 'lbc answer', $4) RETURNING id::text`, question, authorType, author, accepted).Scan(&id))
		return id
	}
	accepted := answer("agent", holder, true)
	plain := answer("agent", holder, false)
	userAnswer := answer("human", user.ID, false)
	_, err = pool.Exec(ctx, `UPDATE posts SET accepted_answer_id = $1 WHERE id = $2`, accepted, question)
	require.NoError(t, err)
	var response string
	require.NoError(t, pool.QueryRow(ctx, `
		INSERT INTO responses (idea_id, author_type, author_id, content, response_type)
		VALUES ($1, 'agent', $2, 'lbc response', 'support') RETURNING id::text`, idea, holder).Scan(&response))
	insertComment(t, pool, ctx, "post", solved, holder, "lbc agent comment")
	_, err = pool.Exec(ctx, `INSERT INTO comments (target_type, target_id, author_type, author_id, content)
		VALUES ('post', $1, 'human', $2, 'lbc human comment')`, userSolved, user.ID)
	require.NoError(t, err)
	approach := insertApproach(t, pool, ctx, solved, newcomer, "succeeded", false)

	votes := 0
	vote := func(targetType, targetID, direction string, at *time.Time) {
		votes++
		_, err := pool.Exec(ctx, `
			INSERT INTO votes (target_type, target_id, voter_type, voter_id, direction, confirmed, created_at)
			VALUES ($1, $2, 'agent', $3, $4, true, COALESCE($5, NOW()))`,
			targetType, targetID, fmt.Sprintf("%s_%d", voter, votes), direction, at)
		require.NoError(t, err)
	}
	vote("post", solved, "up", nil)
	vote("post", open, "down", nil)
	vote("answer", accepted, "up", nil)
	vote("answer", accepted, "up", &old)
	vote("answer", plain, "down", nil)
	vote("response", response, "up", nil)
	vote("post", userSolved, "up", nil)
	vote("answer", userAnswer, "up", nil)
	vote("approach", approach, "up", nil)
	vote("post", holderUntagged, "up", nil)

	type view struct {
		name string
		tag  bool
		opts models.LeaderboardOptions
	}
	var views []view
	for _, tf := range []string{"all_time", "monthly", "weekly"} {
		for _, typ := range []string{"all", "agents", "users"} {
			views = append(views, view{name: tf + "/" + typ, opts: models.LeaderboardOptions{Type: typ, Timeframe: tf}})
		}
		views = append(views, view{name: "tag/" + tf, tag: true, opts: models.LeaderboardOptions{Type: "all", Timeframe: tf}})
	}
	fetchAll := func(main, byTag func(context.Context, models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error)) map[string]map[string]models.LeaderboardEntry {
		out := map[string]map[string]models.LeaderboardEntry{}
		for _, v := range views {
			f := main
			if v.tag {
				f = byTag
			}
			out[v.name] = leaderboardEntriesFor(t, func(o models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
				return f(ctx, o)
			}, v.opts, owners...)
		}
		return out
	}

	// What the legacy LeaderboardRepository (deleted, idx 68) served for this fixture in every
	// view, ranks aside: measured on e726cb7f by running this test against it.
	before := map[string]map[string]models.LeaderboardEntry{
		"all_time/agents": {
			holder:   {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 265, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 4, TotalContributions: 6}},
			newcomer: {ID: newcomer, Type: "agent", DisplayName: newcomer, AvatarURL: "", Reputation: 0, KeyStats: models.LeaderboardStats{ProblemsSolved: 0, AnswersAccepted: 0, UpvotesReceived: 0, TotalContributions: 0}},
		},
		"all_time/all": {
			holder:   {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 265, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 4, TotalContributions: 6}},
			newcomer: {ID: newcomer, Type: "agent", DisplayName: newcomer, AvatarURL: "", Reputation: 0, KeyStats: models.LeaderboardStats{ProblemsSolved: 0, AnswersAccepted: 0, UpvotesReceived: 0, TotalContributions: 0}},
			user.ID:  {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 141, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"all_time/users": {
			user.ID: {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 141, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"monthly/agents": {
			holder:   {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 248, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 3, TotalContributions: 5}},
			newcomer: {ID: newcomer, Type: "agent", DisplayName: newcomer, AvatarURL: "", Reputation: 0, KeyStats: models.LeaderboardStats{ProblemsSolved: 0, AnswersAccepted: 0, UpvotesReceived: 0, TotalContributions: 0}},
		},
		"monthly/all": {
			holder:   {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 248, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 3, TotalContributions: 5}},
			newcomer: {ID: newcomer, Type: "agent", DisplayName: newcomer, AvatarURL: "", Reputation: 0, KeyStats: models.LeaderboardStats{ProblemsSolved: 0, AnswersAccepted: 0, UpvotesReceived: 0, TotalContributions: 0}},
			user.ID:  {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 141, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"monthly/users": {
			user.ID: {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 141, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"tag/all_time": {
			holder:  {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 154, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 3, TotalContributions: 5}},
			user.ID: {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 104, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"tag/monthly": {
			holder:  {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 152, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 2, TotalContributions: 4}},
			user.ID: {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 104, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"tag/weekly": {
			holder:  {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 152, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 2, TotalContributions: 4}},
			user.ID: {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 104, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"weekly/agents": {
			holder:   {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 248, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 3, TotalContributions: 5}},
			newcomer: {ID: newcomer, Type: "agent", DisplayName: newcomer, AvatarURL: "", Reputation: 0, KeyStats: models.LeaderboardStats{ProblemsSolved: 0, AnswersAccepted: 0, UpvotesReceived: 0, TotalContributions: 0}},
		},
		"weekly/all": {
			holder:   {ID: holder, Type: "agent", DisplayName: holder, AvatarURL: "", Reputation: 248, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 1, UpvotesReceived: 3, TotalContributions: 5}},
			newcomer: {ID: newcomer, Type: "agent", DisplayName: newcomer, AvatarURL: "", Reputation: 0, KeyStats: models.LeaderboardStats{ProblemsSolved: 0, AnswersAccepted: 0, UpvotesReceived: 0, TotalContributions: 0}},
			user.ID:  {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 141, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
		"weekly/users": {
			user.ID: {ID: user.ID, Type: "user", DisplayName: "Leaderboard Cutover User", AvatarURL: "", Reputation: 141, KeyStats: models.LeaderboardStats{ProblemsSolved: 1, AnswersAccepted: 0, UpvotesReceived: 2, TotalContributions: 3}},
		},
	}
	// The fixture, scored by the legacy rules: holder = 125 (solved) + 25 (open) + 15 + 15
	// (ideas) + 60 (accepted answer) + 10 (answer) + 5 (response) + 2 (comment) + votes
	// (+2 -1 +2 +2 -1 +2, and +2 on the untagged post) = 265; the user = 125 + 10 + 2 + 2 + 2
	// = 141; the newcomer's approach and the vote on it score nothing.
	require.Equal(t, 265, before["all_time/all"][holder].Reputation)
	require.Equal(t, 141, before["all_time/all"][user.ID].Reputation)
	require.Equal(t, 0, before["all_time/all"][newcomer].Reputation)
	require.Equal(t, 265-15-2, before["monthly/all"][holder].Reputation, "the 60-day-old idea and vote fall outside the month")
	require.Equal(t, 100+50+3*2-2, before["tag/all_time"][holder].Reputation,
		"tag rules: solved, accepted, votes on tagged posts and on answers to tagged questions, not the untagged post")
	require.NotContains(t, before["tag/all_time"], newcomer, "a tag leaderboard lists only positive reputation")

	_, err = MigrateContributions(ctx, pool)
	require.NoError(t, err)
	report, err := RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, report.ReputationHistory, int64(20), "the fixture's earned events are frozen")

	canonical := NewCanonicalLeaderboardRepository(pool)
	fetchCanonical := func() map[string]map[string]models.LeaderboardEntry {
		return fetchAll(canonical.GetLeaderboard, func(c context.Context, o models.LeaderboardOptions) ([]models.LeaderboardEntry, int, error) {
			return canonical.GetLeaderboardByTag(c, tag, o)
		})
	}
	after := fetchCanonical()
	for _, v := range views {
		assert.Equal(t, before[v.name], after[v.name], "%s: earned reputation and key stats are unchanged by the cutover", v.name)
	}

	// After the cutover, votes score live on posts and replies, migrated ones included;
	// writing a post or reply scores nothing by itself.
	replies, posts := NewReplyRepository(pool), NewPostRepository(pool)
	acceptedReply, _, _, _ := replyFor(t, pool, ctx, "answer", accepted)
	approachReply, _, _, _ := replyFor(t, pool, ctx, "approach", approach)
	require.NoError(t, replies.Vote(ctx, acceptedReply, "agent", late+"_1", "up"))
	require.NoError(t, replies.Vote(ctx, approachReply, "agent", late+"_2", "up"))
	native := post("post", "open", "agent", newcomer)
	require.NoError(t, posts.Vote(ctx, native, "agent", late+"_3", "up"))
	nativeReply, err := replies.Create(ctx, &models.Reply{PostID: solved, AuthorType: models.AuthorTypeAgent, AuthorID: newcomer, Body: "lbc native reply"})
	require.NoError(t, err)
	require.NoError(t, replies.Vote(ctx, nativeReply.ID, "agent", late+"_4", "down"))
	require.NoError(t, posts.Vote(ctx, untagged(newcomer), "agent", late+"_5", "up"))

	live := fetchCanonical()
	for _, name := range []string{"all_time/all", "all_time/agents", "monthly/all", "weekly/agents"} {
		h := before[name][holder]
		h.Reputation += 2
		h.KeyStats.UpvotesReceived++
		h.KeyStats.TotalContributions++
		assert.Equal(t, h, live[name][holder], "%s: a new vote on a migrated answer scores", name)
		assert.Equal(t, 2+2-1+2, live[name][newcomer].Reputation, "%s: new votes score, the migrated approach vote does not", name)
		assert.Equal(t, 3, live[name][newcomer].KeyStats.UpvotesReceived, name)
	}
	assert.Equal(t, 2+2-1, live["tag/all_time"][newcomer].Reputation, "votes on tagged posts and on replies to them, not on the untagged post")
	assert.Equal(t, before["tag/all_time"][holder].Reputation+2, live["tag/all_time"][holder].Reputation)
	assert.Equal(t, before["all_time/users"][user.ID], live["all_time/users"][user.ID])

	again, err := RemapLegacyRelations(ctx, pool)
	require.NoError(t, err)
	assert.Zero(t, again.ReputationHistory, "a second cutover run freezes nothing new")
	assert.Equal(t, live, fetchCanonical(), "a second cutover run changes no leaderboard")

	archiveLegacy()
	assert.Equal(t, live, fetchCanonical(), "the legacy archive changes no leaderboard")
}
