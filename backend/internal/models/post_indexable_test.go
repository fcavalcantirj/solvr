package models

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Task idx 80: a post page may be indexed exactly when the sitemap would list it
// (db.sitemapPostEligible): publicly eligible and not in a legacy hidden status.
func TestPost_Indexable_MatchesTheSitemapRule(t *testing.T) {
	deleted := time.Now()
	ok := func() Post {
		return Post{
			Status:           PostStatusOpen,
			PublicationState: PublicationPublished,
			ModerationState:  ModerationApproved,
			Visibility:       VisibilityPublic,
		}
	}
	cases := []struct {
		name string
		edit func(p *Post)
		want bool
	}{
		{"published approved public", func(p *Post) {}, true},
		{"empty visibility means public", func(p *Post) { p.Visibility = "" }, true},
		{"legacy status rejected", func(p *Post) { p.Status = PostStatusRejected }, false},
		{"legacy status pending_review", func(p *Post) { p.Status = PostStatusPendingReview }, false},
		{"legacy status draft", func(p *Post) { p.Status = PostStatusDraft }, false},
		{"moderation pending", func(p *Post) { p.ModerationState = ModerationPending }, false},
		{"unpublished draft", func(p *Post) { p.PublicationState = PublicationDraft }, false},
		{"family visibility", func(p *Post) { p.Visibility = VisibilityFamily }, false},
		{"deleted", func(p *Post) { p.DeletedAt = &deleted }, false},
	}
	for _, c := range cases {
		p := ok()
		c.edit(&p)
		assert.Equal(t, c.want, p.Indexable(), c.name)
	}
}
