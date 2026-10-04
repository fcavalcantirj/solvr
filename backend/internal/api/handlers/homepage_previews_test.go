package handlers

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/db"
	"github.com/fcavalcantirj/solvr/internal/models"
)

// The homepage's featured rooms (SPEC Part 26, "Featured rooms"): the operator
// curates a pool, the API picks today's rooms from it and quotes what each room
// set out to do and what came out. Pure builders — no database.

func featured(slugs ...string) []db.FeaturedRoom {
	out := make([]db.FeaturedRoom, 0, len(slugs))
	for i, s := range slugs {
		out = append(out, db.FeaturedRoom{
			RoomID: uuid.New(), Slug: s, Public: true,
			FeaturedAt: time.Date(2026, 10, 1, 0, 0, i, 0, time.UTC),
		})
	}
	return out
}

func selectedSlugs(rooms []db.FeaturedRoom) []string {
	out := make([]string, 0, len(rooms))
	for _, r := range rooms {
		out = append(out, r.Slug)
	}
	return out
}

func TestSelectFeaturedRooms_ShowsASmallPoolWholeAndInOrder(t *testing.T) {
	day := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	got := selectFeaturedRooms(featured("a", "b"), "the-example", day)
	assert.Equal(t, []string{"a", "b"}, selectedSlugs(got))
}

func TestSelectFeaturedRooms_NeverShowsTheExampleRoom(t *testing.T) {
	// The example section further down the page quotes that room; the homepage
	// never shows one room twice.
	day := time.Date(2026, 10, 4, 12, 0, 0, 0, time.UTC)
	got := selectFeaturedRooms(featured("a", "the-example", "b"), "the-example", day)
	assert.Equal(t, []string{"a", "b"}, selectedSlugs(got))
}

func TestSelectFeaturedRooms_RotatesALargePoolOneDayAtATime(t *testing.T) {
	pool := featured("a", "b", "c", "d", "e")
	// 2026-10-04 is day 20730 since 1970-01-01; 20730 mod 5 = 0.
	d0 := time.Date(2026, 10, 4, 0, 0, 1, 0, time.UTC)
	assert.Equal(t, []string{"a", "b", "c"}, selectedSlugs(selectFeaturedRooms(pool, "x", d0)))
	// Stable through the day.
	assert.Equal(t, []string{"a", "b", "c"}, selectedSlugs(selectFeaturedRooms(pool, "x", d0.Add(23*time.Hour))))
	// Next day starts one further on, wrapping around the pool.
	assert.Equal(t, []string{"b", "c", "d"}, selectedSlugs(selectFeaturedRooms(pool, "x", d0.AddDate(0, 0, 1))))
	assert.Equal(t, []string{"e", "a", "b"}, selectedSlugs(selectFeaturedRooms(pool, "x", d0.AddDate(0, 0, 4))))
	// The day is the UTC day, whatever zone the clock is read in.
	saoPaulo := time.FixedZone("BRT", -3*3600)
	assert.Equal(t, []string{"a", "b", "c"}, selectedSlugs(selectFeaturedRooms(pool, "x", d0.In(saoPaulo))))
}

func TestSelectFeaturedRooms_EmptyPoolShowsNothing(t *testing.T) {
	assert.Empty(t, selectFeaturedRooms(nil, "x", time.Now()))
	assert.Empty(t, selectFeaturedRooms(featured("x"), "x", time.Now()), "a pool of only the example room is empty")
}

func TestOverviewPlainText_RemovesMarkdownAndCollapsesSpace(t *testing.T) {
	in := "## PROJECT — One AI, Many Bodies\n\n> quoted line\n- **bold** item with `code`\n1. __under__ [a link](https://x.y)\n\n\nend"
	assert.Equal(t, "PROJECT — One AI, Many Bodies quoted line bold item with code under a link end", overviewPlainText(in))
	assert.Equal(t, "📍 AntiHunter N16R8 Lab", overviewPlainText("📍 **AntiHunter N16R8 Lab**"), "emoji stay, emphasis goes")
	assert.Equal(t, "", overviewPlainText("   \n\n "))
}

func previewSource(slug string, ask, outcome *models.Message) db.PreviewSource {
	return db.PreviewSource{
		Room: models.Room{
			Slug: slug, DisplayName: "Room " + slug, Description: strPtr("Two agents build a parser"),
			MessageCount: 9, LastActiveAt: time.Now().Add(-25 * time.Hour),
		},
		Participants: []db.RoomParticipant{
			{Name: "planner", AuthorType: "agent", MessageCount: 5},
			{Name: "executor", AuthorType: "agent", MessageCount: 4},
		},
		ParticipantCount: 2,
		Ask:              ask,
		Outcome:          outcome,
	}
}

func TestOverviewPreviews_QuoteTheAskAndTheOutcomeAsPlainText(t *testing.T) {
	ask := &models.Message{ID: 1, AuthorType: "agent", AgentName: "planner", Content: "## Build the **parser**", SequenceNum: intPtr(2)}
	long := "RESULT: " + strings.Repeat("parser shipped and verified ", 20)
	outcome := &models.Message{ID: 9, AuthorType: "agent", AgentName: "executor", Content: long, SequenceNum: intPtr(9)}

	section := buildOverviewPreviews([]db.PreviewSource{previewSource("room-a", ask, outcome)})

	require.NotNil(t, section)
	require.Len(t, section.Rooms, 1)
	p := section.Rooms[0]
	assert.Equal(t, "/rooms/room-a", p.URL)
	assert.Equal(t, "Two agents build a parser", p.Purpose)
	require.NotNil(t, p.Ask)
	assert.Equal(t, "Build the parser", p.Ask.Excerpt)
	assert.False(t, p.Ask.IsExcerpt)
	assert.Equal(t, "planner", p.Ask.Author)
	assert.Equal(t, "/rooms/room-a#message-2", p.Ask.MessageURL)

	require.NotNil(t, p.Outcome)
	assert.True(t, p.Outcome.IsExcerpt)
	assert.LessOrEqual(t, len([]rune(p.Outcome.Excerpt)), overviewQuoteMaxChars+1, "cut on the server, with its ellipsis")
	assert.True(t, strings.HasSuffix(p.Outcome.Excerpt, "…"))
	assert.Equal(t, fmt.Sprintf("Excerpt — the original message is %d characters", len([]rune(strings.TrimSpace(long)))), p.Outcome.ExcerptNote)
	assert.Equal(t, "/rooms/room-a#message-9", p.Outcome.MessageURL)
	assert.NotEmpty(t, section.Note, "the section states it is selected, not ranked")
	assert.Equal(t, "The ask", section.AskLabel, "the API words the captions the page shows")
	assert.Equal(t, "The outcome", section.OutcomeLabel)
}

func TestOverviewPreviews_ARoomWithOneMessageHasNoOutcome(t *testing.T) {
	ask := &models.Message{ID: 1, AuthorType: "agent", AgentName: "planner", Content: "Room is open", SequenceNum: intPtr(1)}
	section := buildOverviewPreviews([]db.PreviewSource{previewSource("room-a", ask, nil)})
	require.NotNil(t, section)
	assert.NotNil(t, section.Rooms[0].Ask)
	assert.Nil(t, section.Rooms[0].Outcome)
}

func TestOverviewPreviews_NothingToShowMeansNoSection(t *testing.T) {
	assert.Nil(t, buildOverviewPreviews(nil), "an empty pool omits the section")
}

func TestOverviewPreviews_CapsAtThree(t *testing.T) {
	sources := make([]db.PreviewSource, 0, 5)
	for _, s := range []string{"room-a", "room-b", "room-c", "room-d", "room-e"} {
		sources = append(sources, previewSource(s, nil, nil))
	}
	section := buildOverviewPreviews(sources)
	require.NotNil(t, section)
	assert.Len(t, section.Rooms, maxOverviewPreviews)
	assert.Equal(t, 3, maxOverviewPreviews)
}

func TestOverviewPreviews_StateTheRealParticipantCountBeyondTheShownNames(t *testing.T) {
	names := make([]db.RoomParticipant, 0, 6)
	for i := 0; i < 6; i++ {
		names = append(names, db.RoomParticipant{Name: fmt.Sprintf("agent-%d", i), AuthorType: "agent", MessageCount: 1})
	}
	big := db.PreviewSource{
		Room:             models.Room{Slug: "twenty-agents", DisplayName: "Twenty agents", LastActiveAt: time.Now()},
		Participants:     names,
		ParticipantCount: 20,
	}
	pair := db.PreviewSource{
		Room:             models.Room{Slug: "two-agents", DisplayName: "Two agents", LastActiveAt: time.Now()},
		Participants:     names[:2],
		ParticipantCount: 2,
	}
	oneMore := db.PreviewSource{
		Room:             models.Room{Slug: "seven-agents", DisplayName: "Seven agents", LastActiveAt: time.Now()},
		Participants:     names,
		ParticipantCount: 7,
	}

	section := buildOverviewPreviews([]db.PreviewSource{big, pair, oneMore})

	require.NotNil(t, section)
	require.Len(t, section.Rooms, 3)
	assert.Len(t, section.Rooms[0].Participants, 6, "the card still lists a bounded set of names")
	assert.Equal(t, 20, section.Rooms[0].ParticipantCount, "but states how many took part")
	assert.Equal(t, "+14 more participants", section.Rooms[0].MoreParticipantsLabel)
	assert.Equal(t, 2, section.Rooms[1].ParticipantCount)
	assert.Empty(t, section.Rooms[1].MoreParticipantsLabel, "nothing hidden, nothing to say")
	assert.Equal(t, "+1 more participant", section.Rooms[2].MoreParticipantsLabel)
}
