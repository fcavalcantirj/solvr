package seo

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/stretchr/testify/assert"
)

// SPEC.md 27.1, blog posts: the description and the excerpt are clean text composed at read
// time, so posts written before the rule are served the same way.

func TestBlogDescription_KeepsTheAuthorsOwnMetaDescription(t *testing.T) {
	assert.Equal(t, "What the post says", BlogDescription("What the post says", "Title", "**Body**"))
}

func TestBlogDescription_ComposesOneFromTheBodyWhenNoneWasSupplied(t *testing.T) {
	body := "# Two agents\n\nThey **plan** and [build](https://x.test) together, then review."
	assert.Equal(t, "Two agents They plan and build together, then review.", BlogDescription("", "Title", body))
	assert.Equal(t, "Two agents They plan and build together, then review.", BlogDescription("   ", "Title", body))

	long := strings.Repeat("planner **executor** review ", 40)
	got := BlogDescription("", "Title", long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), DescriptionLength)
	assert.NotContains(t, got, "*")
	assert.True(t, strings.HasSuffix(got, "…"), got)
}

func TestBlogDescription_FallsBackToTheTitle(t *testing.T) {
	assert.Equal(t, "Only a title", BlogDescription("", "Only a title", "![img](a.png)"))
}

func TestBlogExcerpt_RemovesTheMarkdownOfTheAuthorsExcerpt(t *testing.T) {
	excerpt := "A fixture blog post. It has **two paragraphs** and a [link](https://example.test)."
	assert.Equal(t, "A fixture blog post. It has two paragraphs and a link.", BlogExcerpt(excerpt, "the body"))
}

// The excerpt the API generated at create time is 500 bytes of raw Markdown, cut anywhere: it
// is composed again from the body, cut at a word.
func TestBlogExcerpt_ComposesTheGeneratedOneFromTheBody(t *testing.T) {
	body := strings.Repeat("Ação **rápida** e [link](https://x.test) ", 40)
	stored := models.GenerateExcerpt(body, BlogExcerptLength)
	got := BlogExcerpt(stored, body)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), BlogExcerptLength)
	assert.True(t, utf8.ValidString(got))
	assert.NotContains(t, got, "**")
	assert.NotContains(t, got, "](")
	assert.True(t, strings.HasPrefix(got, "Ação rápida e link Ação"), got)
	assert.True(t, strings.HasSuffix(got, "…"), got)

	assert.Equal(t, "Short body with bold.", BlogExcerpt("", "Short body with **bold**."))
}
