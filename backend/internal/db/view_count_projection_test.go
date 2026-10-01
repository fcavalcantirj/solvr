package db

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/require"
)

// A post's view_count is a projection of its post_views rows (idx 77, migration 000120).
// Before it, RecordView inserted the view row and then incremented the counter in a second
// statement outside any transaction: a caller that left right after the insert stored the
// view but never counted it, and every retry by the same viewer hit the unique key and
// counted nothing again (measured on HEAD, idx 77 slice 4). These tests run on scratch
// databases so a drift check or rebuild of every post sees only rows they created.

type viewCount struct{ stored, rows int }

func readViewCount(ctx context.Context, t *testing.T, pool *Pool, postID string) viewCount {
	t.Helper()
	var c viewCount
	require.NoError(t, pool.QueryRow(ctx, `
		SELECT view_count, (SELECT COUNT(*) FROM post_views WHERE post_id = $1)
		FROM posts WHERE id = $1`, postID).Scan(&c.stored, &c.rows))
	return c
}

type viewCountDrift struct {
	postID       string
	stored, rows int
}

func readViewCountDrift(ctx context.Context, t *testing.T, pool *Pool, postID *string) []viewCountDrift {
	t.Helper()
	rows, err := pool.Query(ctx, `SELECT post_id::text, stored_view_count, view_rows FROM view_count_drift($1::uuid)`, postID)
	require.NoError(t, err)
	defer rows.Close()
	var out []viewCountDrift
	for rows.Next() {
		var d viewCountDrift
		require.NoError(t, rows.Scan(&d.postID, &d.stored, &d.rows))
		out = append(out, d)
	}
	require.NoError(t, rows.Err())
	return out
}

func rebuildViewCounts(ctx context.Context, pool *Pool, postID *string) (int, error) {
	var n int
	err := pool.QueryRow(ctx, `SELECT rebuild_view_counts($1::uuid)`, postID).Scan(&n)
	return n, err
}

// viewTarget creates a post to view.
func viewTarget(ctx context.Context, t *testing.T, pool *Pool, label string) string {
	t.Helper()
	post, err := NewPostRepository(pool).Create(ctx, &models.Post{
		Type: models.PostTypePost, Title: "View count target " + label,
		Description:  "A post whose view count is derived from its view rows (" + label + ")",
		PostedByType: models.AuthorTypeAgent, PostedByID: authorAgent(ctx, t, pool, "view_author"), Status: models.PostStatusOpen,
	})
	require.NoError(t, err)
	return post.ID
}

func TestViewCounts_EveryViewRowMovesTheCountInItsOwnTransaction(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	postID, otherID := viewTarget(ctx, t, pool, "events"), viewTarget(ctx, t, pool, "other")
	views := NewViewsRepository(pool)
	record := func(viewerType, viewerID string) int {
		t.Helper()
		n, err := views.RecordView(ctx, postID, viewerType, viewerID)
		require.NoError(t, err)
		return n
	}
	exec := func(sql string, args ...any) {
		t.Helper()
		_, err := pool.Exec(ctx, sql, args...)
		require.NoError(t, err)
	}

	require.Equal(t, 1, record("agent", "alice"))
	require.Equal(t, 1, record("agent", "alice"), "a repeated view by one viewer counts once")
	require.Equal(t, 2, record("human", "alice"), "another viewer type is another viewer")
	require.Equal(t, 3, record("anonymous", "session-1"))
	require.Equal(t, viewCount{3, 3}, readViewCount(ctx, t, pool, postID))

	// Whoever writes the view row keeps the count: a fixture, an operator, a cleanup.
	exec(`INSERT INTO post_views (post_id, viewer_type, viewer_id) VALUES ($1, 'agent', 'bob')`, postID)
	require.Equal(t, viewCount{4, 4}, readViewCount(ctx, t, pool, postID), "an inserted view row is counted")
	exec(`DELETE FROM post_views WHERE post_id = $1 AND viewer_id IN ('bob', 'session-1')`, postID)
	require.Equal(t, viewCount{2, 2}, readViewCount(ctx, t, pool, postID), "a deleted view row is uncounted")
	exec(`UPDATE post_views SET post_id = $1 WHERE post_id = $2 AND viewer_type = 'human'`, otherID, postID)
	require.Equal(t, viewCount{1, 1}, readViewCount(ctx, t, pool, postID), "a view moved away is uncounted here")
	require.Equal(t, viewCount{1, 1}, readViewCount(ctx, t, pool, otherID), "and counted where it moved")
	exec(`UPDATE post_views SET viewed_at = viewed_at - interval '1 day' WHERE post_id = $1`, postID)
	require.Equal(t, viewCount{1, 1}, readViewCount(ctx, t, pool, postID), "an update that keeps the post moves nothing")

	// A view whose transaction rolls back never counted.
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	_, err = tx.Exec(ctx, `INSERT INTO post_views (post_id, viewer_type, viewer_id) VALUES ($1, 'agent', 'erin')`, postID)
	require.NoError(t, err)
	require.NoError(t, tx.Rollback(ctx))
	require.Equal(t, viewCount{1, 1}, readViewCount(ctx, t, pool, postID), "a rolled-back view is not counted")

	n, err := views.GetViewCount(ctx, postID)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Empty(t, readViewCountDrift(ctx, t, pool, nil))
}

type viewCancelRequestKey struct{}

type viewInsertMark struct{}

// cancelAfterViewInsert ends a call's context as soon as its post_views INSERT returns:
// the caller is gone right after the view is stored, as when a client disconnects or its
// deadline fires mid-request.
type cancelAfterViewInsert struct{ fired atomic.Int32 }

func (c *cancelAfterViewInsert) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "INSERT INTO post_views") {
		return context.WithValue(ctx, viewInsertMark{}, true)
	}
	return ctx
}

func (c *cancelAfterViewInsert) TraceQueryEnd(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryEndData) {
	if ctx.Value(viewInsertMark{}) == nil || data.Err != nil {
		return
	}
	if cancel, ok := ctx.Value(viewCancelRequestKey{}).(context.CancelFunc); ok {
		c.fired.Add(1)
		cancel()
	}
}

// Measured on HEAD before 000120: the stored view stayed uncounted (view_count 0, one view
// row) and the same viewer's retry answered 0.
func TestViewCounts_ACallerThatLeavesRightAfterTheInsertIsStillCounted(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	postID := viewTarget(ctx, t, pool, "cancelled")
	tracer := &cancelAfterViewInsert{}
	traced, err := NewPool(ctx, pool.Config().ConnString(), WithQueryTracer(tracer))
	require.NoError(t, err)
	defer traced.Close()

	callCtx, callCancel := context.WithCancel(ctx)
	defer callCancel()
	_, err = NewViewsRepository(traced).RecordView(context.WithValue(callCtx, viewCancelRequestKey{}, callCancel), postID, "agent", "leaver")
	t.Logf("the call whose caller left after the insert returned err=%v", err)
	require.Equal(t, int32(1), tracer.fired.Load(), "the caller left right after the insert")
	require.Equal(t, viewCount{1, 1}, readViewCount(ctx, t, pool, postID), "the stored view is counted")

	n, err := NewViewsRepository(pool).RecordView(ctx, postID, "agent", "leaver")
	require.NoError(t, err)
	require.Equal(t, 1, n, "the viewer's retry reads the count that includes its view")
}

func TestViewCounts_ConcurrentViewsCountEachViewerOnce(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	postID := viewTarget(ctx, t, pool, "concurrent")
	views := NewViewsRepository(pool)

	var wg sync.WaitGroup
	errs := make(chan error, 60)
	for i := range 30 {
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err := views.RecordView(ctx, postID, "agent", fmt.Sprintf("viewer_%d", i))
			errs <- err
		}()
		go func() {
			defer wg.Done()
			_, err := views.RecordView(ctx, postID, "anonymous", "one-session")
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}
	require.Equal(t, viewCount{31, 31}, readViewCount(ctx, t, pool, postID), "30 distinct viewers plus one repeated viewer")
}

func TestViewCounts_DriftNamesEveryBrokenCountAndTheRebuildRepairsIt(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	healthy, low, high := viewTarget(ctx, t, pool, "healthy"), viewTarget(ctx, t, pool, "low"), viewTarget(ctx, t, pool, "high")
	views := NewViewsRepository(pool)
	for _, id := range []string{healthy, low, high} {
		for _, v := range []string{"v1", "v2"} {
			_, err := views.RecordView(ctx, id, "agent", v)
			require.NoError(t, err)
		}
	}
	// Stored values the view rows do not support: a lost increment (the measured failure)
	// and a counter written by hand.
	_, err := pool.Exec(ctx, `UPDATE posts SET view_count = 1 WHERE id = $1`, low)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE posts SET view_count = 40 WHERE id = $1`, high)
	require.NoError(t, err)
	xmin := func(id string) string {
		var x string
		require.NoError(t, pool.QueryRow(ctx, `SELECT xmin::text FROM posts WHERE id = $1`, id).Scan(&x))
		return x
	}
	healthyXmin := xmin(healthy)

	drift := readViewCountDrift(ctx, t, pool, nil)
	require.ElementsMatch(t, []viewCountDrift{{low, 1, 2}, {high, 40, 2}}, drift, "exactly the two broken posts")
	require.Equal(t, []viewCountDrift{{low, 1, 2}}, readViewCountDrift(ctx, t, pool, &low), "filtered by post")
	require.Empty(t, readViewCountDrift(ctx, t, pool, &healthy))

	n, err := rebuildViewCounts(ctx, pool, &low)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, viewCount{2, 2}, readViewCount(ctx, t, pool, low))
	require.Equal(t, []viewCountDrift{{high, 40, 2}}, readViewCountDrift(ctx, t, pool, nil), "a one-post rebuild leaves the other broken post alone")

	n, err = rebuildViewCounts(ctx, pool, nil)
	require.NoError(t, err)
	require.Equal(t, 1, n)
	require.Equal(t, viewCount{2, 2}, readViewCount(ctx, t, pool, high))
	require.Empty(t, readViewCountDrift(ctx, t, pool, nil))
	require.Equal(t, healthyXmin, xmin(healthy), "a consistent post is not rewritten")

	n, err = rebuildViewCounts(ctx, pool, nil)
	require.NoError(t, err)
	require.Equal(t, 0, n, "replaying the rebuild changes nothing")
}

// A rebuild and a view that meet must not lose the view, whichever holds the post first.
func TestViewCounts_ARebuildAndAViewNeverLoseEachOther(t *testing.T) {
	pool, _ := newMigratedScratchDatabase(t)
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	postID := viewTarget(ctx, t, pool, "inflight")
	views := NewViewsRepository(pool)
	for i := range 3 {
		_, err := views.RecordView(ctx, postID, "agent", fmt.Sprintf("committed_%d", i))
		require.NoError(t, err)
	}
	breakCount := func() {
		_, err := pool.Exec(ctx, `UPDATE posts SET view_count = 50 WHERE id = $1`, postID)
		require.NoError(t, err)
	}

	// 1. The view is in flight: the rebuild waits for it, then counts it.
	breakCount()
	tx, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx.Rollback(context.Background()) //nolint:errcheck
	_, err = tx.Exec(ctx, `INSERT INTO post_views (post_id, viewer_type, viewer_id) VALUES ($1, 'agent', 'in_flight')`, postID)
	require.NoError(t, err)
	done := make(chan error, 1)
	go func() { _, err := rebuildViewCounts(ctx, pool, &postID); done <- err }()
	waitForLockWait(ctx, t, pool, "%rebuild_view_counts%", done)
	require.NoError(t, tx.Commit(ctx))
	require.NoError(t, <-done)
	require.Equal(t, viewCount{4, 4}, readViewCount(ctx, t, pool, postID), "the in-flight view is counted")

	// 2. The rebuild is in flight: the view waits for it, then lands on the rebuilt count.
	breakCount()
	tx2, err := pool.BeginTx(ctx)
	require.NoError(t, err)
	defer tx2.Rollback(context.Background()) //nolint:errcheck
	var repaired int
	require.NoError(t, tx2.QueryRow(ctx, `SELECT rebuild_view_counts($1::uuid)`, postID).Scan(&repaired))
	require.Equal(t, 1, repaired)
	viewed := make(chan error, 1)
	go func() { _, err := views.RecordView(ctx, postID, "agent", "late_viewer"); viewed <- err }()
	waitForLockWait(ctx, t, pool, "%INSERT INTO post_views%", viewed)
	require.NoError(t, tx2.Commit(ctx))
	require.NoError(t, <-viewed)
	require.Equal(t, viewCount{5, 5}, readViewCount(ctx, t, pool, postID), "the late view lands on the rebuilt count")
}
