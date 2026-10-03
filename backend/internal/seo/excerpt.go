// Package seo derives the search-facing text of public pages (titles and
// descriptions) from the content those pages visibly show. The web client renders
// what this package produces; it never trims or rewrites content itself.
package seo

import (
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// DescriptionLength is the longest description a page is given, in characters.
const DescriptionLength = 160

var (
	imageRe     = regexp.MustCompile(`\s*!\[([^\]]*)\]\([^)]*\)`)
	linkRe      = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)
	fenceRe     = regexp.MustCompile("(?m)^\\s*```[^\\n]*$")
	headingRe   = regexp.MustCompile(`(?m)^\s{0,3}#{1,6}\s*`)
	quoteRe     = regexp.MustCompile(`(?m)^\s{0,3}>\s?`)
	listRe      = regexp.MustCompile(`(?m)^\s*(?:[-*+]|\d+[.)])\s+`)
	emphasisRe  = regexp.MustCompile("(\\*\\*|__|\\*|`+|~~)")
	underlineRe = regexp.MustCompile(`(^|\s)_+|_+($|[\s.,;:!?])`)
)

// Excerpt returns the visible text of a Markdown body as one line of at most max
// characters. Markup is removed (link and image text are kept), whitespace is
// collapsed, and a longer text is cut at a word boundary and ends with "…".
func Excerpt(markdown string, max int) string {
	s := imageRe.ReplaceAllString(markdown, "")
	s = linkRe.ReplaceAllString(s, "$1")
	s = fenceRe.ReplaceAllString(s, "")
	s = headingRe.ReplaceAllString(s, "")
	s = quoteRe.ReplaceAllString(s, "")
	s = listRe.ReplaceAllString(s, "")
	s = emphasisRe.ReplaceAllString(s, "")
	s = underlineRe.ReplaceAllString(s, "$1$2")
	s = strings.Join(strings.Fields(s), " ")
	return truncateWords(s, max)
}

// truncateWords cuts s to at most max characters at a word boundary, ending in "…".
func truncateWords(s string, max int) string {
	if utf8.RuneCountInString(s) <= max {
		return s
	}
	runes := []rune(s)
	cut := string(runes[:max-1])
	if i := strings.LastIndex(cut, " "); i > 0 {
		cut = cut[:i]
	}
	return strings.TrimRight(cut, " ,;:") + "…"
}

// PostDescription is a post page's search description: its body's excerpt, or the
// title when the body has no visible text.
func PostDescription(title, body string) string {
	if d := Excerpt(body, DescriptionLength); d != "" {
		return d
	}
	return Excerpt(title, DescriptionLength)
}

// RoomDescription is a room page's search description, from the room's stated
// purpose: its own description, otherwise the task it was opened with (its first
// message), otherwise a factual line naming the room and its size.
func RoomDescription(displayName string, description *string, initialTask string, messageCount int) string {
	if description != nil {
		if d := Excerpt(*description, DescriptionLength); d != "" {
			return d
		}
	}
	if d := Excerpt(initialTask, DescriptionLength); d != "" {
		return d
	}
	noun := "messages"
	if messageCount == 1 {
		noun = "message"
	}
	return fmt.Sprintf("%s: a public Solvr room with %d %s.", displayName, messageCount, noun)
}

// PostTitle is a post page's unique search title (task idx 82). A title shared with
// other indexable posts (twins) names its author; one the same author also used
// (sameAuthorTwins) also names the post's date, so no two indexable pages share one.
func PostTitle(title, author string, created time.Time, twins, sameAuthorTwins int) string {
	t := strings.Join(strings.Fields(title), " ")
	if twins == 0 {
		return t
	}
	t += " — " + author
	if sameAuthorTwins > 0 {
		t += " (" + created.UTC().Format("2006-01-02") + ")"
	}
	return t
}
