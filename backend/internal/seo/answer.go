package seo

import (
	"math"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// maxAnswerPart is the most of a description its answer ("Fix: …") takes while the problem
// before it needs the rest; a short problem leaves the answer more.
const maxAnswerPart = 80

// The words that head a body's answer.
const answerWords = `(fix|solution|resolution|answer|workaround|root[ \t]+cause)`

var (
	// answerHeadingRe matches a heading that names the answer ("## The Solution: a proxy").
	answerHeadingRe = regexp.MustCompile(`(?im)^[ \t]{0,3}#{1,6}[ \t]+(?:the[ \t]+)?` + answerWords + `\b([^\n]*)$`)
	// answerLabelRe matches a line led by a label naming the answer ("FIX:", "**Root cause (verified):**").
	answerLabelRe = regexp.MustCompile(`(?im)^[ \t]{0,3}(?:\*\*|__)?(?:the[ \t]+)?` + answerWords +
		`(?:[ \t]*\([^)\n]*\))?[ \t]*(?:\*\*|__)?:(?:\*\*|__)?([^\n]*)$`)
	anyHeadingRe = regexp.MustCompile(`(?m)^[ \t]{0,3}#{1,6}[ \t]`)
)

// visibleText is the whole visible text of a Markdown body, on one line.
func visibleText(markdown string) string {
	return Excerpt(markdown, math.MaxInt32)
}

// describe is the search description of a Markdown body: its problem and answer when it
// names its answer, else its excerpt.
func describe(markdown string) string {
	if d := problemAndAnswer(markdown); d != "" {
		return d
	}
	return Excerpt(markdown, DescriptionLength)
}

// problemAndAnswer describes a body that names its answer (a fix, solution, resolution,
// answer, workaround or root cause heading or label) by the text before it, the problem, then
// the answer under its own label: "<problem> Root cause: <answer>", at most DescriptionLength
// characters, both parts cut at a word. It is "" when the body names no answer, or names one
// with no text.
func problemAndAnswer(body string) string {
	problem, label, answer := splitAnswer(body)
	if answer == "" {
		return ""
	}
	labelled := label + ": " + answer
	if problem == "" {
		return truncateWords(labelled, DescriptionLength)
	}
	problemRoom := min(utf8.RuneCountInString(problem), DescriptionLength-1-maxAnswerPart)
	part := truncateWords(labelled, DescriptionLength-1-problemRoom)
	return truncateWords(problem, DescriptionLength-1-utf8.RuneCountInString(part)) + " " + part
}

// splitAnswer finds a body's first answer heading or label and returns the visible text before
// it, the label (its answer word, capitalized: "Root cause") and the answer's visible text up to
// the next heading.
func splitAnswer(body string) (problem, label, answer string) {
	heading := answerHeadingRe.FindStringSubmatchIndex(body)
	labelled := answerLabelRe.FindStringSubmatchIndex(body)
	m, isHeading := labelled, false
	if heading != nil && (labelled == nil || heading[0] <= labelled[0]) {
		m, isHeading = heading, true
	}
	if m == nil {
		return "", "", ""
	}
	end := len(body)
	if next := anyHeadingRe.FindStringIndex(body[m[1]:]); next != nil {
		end = m[1] + next[0]
	}
	if isHeading {
		answer = joinSentences(headingRemainder(body[m[4]:m[5]]), visibleText(body[m[1]:end]))
	} else {
		answer = visibleText(body[m[4]:end])
	}
	return visibleText(body[:m[0]]), capitalize(body[m[2]:m[3]]), answer
}

// headingRemainder is the visible text after the colon of an answer heading
// ("Solution: a proxy" gives "a proxy"); a heading with no colon gives "".
func headingRemainder(rest string) string {
	i := strings.Index(rest, ":")
	if i < 0 {
		return ""
	}
	return visibleText(rest[i+1:])
}

// joinSentences joins two visible texts as sentences, adding a full stop when the first has none.
func joinSentences(a, b string) string {
	switch {
	case a == "":
		return b
	case b == "":
		return a
	case strings.HasSuffix(a, ".") || strings.HasSuffix(a, "!") || strings.HasSuffix(a, "?"):
		return a + " " + b
	}
	return a + ". " + b
}

// capitalize writes an answer word as a label: "ROOT  CAUSE" gives "Root cause".
func capitalize(word string) string {
	w := []rune(strings.ToLower(strings.Join(strings.Fields(word), " ")))
	w[0] = unicode.ToUpper(w[0])
	return string(w)
}
