package jobs

import (
	"context"
	"log/slog"
	"time"

	"github.com/fcavalcantirj/solvr/internal/db"
)

// The blog cover job (SEO audit 2026-11-08, Release B1). A blog post published without a
// cover image gets one drawn, but not at once: the author has DefaultBlogCoverGrace to upload
// their own (PUT /v1/blog/{slug}/cover). After that, every run draws at most
// DefaultBlogCoversPerRun covers, one at a time, oldest publication first; a post whose
// drawing failed db.MaxCoverAttempts times is left to the operator. One run at a time across
// instances (the cover lock). The operator's backfill and regenerate routes are not bound by
// the grace or the attempts.

const (
	// DefaultBlogCoverInterval is how often the job runs.
	DefaultBlogCoverInterval = time.Hour
	// DefaultBlogCoverGrace is the author's time to upload a cover before one is drawn.
	DefaultBlogCoverGrace = 24 * time.Hour
	// DefaultBlogCoversPerRun bounds one run: a cover takes about a minute to draw.
	DefaultBlogCoversPerRun = 3
)

// BlogCoverQueue lists the posts that need a cover and counts the failed tries.
type BlogCoverQueue interface {
	TryLock(ctx context.Context) (unlock func(), ok bool, err error)
	ListPublishedWithoutCover(ctx context.Context, grace time.Duration, limit int) ([]db.BlogCoverBrief, error)
	IncrementCoverAttempts(ctx context.Context, blogPostID string) error
}

// CoverDrawer draws and stores one post's cover (handlers.CoverMaker.Make).
type CoverDrawer interface {
	Make(ctx context.Context, brief *db.BlogCoverBrief) (string, error)
}

// BlogCoverJob draws the covers of published posts past the grace period.
type BlogCoverJob struct {
	queue  BlogCoverQueue
	drawer CoverDrawer
	grace  time.Duration
	perRun int
}

// NewBlogCoverJob creates the job.
func NewBlogCoverJob(queue BlogCoverQueue, drawer CoverDrawer, grace time.Duration, perRun int) *BlogCoverJob {
	if grace < 0 {
		grace = DefaultBlogCoverGrace
	}
	if perRun <= 0 {
		perRun = DefaultBlogCoversPerRun
	}
	return &BlogCoverJob{queue: queue, drawer: drawer, grace: grace, perRun: perRun}
}

// BlogCoverRun is what one run did.
type BlogCoverRun struct {
	Drawn  int
	Failed int
	Busy   bool // another run holds the lock
}

// RunOnce draws the covers one run is allowed to.
func (j *BlogCoverJob) RunOnce(ctx context.Context) BlogCoverRun {
	unlock, ok, err := j.queue.TryLock(ctx)
	if err != nil {
		slog.Error("blog covers: lock failed", "error", err)
		return BlogCoverRun{}
	}
	if !ok {
		return BlogCoverRun{Busy: true}
	}
	defer unlock()
	briefs, err := j.queue.ListPublishedWithoutCover(ctx, j.grace, j.perRun)
	if err != nil {
		slog.Error("blog covers: list failed", "error", err)
		return BlogCoverRun{}
	}
	var run BlogCoverRun
	for i := range briefs {
		if ctx.Err() != nil {
			return run
		}
		brief := briefs[i]
		if _, err := j.drawer.Make(ctx, &brief); err != nil {
			run.Failed++
			slog.Error("blog covers: not drawn", "slug", brief.Slug, "error", err)
			if incErr := j.queue.IncrementCoverAttempts(ctx, brief.ID); incErr != nil {
				slog.Error("blog covers: attempt not counted", "slug", brief.Slug, "error", incErr)
			}
			continue
		}
		run.Drawn++
	}
	return run
}

// RunScheduled runs once, then every interval, until ctx ends.
func (j *BlogCoverJob) RunScheduled(ctx context.Context, interval time.Duration) {
	j.runLogged(ctx)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			j.runLogged(ctx)
		}
	}
}

func (j *BlogCoverJob) runLogged(ctx context.Context) {
	run := j.RunOnce(ctx)
	if run.Busy {
		return
	}
	if run.Drawn > 0 || run.Failed > 0 {
		slog.Info("blog covers run", "drawn", run.Drawn, "failed", run.Failed)
	}
}
