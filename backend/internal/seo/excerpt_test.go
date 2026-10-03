package seo

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/stretchr/testify/assert"
)

func TestExcerpt_StripsMarkdownToVisibleText(t *testing.T) {
	in := "# Fixing the build\n\nSome **bold** and _soft_ text with `code` and a [link](https://example.com/x) ![shot](a.png).\n\n> quoted line\n- item one\n1. item two"
	assert.Equal(t, "Fixing the build Some bold and soft text with code and a link. quoted line item one item two",
		Excerpt(in, 160))
}

func TestExcerpt_KeepsCodeFenceContentWithoutTheFences(t *testing.T) {
	assert.Equal(t, "Run this: go test ./...", Excerpt("Run this:\n```bash\ngo test ./...\n```", 160))
}

func TestExcerpt_CutsAtAWordBoundaryWithinTheLimit(t *testing.T) {
	in := strings.Repeat("planner executor review ", 20)
	got := Excerpt(in, 60)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), 60)
	assert.True(t, strings.HasSuffix(got, "…"), "a cut excerpt ends with an ellipsis: %q", got)
	body := strings.TrimSuffix(got, "…")
	assert.True(t, strings.HasPrefix(in, body), "the cut keeps whole words from the start: %q", got)
	assert.False(t, strings.HasSuffix(body, " "), "no trailing space before the ellipsis: %q", got)
	next := in[len(body):]
	assert.True(t, strings.HasPrefix(next, " "), "the cut falls between words: %q", got)
}

func TestExcerpt_ShortTextIsReturnedWhole(t *testing.T) {
	assert.Equal(t, "Two agents, one room.", Excerpt("  Two   agents,\n one room.  ", 160))
}

func TestExcerpt_EmptyAndMarkupOnlyGiveEmpty(t *testing.T) {
	assert.Equal(t, "", Excerpt("", 160))
	assert.Equal(t, "", Excerpt("   \n\n# \n", 160))
}

func TestExcerpt_CountsRunesNotBytes(t *testing.T) {
	in := strings.Repeat("ação ", 50)
	got := Excerpt(in, 40)
	assert.LessOrEqual(t, utf8.RuneCountInString(got), 40)
	assert.True(t, utf8.ValidString(got))
}

func TestRoomDescription_PrefersTheRoomsOwnPurpose(t *testing.T) {
	d := "Plan and ship the **garden** kestrel firmware"
	assert.Equal(t, "Plan and ship the garden kestrel firmware",
		RoomDescription("kestrel-room", &d, "first message", 12))
}

func TestRoomDescription_FallsBackToTheInitialTask(t *testing.T) {
	empty := "  "
	assert.Equal(t, "Build a tic-tac-toe game in Go and review it",
		RoomDescription("ttt", &empty, "Build a tic-tac-toe game in **Go** and review it", 4))
	assert.Equal(t, "Build a tic-tac-toe game in Go and review it",
		RoomDescription("ttt", nil, "Build a tic-tac-toe game in Go and review it", 4))
}

func TestRoomDescription_LastResortIsFactual(t *testing.T) {
	assert.Equal(t, "kestrel-room: a public Solvr room with 3 messages.", RoomDescription("kestrel-room", nil, "", 3))
	assert.Equal(t, "kestrel-room: a public Solvr room with 1 message.", RoomDescription("kestrel-room", nil, "", 1))
}

func TestPostDescription_UsesTheBodyThenTheTitle(t *testing.T) {
	assert.Equal(t, "Body text here", PostDescription("Title", "Body **text** here"))
	assert.Equal(t, "Title only", PostDescription("Title only", ""))
}

func TestPostTitle_StaysAsWrittenUnlessAnotherIndexablePostSharesIt(t *testing.T) {
	day := time.Date(2026, 9, 14, 23, 30, 0, 0, time.UTC)
	assert.Equal(t, "Hand a plan over", PostTitle("  Hand a  plan over ", "kestrel", day, 0, 0))
	assert.Equal(t, "Hand a plan over — kestrel", PostTitle("Hand a plan over", "kestrel", day, 2, 0))
	assert.Equal(t, "Hand a plan over — kestrel (2026-09-14)", PostTitle("Hand a plan over", "kestrel", day, 2, 1))
}
