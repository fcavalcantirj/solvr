package api

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/covers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/jobs"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The content operations of the 2026-11-08 SEO audit (Release A), kept out of router.go,
// which is near the file-size limit.
//
//	GET    /v1/blog/{slug}/cover.jpg      a blog post's cover (public)
//	PUT    /v1/blog/{slug}/cover          the author's own cover (authenticated, router_v1.go)
//	DELETE /v1/blog/{slug}/cover          the author removes it (authenticated, router_v1.go)
//	POST /admin/posts/{id}/moderation     the operator's approve/reject on a post the automatic
//	                                      moderation left pending (operator-only)
//	POST /admin/blog/{slug}/cover         draw a post's cover now (operator-only)
//	POST /admin/blog/covers/backfill      draw the covers of the published posts without one
//	                                      (operator-only; {"limit","dry_run"})
//
// With OPENAI_API_KEY set, jobs.BlogCoverJob (NewBlogCoverJob, started by main) draws a cover
// 24 h after a post is published without one; without it covers are served but never drawn and
// the admin cover routes answer 503. PUBLIC_API_URL (default https://api.solvr.dev) is the origin the cover
// URL names; OPENAI_IMAGE_MODEL overrides the model.
// contentOps holds the handlers of the content operations, built once (newContentOps) so the
// root router mounts the admin routes and the /v1 router mounts the public cover route and
// hands the blog handler the maker.
type contentOps struct {
	decisions *handlers.PostModerationDecisionHandler
	covers    *handlers.BlogCoverHandler
	maker     *handlers.CoverMaker // nil without OPENAI_API_KEY
}

// newContentOps builds the handlers; nil without a database.
func newContentOps(pool *db.Pool) *contentOps {
	if pool == nil {
		return nil
	}
	postRepo := db.NewPostRepository(pool)
	ops := &contentOps{
		decisions: handlers.NewPostModerationDecisionHandler(&moderatedPosts{posts: postRepo, flags: db.NewContributionModerationRepository(pool)}),
	}
	coverStore := db.NewBlogCoverRepository(pool)
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		publicAPI := publicAPIOrigin()
		ops.maker = handlers.NewCoverMaker(coverStore, coverImageService(key), publicAPI)
		slog.Info("blog covers are drawn 24 h after publication when the author uploaded none")
	} else {
		slog.Info("blog covers are served but not drawn: OPENAI_API_KEY is not set")
	}
	ops.covers = handlers.NewBlogCoverHandler(coverStore, ops.maker)
	ops.covers.SetPostReader(db.NewBlogPostRepository(pool))
	ops.covers.SetPublicAPI(publicAPIOrigin())
	return ops
}

// coverImageService is OpenAI's images API with the key; OPENAI_IMAGE_MODEL picks the model and
// OPENAI_BASE_URL points at another root (the route probes' fake).
func coverImageService(key string) covers.Service {
	opts := []covers.Option{covers.WithModel(os.Getenv("OPENAI_IMAGE_MODEL"))}
	if base := os.Getenv("OPENAI_BASE_URL"); base != "" {
		opts = append(opts, covers.WithBaseURL(base))
	}
	return covers.NewOpenAIService(key, opts...)
}

// publicAPIOrigin is the API's public origin (PUBLIC_API_URL, default https://api.solvr.dev).
func publicAPIOrigin() string {
	if v := os.Getenv("PUBLIC_API_URL"); v != "" {
		return v
	}
	return "https://api.solvr.dev"
}

// NewBlogCoverJob builds the cover job (jobs.BlogCoverJob) for main: nil without a database or
// OPENAI_API_KEY. BLOG_COVER_GRACE (Go duration, default 24h) and BLOG_COVER_PER_RUN (default 3)
// tune it.
func NewBlogCoverJob(pool *db.Pool) *jobs.BlogCoverJob {
	key := os.Getenv("OPENAI_API_KEY")
	if pool == nil || key == "" {
		return nil
	}
	store := db.NewBlogCoverRepository(pool)
	maker := handlers.NewCoverMaker(store, coverImageService(key), publicAPIOrigin())
	grace := jobs.DefaultBlogCoverGrace
	if v := os.Getenv("BLOG_COVER_GRACE"); v != "" {
		if d, err := time.ParseDuration(v); err == nil && d >= 0 {
			grace = d
		}
	}
	perRun := jobs.DefaultBlogCoversPerRun
	if v := os.Getenv("BLOG_COVER_PER_RUN"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			perRun = n
		}
	}
	return jobs.NewBlogCoverJob(store, maker, grace, perRun)
}

// mountContentOpsAdminRoutes registers the operator routes on the root router.
func mountContentOpsAdminRoutes(r chi.Router, ops *contentOps, operatorOnly func(http.Handler) http.Handler) {
	if ops == nil {
		return
	}
	r.With(operatorOnly).Post("/admin/posts/{id}/moderation", ops.decisions.Decide)
	r.With(operatorOnly).Post("/admin/blog/{slug}/cover", ops.covers.Regenerate)
	r.With(operatorOnly).Post("/admin/blog/covers/backfill", ops.covers.Backfill)
}

// moderatedPosts joins the post store and the flag store for the decision route.
type moderatedPosts struct {
	posts *db.PostRepository
	flags *db.ContributionModerationRepository
}

func (m *moderatedPosts) FindByID(ctx context.Context, id string) (*models.PostWithAuthor, error) {
	return m.posts.FindByID(ctx, id)
}

func (m *moderatedPosts) UpdateStatus(ctx context.Context, id string, status models.PostStatus) error {
	return m.posts.UpdateStatus(ctx, id, status)
}

func (m *moderatedPosts) CreateFlag(ctx context.Context, flag *models.Flag) (*models.Flag, error) {
	return m.flags.CreateFlag(ctx, flag)
}
