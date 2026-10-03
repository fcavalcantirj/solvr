package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// cleanInstallIPFS stands in for the IPFS node: it keeps what the crystallizer adds.
type cleanInstallIPFS struct{ added []byte }

func (f *cleanInstallIPFS) Add(_ context.Context, r io.Reader) (string, error) {
	data, err := io.ReadAll(r)
	f.added = data
	return "bafycleaninstall", err
}

func (f *cleanInstallIPFS) Pin(context.Context, string) error { return nil }

// Task idx 68 step 6: a clean installation — every migration applied to an empty database,
// the legacy archive included — creates, searches, moderates, exports and deletes a post and a
// reply with every legacy table absent from live storage. The export is the canonical
// crystallization snapshot (PostSnapshot v2.0), and it carries the reply.
func TestCleanInstall_PostAndReplyLifecycleWithoutLegacyTables(t *testing.T) {
	mod := useRecordingModerator(t)
	ts, pool := serveConsumerSchema(t, newConsumerScratchSchema(t, len(consumerSchemaFiles(t)), nil))
	ctx := context.Background()
	call := func(method, path, bearer, body string) statusContractAnswer {
		t.Helper()
		got, err := callStatusContract(http.DefaultClient, method, ts.URL+path, bearer, body)
		require.NoError(t, err, "%s %s", method, path)
		return got
	}
	dataID := func(got statusContractAnswer) string {
		t.Helper()
		var env struct {
			Data struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		require.NoError(t, json.Unmarshal([]byte(got.body), &env), got.body)
		require.NotEmpty(t, env.Data.ID, got.body)
		return env.Data.ID
	}

	// Every legacy table is out of live storage, and a clean install archived nothing.
	for _, table := range db.LegacyTables {
		var live bool
		require.NoError(t, pool.QueryRow(ctx, `SELECT to_regclass('public.' || $1) IS NOT NULL`, table).Scan(&live))
		require.False(t, live, "%s is not live storage", table)
	}
	var archivedRows, manifestRows int
	require.NoError(t, pool.QueryRow(ctx, `SELECT COALESCE(sum(row_count), 0), count(*) FROM legacy_archive.manifest`).
		Scan(&archivedRows, &manifestRows))
	require.Equal(t, 10, manifestRows, "the six tables, post_fields and the three target tables")
	require.Zero(t, archivedRows, "a clean install has nothing to archive")

	// Create: an agent writes a post; moderation approves it.
	_, key := registerConsumerAgent(t, ts.URL, "clean_install_"+uuid.NewString()[:8])
	marker := "cleaninstall" + uuid.NewString()[:8]
	mod.QueueResults(approved())
	created := call(http.MethodPost, "/v1/posts", key, fmt.Sprintf(
		`{"title":"Worker pool drains slowly %s","description":"Workers drain slowly under load in %s; this post says what we measured and what we tried.","tags":["%s"]}`,
		marker, marker, marker))
	require.Equal(t, http.StatusCreated, created.status, created.body)
	postID := dataID(created)
	waitForValue(t, pool, "open", `SELECT status FROM posts WHERE id = $1::uuid`, postID)
	waitForValue(t, pool, "post", `SELECT type FROM posts WHERE id = $1::uuid`, postID)

	// ... and a reply on it.
	replied := call(http.MethodPost, "/v1/posts/"+postID+"/replies", key,
		fmt.Sprintf(`{"body":"Raising the pool size fixed the %s drain: measured before and after."}`, marker))
	require.Equal(t, http.StatusCreated, replied.status, replied.body)
	replyID := dataID(replied)
	// The reply is moderated asynchronously too (approved: nothing queued); let that verdict
	// land before queueing the next one.
	require.Eventually(t, func() bool { return mod.GetCalls() >= 2 }, waitTimeout, waitTick, "the reply is moderated")

	// Moderate: a second post is rejected, with a verdict reply.
	mod.QueueResults(rejected("advertising"))
	bad := call(http.MethodPost, "/v1/posts", key, fmt.Sprintf(
		`{"title":"Buy cheap followers %s","description":"Cheap followers for your repository, %s, fast delivery and a money-back guarantee."}`,
		marker, marker))
	require.Equal(t, http.StatusCreated, bad.status, bad.body)
	badID := dataID(bad)
	waitForValue(t, pool, "rejected", `SELECT status FROM posts WHERE id = $1::uuid`, badID)
	waitForValue(t, pool, "1", `SELECT count(*)::text FROM replies WHERE post_id = $1::uuid AND author_type = 'system'`, badID)

	// Search: the post is found with the reply anchored in it; the rejected post is not.
	searched := call(http.MethodGet, "/v1/search?q="+url.QueryEscape(marker+" drain"), "", "")
	require.Equal(t, http.StatusOK, searched.status, searched.body)
	var search struct {
		Data []struct {
			ID             string `json:"id"`
			Type           string `json:"type"`
			MatchedReplies []struct {
				ID string `json:"id"`
			} `json:"matched_replies"`
		} `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(searched.body), &search), searched.body)
	var foundPost, foundReply bool
	for _, r := range search.Data {
		require.NotEqual(t, badID, r.ID, "a rejected post is not searchable")
		if r.ID == postID {
			foundPost = true
			require.Equal(t, "post", r.Type)
			for _, m := range r.MatchedReplies {
				foundReply = foundReply || m.ID == replyID
			}
		}
	}
	require.True(t, foundPost, "search finds the post: %s", searched.body)
	require.True(t, foundReply, "search anchors the reply in its post: %s", searched.body)

	// Export: the crystallization snapshot of the post carries the reply.
	ipfs := &cleanInstallIPFS{}
	cfg := services.DefaultCrystallizationConfig()
	cfg.StabilityPeriod = 0
	crystallizer := services.NewPostCrystallizationServiceWithConfig(db.NewPostCrystallizationRepository(pool),
		db.NewPostRepository(pool), db.NewReplyRepository(pool), ipfs, ipfs, cfg)
	cid, err := crystallizer.CrystallizePost(ctx, postID)
	require.NoError(t, err)
	require.Equal(t, "bafycleaninstall", cid)
	var snapshot services.PostSnapshot
	require.NoError(t, json.NewDecoder(bytes.NewReader(ipfs.added)).Decode(&snapshot))
	require.Equal(t, "2.0", snapshot.Version)
	require.Equal(t, postID, snapshot.PostID)
	require.Len(t, snapshot.Replies, 1)
	require.Equal(t, replyID, snapshot.Replies[0].ID)
	waitForValue(t, pool, cid, `SELECT COALESCE(crystallization_cid, '') FROM posts WHERE id = $1::uuid`, postID)

	// Delete: the reply, then the post; both leave reads and search.
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent},
		call(http.MethodDelete, "/v1/replies/"+replyID, key, "").status, "delete the reply")
	require.Equal(t, http.StatusNotFound, call(http.MethodGet, "/v1/replies/"+replyID, "", "").status)
	require.Contains(t, []int{http.StatusOK, http.StatusNoContent},
		call(http.MethodDelete, "/v1/posts/"+postID, key, "").status, "delete the post")
	require.Equal(t, http.StatusNotFound, call(http.MethodGet, "/v1/posts/"+postID, "", "").status)
	searched = call(http.MethodGet, "/v1/search?q="+url.QueryEscape(marker+" drain"), "", "")
	require.Equal(t, http.StatusOK, searched.status, searched.body)
	require.NotContains(t, searched.body, postID, "a deleted post is not searchable")
	require.NotContains(t, searched.body, replyID, "a deleted reply is not searchable")
}
