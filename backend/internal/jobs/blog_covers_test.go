package jobs

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The cover job (Release B1): past the grace period, at most perRun covers a run, one at a
// time; a failure counts an attempt and the run goes on; a run that finds the lock taken does
// nothing.

type fakeCoverQueue struct {
	attempts map[string]int
	briefs   []db.BlogCoverBrief
	grace    time.Duration
	limit    int
	busy     bool
}

func (f *fakeCoverQueue) TryLock(context.Context) (unlock func(), ok bool, err error) {
	if f.busy {
		return nil, false, nil
	}
	return func() {}, true, nil
}

func (f *fakeCoverQueue) ListPublishedWithoutCover(_ context.Context, grace time.Duration, limit int) ([]db.BlogCoverBrief, error) {
	f.grace, f.limit = grace, limit
	if len(f.briefs) > limit {
		return f.briefs[:limit], nil
	}
	return f.briefs, nil
}

func (f *fakeCoverQueue) IncrementCoverAttempts(_ context.Context, id string) error {
	if f.attempts == nil {
		f.attempts = map[string]int{}
	}
	f.attempts[id]++
	return nil
}

type fakeDrawer struct {
	fail  map[string]bool
	drawn []string
}

func (f *fakeDrawer) Make(_ context.Context, b *db.BlogCoverBrief) (string, error) {
	if f.fail[b.Slug] {
		return "", errors.New("model refused")
	}
	f.drawn = append(f.drawn, b.Slug)
	return "https://api.solvr.dev/v1/blog/" + b.Slug + "/cover.jpg", nil
}

func TestBlogCoverJob_DrawsPastTheGraceUpToThePerRunCap(t *testing.T) {
	queue := &fakeCoverQueue{briefs: []db.BlogCoverBrief{{ID: "1", Slug: "one"}, {ID: "2", Slug: "two"}, {ID: "3", Slug: "three"}, {ID: "4", Slug: "four"}}}
	drawer := &fakeDrawer{fail: map[string]bool{"two": true}}
	job := NewBlogCoverJob(queue, drawer, 24*time.Hour, 3)

	run := job.RunOnce(context.Background())

	assert.Equal(t, 24*time.Hour, queue.grace, "the author's grace is passed to the list")
	assert.Equal(t, 3, queue.limit)
	assert.Equal(t, BlogCoverRun{Drawn: 2, Failed: 1}, run)
	assert.Equal(t, []string{"one", "three"}, drawer.drawn)
	assert.Equal(t, map[string]int{"2": 1}, queue.attempts, "a failure counts one attempt, the run goes on")
}

func TestBlogCoverJob_DoesNothingWhileAnotherRunHoldsTheLock(t *testing.T) {
	queue := &fakeCoverQueue{busy: true, briefs: []db.BlogCoverBrief{{ID: "1", Slug: "one"}}}
	drawer := &fakeDrawer{}
	run := NewBlogCoverJob(queue, drawer, 0, 0).RunOnce(context.Background())
	assert.True(t, run.Busy)
	assert.Empty(t, drawer.drawn)
}

func TestBlogCoverJob_Defaults(t *testing.T) {
	job := NewBlogCoverJob(&fakeCoverQueue{}, &fakeDrawer{}, -1, 0)
	require.Equal(t, DefaultBlogCoverGrace, job.grace)
	require.Equal(t, DefaultBlogCoversPerRun, job.perRun)
}
