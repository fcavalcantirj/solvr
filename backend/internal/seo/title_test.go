package seo

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// SPEC.md 27.3, titles: a content page's title states the problem it resolves first, in at
// most TitleLength characters, taken from the page's own text.

func TestSearchTitle_KeepsAShortTitleWhole(t *testing.T) {
	assert.Equal(t, "gateway start blocked", SearchTitle("  gateway  start\nblocked ", TitleLength))
}

func TestSearchTitle_KeepsTheLeadingClausesThatFit(t *testing.T) {
	cases := map[string]string{
		// The headline before the colon, rather than half of its subtitle.
		"How We Bypassed Anthropic's Third-Party OAuth Block: A Billing Proxy Guide for OpenClaw (Humans & Agents)": "How We Bypassed Anthropic's Third-Party OAuth Block",
		// A trailing parenthetical is the first thing to go.
		"What AI provider APIs actually expose for usage monitoring (Claude, ChatGPT/Codex, OpenRouter, Groq)": "What AI provider APIs actually expose for usage monitoring",
		// Clauses joined by a dash or a comma.
		"Plant Wars — FULL master spec + execution ledger (T0.1–T5.4, DoD+Check per task) [living plan]":          "Plant Wars — FULL master spec + execution ledger",
		"Cardputer ADV UIFlow2: stale launcher screen, OFFLINE badge, and completely unresponsive keyboard after": "Cardputer ADV UIFlow2: stale launcher screen, OFFLINE badge",
	}
	for in, want := range cases {
		assert.Equal(t, want, SearchTitle(in, TitleLength), in)
	}
}

func TestSearchTitle_CutsAtAWordWhenNoLongClauseFits(t *testing.T) {
	// The only clause that fits ("gateway start blocked") would drop most of the error string.
	got := SearchTitle("gateway start blocked: existing config is missing gateway.mode", TitleLength)
	assert.Equal(t, "gateway start blocked: existing config is missing…", got)

	got = SearchTitle("claude-agent-sdk: JSON message exceeded maximum buffer size of 1048576 bytes — turn fails, session resets", TitleLength)
	assert.Equal(t, "claude-agent-sdk: JSON message exceeded maximum buffer size…", got)
}

func TestSearchTitle_NeverLeavesABracketOrQuoteOpen(t *testing.T) {
	// The clause before the comma would end inside the quoted error string.
	got := SearchTitle(`Fix "OAuth authentication is currently not supported, use an API key" in OpenClaw`, TitleLength)
	assert.Equal(t, `Fix "OAuth authentication is currently not supported, use…`, got)
}

func TestSearchTitle_AlwaysFitsTheLimit(t *testing.T) {
	for _, in := range []string{
		strings.Repeat("word ", 40),
		strings.Repeat("a", 90),
		"Ação rápida: " + strings.Repeat("ç", 80),
	} {
		got := SearchTitle(in, TitleLength)
		assert.LessOrEqual(t, utf8.RuneCountInString(got), TitleLength, in)
		assert.True(t, utf8.ValidString(got))
	}
}

func TestPostTitle_IsCutToTheTitleLength(t *testing.T) {
	day := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	long := "How We Bypassed Anthropic's Third-Party OAuth Block: A Billing Proxy Guide for OpenClaw (Humans & Agents)"
	assert.Equal(t, "How We Bypassed Anthropic's Third-Party OAuth Block", PostTitle(long, "kestrel", day, 0, 0))
}

// A twin's suffix (its author, and its date) always survives the cut: it is what keeps the
// title unique.
func TestPostTitle_KeepsTheTwinSuffixWithinTheLength(t *testing.T) {
	day := time.Date(2026, 9, 14, 10, 0, 0, 0, time.UTC)
	long := "claude-agent-sdk: JSON message exceeded maximum buffer size of 1048576 bytes"
	got := PostTitle(long, "kestrel", day, 2, 1)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), TitleLength, got)
	assert.True(t, strings.HasSuffix(got, " — kestrel (2026-09-14)"), got)
	assert.True(t, strings.HasPrefix(got, "claude-agent-sdk: JSON message"), got)

	got = PostTitle("Hand a plan over", strings.Repeat("very-long-author-name-", 4), day, 2, 1)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), TitleLength, got)
	assert.True(t, strings.HasPrefix(got, "Hand a plan over — very-long"), got)
	assert.True(t, strings.HasSuffix(got, "(2026-09-14)"), got)
}

func TestRoomTitle_NamesTheRoomThenItsStatedPurpose(t *testing.T) {
	purpose := "Helping other agents to configure new models auth login --provider anthropic --method cli --set-default command"
	got := RoomTitle("fix-openclaw-oauth-cli", &purpose, "")
	assert.Equal(t, "fix-openclaw-oauth-cli: Helping other agents to configure…", got)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), TitleLength)
}

func TestRoomTitle_UsesTheInitialTaskWhenTheRoomStatesNoPurpose(t *testing.T) {
	assert.Equal(t, "gateway-fix: gateway start blocked", RoomTitle("gateway-fix", nil, "gateway start blocked"))
	empty := "  "
	assert.Equal(t, "gateway-fix: Find out why the gateway dies", RoomTitle("gateway-fix", &empty, "**Find out** why the gateway dies"))
}

func TestRoomTitle_IsTheNameAloneWhenNothingElseFits(t *testing.T) {
	assert.Equal(t, "planner-room", RoomTitle("planner-room", nil, ""))
	name := "a room whose name is already most of the title length ok"
	purpose := "and a purpose that cannot fit after it"
	assert.Equal(t, name, RoomTitle(name, &purpose, ""))
	assert.LessOrEqual(t, utf8.RuneCountInString(RoomTitle(strings.Repeat("n", 90), nil, "")), TitleLength)
}

func TestBlogTitle_IsTheSearchTitleOfThePostsTitle(t *testing.T) {
	assert.Equal(t, "What AI provider APIs actually expose for usage monitoring",
		BlogTitle("What AI provider APIs actually expose for usage monitoring (Claude, ChatGPT/Codex, OpenRouter, Groq)"))
}

// Twins are counted on the search title, so two long titles that cut to the same one are told
// apart like two posts that share a title.
func TestCountTitleTwins_ComparesSearchTitlesIgnoringCase(t *testing.T) {
	own := "How We Bypassed Anthropic's Third-Party OAuth Block: A Billing Proxy Guide"
	twins, same := CountTitleTwins(own, []TitleCandidate{
		{Title: "how we bypassed anthropic's third-party oauth block", SameAuthor: true},
		{Title: "How We  Bypassed Anthropic's Third-Party OAuth Block: the sequel"},
		{Title: "How We Bypassed Anthropic's Third-Party OAuth Block Again"},
	})
	assert.Equal(t, 2, twins)
	assert.Equal(t, 1, same)
}

func TestTitleStem_IsWhatEveryTwinStartsWith(t *testing.T) {
	assert.Equal(t, "gateway start blocked: existing config is missing",
		TitleStem("gateway  start blocked: existing config is missing gateway.mode"))
	assert.Equal(t, "Hand a plan over", TitleStem(" Hand a plan over "))
}
