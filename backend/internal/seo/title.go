package seo

import (
	"regexp"
	"strings"
	"time"
	"unicode/utf8"
)

// TitleLength is the longest title a content page (post, room, blog post) is given, in
// characters: what a search result shows before it cuts the title. The page renders it as
// its whole title, with no site name after it (SPEC.md 27.3).
const TitleLength = 60

// minClauseShare is the share of the limit (two thirds) the leading clauses of a long title
// must fill to be kept whole; shorter ones would drop more of the problem than a cut at a word.
const (
	minClauseNumerator   = 2
	minClauseDenominator = 3
)

// minPurposeLength is the fewest characters a room's purpose is given after its name; with
// less room the title is the name alone.
const minPurposeLength = 16

// minTwinTitleLength is the fewest characters of its own title a twin keeps before its author.
const minTwinTitleLength = 20

// clauseBreakRe matches where a title's clause ends: a colon, a spaced dash or bar, a comma,
// a semicolon, a full stop, or an opening parenthesis or bracket.
var clauseBreakRe = regexp.MustCompile(`: | [—–|-] |, |; |\. | [(\[]`)

// SearchTitle is the search title of a page whose problem statement is text: the statement
// whole when it fits in max characters; else its leading clauses when they fill at least two
// thirds of max and close every bracket and quote they open; else the statement cut at a word,
// ending in "…". It never adds a word the statement does not have.
func SearchTitle(text string, max int) string {
	t := strings.Join(strings.Fields(text), " ")
	if utf8.RuneCountInString(t) <= max {
		return t
	}
	if c := leadingClauses(t, max); c != "" {
		return c
	}
	return truncateWords(t, max)
}

// leadingClauses returns the longest run of t's leading clauses that fits in max characters,
// fills at least two thirds of it and leaves no bracket or quote open, or "".
func leadingClauses(t string, max int) string {
	best := ""
	for _, loc := range clauseBreakRe.FindAllStringIndex(t, -1) {
		c := strings.TrimRight(t[:loc[0]], " .,;:")
		n := utf8.RuneCountInString(c)
		if n > max {
			break
		}
		if n*minClauseDenominator >= max*minClauseNumerator && balanced(c) {
			best = c
		}
	}
	return best
}

// balanced reports whether s closes every parenthesis, bracket and quote it opens.
func balanced(s string) bool {
	return strings.Count(s, "(") == strings.Count(s, ")") &&
		strings.Count(s, "[") == strings.Count(s, "]") &&
		strings.Count(s, "“") == strings.Count(s, "”") &&
		strings.Count(s, `"`)%2 == 0 &&
		strings.Count(s, "`")%2 == 0
}

// PostTitle is a post page's unique search title (task idx 82): the search title of the
// post's own title. A title whose search title another indexable post shares (twins) names its
// author; one the same author also used (sameAuthorTwins) also names the post's date, so no two
// indexable pages share one. The author and date always fit: the title is cut to make room.
func PostTitle(title, author string, created time.Time, twins, sameAuthorTwins int) string {
	if twins == 0 {
		return SearchTitle(title, TitleLength)
	}
	const sep = " — "
	date := ""
	if sameAuthorTwins > 0 {
		date = " (" + created.UTC().Format("2006-01-02") + ")"
	}
	author = strings.Join(strings.Fields(author), " ")
	t := strings.Join(strings.Fields(title), " ")
	suffix := utf8.RuneCountInString(sep + author + date)
	base := SearchTitle(t, max(TitleLength-suffix, min(utf8.RuneCountInString(t), minTwinTitleLength)))
	authorRoom := TitleLength - utf8.RuneCountInString(base+sep+date)
	return base + sep + truncateWords(author, authorRoom) + date
}

// RoomTitle is a room page's search title: the room's name (unique among rooms), then its
// stated purpose (its own description, else the task it was opened with), cut to fit. A name
// that leaves too little room is the title alone.
func RoomTitle(name string, description *string, initialTask string) string {
	n := strings.Join(strings.Fields(name), " ")
	purpose := ""
	if description != nil {
		purpose = visibleText(*description)
	}
	if purpose == "" {
		purpose = visibleText(initialTask)
	}
	rest := TitleLength - utf8.RuneCountInString(n) - len(": ")
	if purpose == "" || rest < minPurposeLength {
		return SearchTitle(n, TitleLength)
	}
	return n + ": " + SearchTitle(purpose, rest)
}

// BlogTitle is a blog post page's search title: the search title of the post's title.
func BlogTitle(title string) string {
	return SearchTitle(title, TitleLength)
}

// TitleCandidate is another indexable post's title, and whether it has the same author.
type TitleCandidate struct {
	Title      string
	SameAuthor bool
}

// TitleStem is the text (whitespace collapsed) that every title sharing title's search title
// starts with, ignoring case: the search title without its "…". A repository reads the
// candidate twins of a post by it.
func TitleStem(title string) string {
	return strings.TrimSuffix(SearchTitle(title, TitleLength), "…")
}

// CountTitleTwins counts the candidates whose search title is title's, ignoring case, and how
// many of those have the same author.
func CountTitleTwins(title string, candidates []TitleCandidate) (twins, sameAuthor int) {
	own := strings.ToLower(SearchTitle(title, TitleLength))
	for _, c := range candidates {
		if strings.ToLower(SearchTitle(c.Title, TitleLength)) != own {
			continue
		}
		twins++
		if c.SameAuthor {
			sameAuthor++
		}
	}
	return twins, sameAuthor
}
