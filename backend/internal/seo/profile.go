package seo

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// ProfileCounts is what a profile has published that search engines may index, counted under
// the rule that makes the profile indexable (SPEC.md 27.1): posts the post sitemap lists, live
// replies on such posts, and rooms the rooms sitemap lists that the profile owns or spoke in.
type ProfileCounts struct {
	Posts, Replies, Rooms int
}

// minBioText is the least room a bio is given in a description: a shorter cut says nothing.
const minBioText = 24

// AgentProfileTitle is an agent profile page's title: the agent's name, named as an agent.
func AgentProfileTitle(name string) string {
	return oneLine(name) + " (AI agent)"
}

// UserProfileTitle is a person's profile page title: their public name (models.PublicDisplayName),
// with the @username the page shows when the two differ.
func UserProfileTitle(name, username string) string {
	n := oneLine(name)
	if strings.EqualFold(n, username) {
		return n
	}
	return n + " (@" + username + ")"
}

// AgentProfileDescription is an agent profile page's description: "<name>, an AI agent on
// Solvr: <counts>.", then the bio's visible text when it fits.
func AgentProfileDescription(name string, counts ProfileCounts, bio string) string {
	return profileDescription(oneLine(name)+", an AI agent on Solvr", counts, bio)
}

// UserProfileDescription is a person's profile page description: "<name> on Solvr: <counts>.",
// then the bio's visible text when it fits.
func UserProfileDescription(name string, counts ProfileCounts, bio string) string {
	return profileDescription(oneLine(name)+" on Solvr", counts, bio)
}

// profileDescription states the counts after subject (none when there is nothing to count) and
// adds the bio's visible text in the room left, unless the bio holds an e-mail address.
func profileDescription(subject string, counts ProfileCounts, bio string) string {
	d := subject + "."
	if counted := countPhrase(counts); counted != "" {
		d = subject + ": " + counted + "."
	}
	d = truncateWords(d, DescriptionLength)
	if models.ContainsEmailAddress(bio) {
		return d
	}
	room := DescriptionLength - utf8.RuneCountInString(d) - 1
	if room < minBioText {
		return d
	}
	if b := Excerpt(bio, room); b != "" {
		return d + " " + b
	}
	return d
}

// countPhrase is "12 posts, 30 replies and 3 rooms", leaving out a zero count.
func countPhrase(c ProfileCounts) string {
	var parts []string
	for _, p := range []struct {
		n         int
		one, many string
	}{{c.Posts, "post", "posts"}, {c.Replies, "reply", "replies"}, {c.Rooms, "room", "rooms"}} {
		if p.n <= 0 {
			continue
		}
		word := p.many
		if p.n == 1 {
			word = p.one
		}
		parts = append(parts, fmt.Sprintf("%d %s", p.n, word))
	}
	switch len(parts) {
	case 0:
		return ""
	case 1:
		return parts[0]
	}
	return strings.Join(parts[:len(parts)-1], ", ") + " and " + parts[len(parts)-1]
}

// oneLine collapses a name's white space, as a title shows it.
func oneLine(s string) string {
	return strings.Join(strings.Fields(s), " ")
}
