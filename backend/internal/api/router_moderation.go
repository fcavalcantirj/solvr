package api

import (
	"os"

	"github.com/fcavalcantirj/solvr/internal/api/handlers"
	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/services"
)

// moderationTargets are the handlers anti-abuse W2 extends moderation to.
type moderationTargets struct {
	posts   *handlers.PostsHandler
	blog    *handlers.BlogHandler
	replies *handlers.RepliesHandler
	notify  handlers.ContributionNotifier
}

// wireAntiAbuseModeration extends content moderation past POST /v1/posts (anti-abuse W2):
// replies are moderated after they are created; published blog posts are moderated; the moderator sees the author's recent
// titles. Everything but the recent titles and the post hand-off needs GROQ_API_KEY, the same
// condition POST /v1/posts already has.
func wireAntiAbuseModeration(pool *db.Pool, t moderationTargets) {
	groqAPIKey := os.Getenv("GROQ_API_KEY")
	if pool == nil || groqAPIKey == "" {
		return
	}
	t.posts.SetRecentTitlesReader(db.NewContentDuplicateRepository(pool))
	var opts []services.Option
	if model := os.Getenv("GROQ_MODEL"); model != "" {
		opts = append(opts, services.WithGroqModel(model))
	}
	mod := wrapContentModerator(services.NewContentModerationService(groqAPIKey, opts...))
	contrib := handlers.NewContributionModerator(mod, db.NewContributionModerationRepository(pool), t.notify)
	t.replies.SetContributionModerator(contrib)
	t.blog.SetBlogModeration(db.NewBlogPostRepository(pool), t.notify)
}
