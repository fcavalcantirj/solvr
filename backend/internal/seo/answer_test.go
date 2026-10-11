package seo

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

// SPEC.md 27.3, descriptions: a body that names its answer (a fix, solution, workaround,
// resolution, answer or root cause section) is described by its problem and that answer.

func TestPostDescription_StatesTheProblemThenTheRootCause(t *testing.T) {
	body := "Every turn fails with JSON message exceeded maximum buffer size of 1048576 bytes.\n\n" +
		"**Root cause (verified in SDK source):** the SDK stdout reader caps a line at 1 MB.\n\n" +
		"## Steps\n\n1. Raise the cap."
	assert.Equal(t,
		"Every turn fails with JSON message exceeded maximum buffer size of 1048576 bytes. Root cause: the SDK stdout reader caps a line at 1 MB.",
		PostDescription("title", body))
}

func TestPostDescription_TakesTheAnswerFromAHeadingAndItsSection(t *testing.T) {
	body := "Third-party tools started getting rejected with HTTP 401.\n\n" +
		"## The Solution: openclaw-billing-proxy\n\nA proxy that makes requests look like Claude Code.\n\n## Setup\n\nClone it."
	assert.Equal(t,
		"Third-party tools started getting rejected with HTTP 401. Solution: openclaw-billing-proxy. A proxy that makes requests look like Claude Code.",
		PostDescription("title", body))
}

func TestPostDescription_ShortensBothPartsToFit(t *testing.T) {
	problem := strings.Repeat("the gateway dies on start ", 12)
	fix := strings.Repeat("set gateway.mode to local ", 12)
	got := PostDescription("title", problem+"\n\nFIX: "+fix)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), DescriptionLength, got)
	assert.True(t, strings.HasPrefix(got, "the gateway dies on start"), got)
	assert.Contains(t, got, "… Fix: set gateway.mode to local")
	assert.True(t, strings.HasSuffix(got, "…"), got)
}

func TestPostDescription_AnAnswerWithNoProblemBeforeItStandsAlone(t *testing.T) {
	assert.Equal(t, "Workaround: Batch posts while the key is valid.",
		PostDescription("title", "## Workaround\n\nBatch posts while the key is valid."))
}

func TestPostDescription_ASentenceStartingWithFixIsNotAnAnswerSection(t *testing.T) {
	body := "The gateway dies.\n\nFix the gateway by restarting it twice."
	assert.Equal(t, "The gateway dies. Fix the gateway by restarting it twice.", PostDescription("title", body))
}

func TestPostDescription_AnEmptyAnswerSectionIsIgnored(t *testing.T) {
	assert.Equal(t, "The gateway dies. Solution", PostDescription("title", "The gateway dies.\n\n## Solution\n"))
}

func TestRoomDescription_TheInitialTaskStatesItsFix(t *testing.T) {
	task := "THE PROBLEM: models auth login rewrites model refs to claude-cli refs. Gateway dead.\n\n" +
		"FIX:\n1. Save a backup first.\n2. Replace claude-cli/ with anthropic/."
	assert.Equal(t,
		"THE PROBLEM: models auth login rewrites model refs to claude-cli refs. Gateway dead. Fix: Save a backup first. Replace claude-cli/ with anthropic/.",
		RoomDescription("room", nil, task, 3))
}

func TestBlogDescription_CleansAndCapsTheAuthorsOwnMetaDescription(t *testing.T) {
	assert.Equal(t, "What the post says", BlogDescription("What the **post** says", "Title", "Body"))
	got := BlogDescription(strings.Repeat("usage limits ", 30), "Title", "Body")
	assert.LessOrEqual(t, utf8.RuneCountInString(got), DescriptionLength)
	assert.True(t, strings.HasSuffix(got, "…"), got)
}
