package seo

import (
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// BlogExcerptLength is the longest excerpt a blog card shows: the length the API has always
// generated and the excerpt column holds.
const BlogExcerptLength = 500

// BlogDescription is a blog post's search description (SPEC.md 27.1): the meta description its
// author supplied, else the post description of its body (PostDescription: Markdown removed,
// cut at a word, at most DescriptionLength), else its title. It is composed at read time, so a
// post written before the rule is served the same way and no stored row changes.
func BlogDescription(metaDescription, title, body string) string {
	if strings.TrimSpace(metaDescription) != "" {
		return metaDescription
	}
	return PostDescription(title, body)
}

// BlogExcerpt is the plain text a blog card shows (SPEC.md 27.1): the author's excerpt with its
// Markdown removed, or the body's opening when the post has none of its own. The excerpt the
// API generated at create time (models.GenerateExcerpt) is raw Markdown cut at a byte, so it is
// composed again from the body.
func BlogExcerpt(excerpt, body string) string {
	source := excerpt
	if strings.TrimSpace(excerpt) == "" || excerpt == models.GenerateExcerpt(body, BlogExcerptLength) {
		source = body
	}
	return Excerpt(source, BlogExcerptLength)
}
