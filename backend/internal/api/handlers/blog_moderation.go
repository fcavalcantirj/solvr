package handlers

import (
	"context"
	"fmt"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BlogUnpublisher sets a rejected blog post back to draft (db.BlogPostRepository).
type BlogUnpublisher interface {
	UnpublishBySlug(ctx context.Context, slug string) error
}

// SetBlogModeration makes published blog posts go through content moderation (anti-abuse W2,
// D5b): a rejected post returns to draft and its author is notified. The post is visible until
// the verdict arrives.
func (h *BlogHandler) SetBlogModeration(unpublish BlogUnpublisher, notify ContributionNotifier) {
	h.unpublisher, h.notify = unpublish, notify
}

// moderatePublished moderates a blog post that is published after a create or an edit.
func (h *BlogHandler) moderatePublished(post *models.BlogPost) {
	if h.contentModService == nil || h.unpublisher == nil || post == nil || post.Status != models.BlogPostStatusPublished {
		return
	}
	p := *post
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()
		result, err := h.contentModService.ModerateContent(ctx, ModerationInput{Title: p.Title, Description: p.Body, Tags: p.Tags})
		if err != nil {
			h.logger.Error("blog moderation failed; left published", "slug", p.Slug, "error", err)
			return
		}
		if result.Approved {
			return
		}
		if err := h.unpublisher.UnpublishBySlug(ctx, p.Slug); err != nil {
			h.logger.Error("blog moderation: unpublish failed", "slug", p.Slug, "error", err)
			return
		}
		if h.notify == nil {
			return
		}
		// Under the event contract with no subject: a blog post is not a post or a reply.
		n := &models.Notification{Type: models.NotificationBlogPostRejected, Title: "Blog post needs changes",
			Body: fmt.Sprintf("Your blog post %q was returned to draft: %s. Edit and publish again.", p.Title, result.Explanation),
			Link: "/blog/" + p.Slug, SchemaVersion: models.NotificationSchemaVersion}
		authorID := p.PostedByID
		if p.PostedByType == models.AuthorTypeHuman {
			n.UserID = &authorID
		} else {
			n.AgentID = &authorID
		}
		if _, err := h.notify(ctx, n); err != nil {
			h.logger.Error("blog moderation: notify failed", "slug", p.Slug, "error", err)
		}
	}()
}
