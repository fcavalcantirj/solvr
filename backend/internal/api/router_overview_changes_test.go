package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// A post taken off the public overview leaves every instance's snapshot when the change
// commits, not when the snapshot's 30 s run out (idx 77 step 5, migration 000123). Measured on
// HEAD before it (idx 77 slice 7 spike, live API): a deleted post, an edited title, a moderation
// rejection and a post whose only reply was deleted stayed on GET /v1/overview for 30.0-30.2 s.
// Two instances below share a scratch database and nothing else, so the one that did not serve
// the write hears of it only through the database.

// overviewPostTitle returns the title GET /v1/overview shows for post id ("" when it is not listed).
func overviewPostTitle(t *testing.T, base, id string) string {
	t.Helper()
	var ov struct {
		Data struct {
			Posts struct {
				Items []struct {
					ID    string `json:"id"`
					Title string `json:"title"`
				} `json:"items"`
			} `json:"posts"`
		} `json:"data"`
	}
	getJSON(t, base+"/v1/overview", &ov)
	for _, item := range ov.Data.Posts.Items {
		if item.ID == id {
			return item.Title
		}
	}
	return ""
}

// overviewShowsWithin waits until every instance's overview shows want for post id ("" = not
// listed) and fails when one still shows something else after the deadline.
func overviewShowsWithin(t *testing.T, deadline time.Duration, id, want, what string, instances ...*roomInstance) {
	t.Helper()
	start := time.Now()
	for _, inst := range instances {
		got := overviewPostTitle(t, inst.ts.URL, id)
		for got != want && time.Since(start) < deadline {
			time.Sleep(25 * time.Millisecond)
			got = overviewPostTitle(t, inst.ts.URL, id)
		}
		require.Equal(t, want, got, "%s: the overview still shows %q %s after the change", what, got, time.Since(start).Round(time.Millisecond))
	}
	t.Logf("%s: every overview caught up in %s", what, time.Since(start).Round(time.Millisecond))
}

func TestOverview_APostTakenOffLeavesEveryInstancesSnapshotAtCommit(t *testing.T) {
	t.Setenv("DATABASE_URL", newHomepageScratchURL(t))
	// No moderator: a content edit then keeps an approved post open (with one it goes back to
	// pending_review and leaves the overview), and nothing calls the live moderation service.
	t.Setenv("GROQ_API_KEY", "")
	a := startRoomInstance(t, RoomRelayOptions{})
	b := startRoomInstance(t, RoomRelayOptions{})
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	agentID, key := registerRoomTestAgent(t, a.ts)
	posts, replies := db.NewPostRepository(a.pool), db.NewReplyRepository(a.pool)
	listed := func(title string) (string, string) {
		t.Helper()
		post, err := posts.Create(ctx, &models.Post{
			Type: models.PostTypeQuestion, Title: title, Description: title + ", the description of the post",
			Tags: []string{"overview"}, PostedByType: models.AuthorTypeAgent, PostedByID: agentID,
			Status: models.PostStatusOpen,
		})
		require.NoError(t, err)
		// Approved the way the moderator approves (the only path to moderation_state approved),
		// so an author's edit keeps it published.
		require.NoError(t, posts.UpdateStatus(ctx, post.ID, models.PostStatusOpen))
		reply, err := replies.Create(ctx, &models.Reply{PostID: post.ID, AuthorType: models.AuthorTypeAgent,
			AuthorID: agentID, Body: "A live reply on " + title})
		require.NoError(t, err)
		return post.ID, reply.ID
	}
	deleted, _ := listed("A post its author deletes")
	edited, _ := listed("A post whose title is edited")
	rejected, _ := listed("A post moderation rejects")
	orphaned, onlyReply := listed("A post that loses its only reply")
	for _, inst := range []*roomInstance{a, b} {
		for id, title := range map[string]string{deleted: "A post its author deletes", edited: "A post whose title is edited",
			rejected: "A post moderation rejects", orphaned: "A post that loses its only reply"} {
			require.Equal(t, title, overviewPostTitle(t, inst.ts.URL, id), "the snapshot lists the post first")
		}
	}

	const deadline = 3 * time.Second
	status, out := doJSON(t, http.MethodDelete, a.ts.URL+"/v1/posts/"+deleted, key, "")
	require.Equal(t, http.StatusNoContent, status, "delete: %v", out)
	overviewShowsWithin(t, deadline, deleted, "", "post deleted on instance a", a, b)

	status, out = doJSONAtCurrentVersion(t, http.MethodPatch, b.ts.URL+"/v1/posts/"+edited, key, `{"title":"A post whose title was edited"}`)
	require.Equal(t, http.StatusOK, status, "edit: %v", out)
	overviewShowsWithin(t, deadline, edited, "A post whose title was edited", "title edited on instance b", a, b)

	_, err := a.pool.Exec(ctx, `UPDATE posts SET status = 'rejected' WHERE id = $1`, rejected)
	require.NoError(t, err)
	overviewShowsWithin(t, deadline, rejected, "", "post rejected by a write no instance served", a, b)

	status, out = doJSON(t, http.MethodDelete, a.ts.URL+"/v1/replies/"+onlyReply, key, "")
	require.Equal(t, http.StatusOK, status, "delete reply: %v", out)
	overviewShowsWithin(t, deadline, orphaned, "", "only reply deleted on instance a", a, b)
}

// racingStats is the overview's stats reader with a change landing in the middle of a build: its
// knowledge read runs after the build has read the listed posts, and the first call runs during.
type racingStats struct {
	handlers.OverviewStatsReader
	during func()
}

func (s *racingStats) GetKnowledgeTotals(ctx context.Context) ([]db.KnowledgeTypeTotals, error) {
	if f := s.during; f != nil {
		s.during = nil
		f()
	}
	return s.OverviewStatsReader.GetKnowledgeTotals(ctx)
}

// A snapshot read before a change and finished after the change's notice must not be kept: it
// would serve the removed post for the rest of its 30 s, exactly what the notices prevent.
func TestOverview_ASnapshotBuiltAcrossAChangeIsNotKept(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	pool, err := db.NewPool(ctx, newHomepageScratchURL(t))
	require.NoError(t, err)
	defer pool.Close()
	agentID := "agent_overview_race"
	_, err = pool.Exec(ctx, `INSERT INTO agents (id, display_name, status) VALUES ($1, $1, 'active')`, agentID)
	require.NoError(t, err)
	posts := db.NewPostRepository(pool)
	post, err := posts.Create(ctx, &models.Post{
		Type: models.PostTypeQuestion, Title: "A post deleted while a snapshot is built",
		Description: "A post deleted while a snapshot is built, the description", Tags: []string{"overview"},
		PostedByType: models.AuthorTypeAgent, PostedByID: agentID, Status: models.PostStatusOpen,
	})
	require.NoError(t, err)
	require.NoError(t, posts.UpdateStatus(ctx, post.ID, models.PostStatusOpen))
	_, err = db.NewReplyRepository(pool).Create(ctx, &models.Reply{PostID: post.ID, AuthorType: models.AuthorTypeAgent,
		AuthorID: agentID, Body: "A live reply"})
	require.NoError(t, err)

	stats := &racingStats{OverviewStatsReader: db.NewCanonicalStatsRepository(pool)}
	h := handlers.NewHomepageOverviewHandler(db.NewCanonicalHomepageRepository(pool), db.NewRoomRepository(pool),
		stats, db.NewSearchAnalyticsRepository(pool), nil)
	h.SetOverviewCache(handlers.NewOverviewCache())
	title := func() string {
		t.Helper()
		ts := httptest.NewServer(http.HandlerFunc(h.GetOverviewConsolidated))
		defer ts.Close()
		return overviewPostTitle(t, ts.URL, post.ID)
	}
	stats.during = func() {
		// The build has read the post; it is deleted now and the notice arrives before the build stores.
		_, err := pool.Exec(ctx, `UPDATE posts SET deleted_at = NOW() WHERE id = $1`, post.ID)
		require.NoError(t, err)
		h.InvalidateCache()
	}
	require.Equal(t, post.Title, title(), "the response of the build that read the post before the delete")
	require.Equal(t, "", title(), "the next read is served a snapshot built across the delete")
}
