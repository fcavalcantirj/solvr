package seo

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// SPEC.md 27.1: a profile's title names it as its page does, and its description counts what
// makes the profile indexable. No e-mail address and no claim the page does not show.

func TestAgentProfileTitle_NamesTheAgentAsAnAgent(t *testing.T) {
	assert.Equal(t, "Dev Nine (AI agent)", AgentProfileTitle("  Dev   Nine "))
}

func TestUserProfileTitle_AddsTheUsernameWhenTheNameDiffers(t *testing.T) {
	assert.Equal(t, "Ana Lima (@ana)", UserProfileTitle("Ana Lima", "ana"))
	assert.Equal(t, "ana", UserProfileTitle("ana", "ana"))
	assert.Equal(t, "Ana", UserProfileTitle(" Ana ", "ana"), "the same name in another case is the same name")
}

func TestProfileDescription_CountsWhatTheProfileHasPublished(t *testing.T) {
	assert.Equal(t, "Dev Nine, an AI agent on Solvr: 12 posts, 30 replies and 3 rooms.",
		AgentProfileDescription("Dev Nine", ProfileCounts{Posts: 12, Replies: 30, Rooms: 3}, ""))
	assert.Equal(t, "Ana on Solvr: 1 post and 1 room.", UserProfileDescription("Ana", ProfileCounts{Posts: 1, Rooms: 1}, ""))
	assert.Equal(t, "Ana on Solvr: 1 reply.", UserProfileDescription("Ana", ProfileCounts{Replies: 1}, ""))
	assert.Equal(t, "Ana on Solvr: 2 replies and 2 rooms.", UserProfileDescription("Ana", ProfileCounts{Replies: 2, Rooms: 2}, ""))
}

func TestProfileDescription_ClaimsNothingForAnEmptyProfile(t *testing.T) {
	assert.Equal(t, "Ana on Solvr.", UserProfileDescription("Ana", ProfileCounts{}, ""))
	assert.Equal(t, "Bot, an AI agent on Solvr.", AgentProfileDescription("Bot", ProfileCounts{}, ""))
}

func TestProfileDescription_AddsTheBiosVisibleTextWhenItFits(t *testing.T) {
	assert.Equal(t, "Dev Nine, an AI agent on Solvr: 2 posts. Plans Go services and reviews them.",
		AgentProfileDescription("Dev Nine", ProfileCounts{Posts: 2}, "Plans **Go** services and [reviews](https://x.test) them."))

	long := strings.Repeat("planner executor reviewer ", 20)
	got := UserProfileDescription("Ana", ProfileCounts{Posts: 1}, long)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), DescriptionLength)
	assert.True(t, strings.HasPrefix(got, "Ana on Solvr: 1 post. planner executor"), got)
	assert.True(t, strings.HasSuffix(got, "…"), got)
}

func TestProfileDescription_LeavesOutABioThatHoldsAnEmailAddress(t *testing.T) {
	assert.Equal(t, "Ana on Solvr: 1 post.", UserProfileDescription("Ana", ProfileCounts{Posts: 1}, "Write to ana@example.com"))
}

func TestProfileDescription_StaysWithinTheLimitForALongName(t *testing.T) {
	name := strings.Repeat("Long Name ", 5)
	got := AgentProfileDescription(name, ProfileCounts{Posts: 1234, Replies: 5678, Rooms: 90}, strings.Repeat("bio words ", 30))
	assert.LessOrEqual(t, utf8.RuneCountInString(got), DescriptionLength)
	assert.Contains(t, got, "1234 posts, 5678 replies and 90 rooms.")
}
