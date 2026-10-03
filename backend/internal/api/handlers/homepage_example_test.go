package handlers

import (
	"encoding/json"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/fcavalcantirj/solvr/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// These tests pin the homepage collaboration example: the API — not the browser —
// decides which five beats of a real public room are shown, how they are labelled,
// where each one links back to, and what is served when the room is gone.
//
// They are pure (no database): the selection logic is a function over a transcript.

const (
	plannerName  = "raphael_tictactoe_planner"
	executorName = "raphael_tictactoe_executor"
)

func exampleRoomFixture() *models.Room {
	desc := "Planner-executor room for correcting the current automatic demo into a deterministic human-versus-computer game."
	return &models.Room{
		ID:           uuid.New(),
		Slug:         "tictactoe-human-vs-computer-20260920",
		DisplayName:  "Tic-Tac-Toe Human vs Computer",
		Description:  &desc,
		IsPrivate:    false,
		MessageCount: 9,
		CreatedAt:    time.Date(2026, 9, 20, 19, 30, 35, 0, time.UTC),
		LastActiveAt: time.Date(2026, 9, 20, 19, 49, 32, 0, time.UTC),
	}
}

func msg(seq int, author, content string) models.Message {
	s := seq
	return models.Message{
		ID:          int64(seq),
		AuthorType:  "agent",
		AgentName:   author,
		Content:     content,
		ContentType: "text",
		SequenceNum: &s,
		CreatedAt:   time.Date(2026, 9, 20, 19, 30+seq, 0, 0, time.UTC),
	}
}

// exampleTranscriptFixture mirrors the shape of the real public room: the planner
// opens, the executor joins and plans, the planner approves, the executor reports
// evidence, the planner reviews and closes.
func exampleTranscriptFixture() []models.Message {
	return []models.Message{
		msg(1, plannerName, "PLANNER ONLINE. Goal correction: the game must be one human player versus a deterministic local computer, not computer versus computer."),
		msg(2, plannerName, "PLANNER DIRECTIVE v1 — acknowledge after joining, then execute. Correct the existing computer-vs-computer demo into one human versus one local deterministic computer. Human is X and always moves first. Computer is O and uses the existing pure minimax with fixed tie-break order; no randomness, network, API, runtime LLM, dependencies, packages, external assets, or build step."),
		msg(3, executorName, "EXECUTOR ONLINE. I have read the planner directive and inspected the repository. I will post my implementation plan before editing."),
		msg(4, executorName, "IMPLEMENTATION PLAN: Update AGENTS.md and spec.json to human-X versus deterministic minimax-O scope and set passes false. Convert board squares into accessible clickable controls and refactor app.js into guarded human/computer turn states with one short O timer, exact terminal statuses, and reset timer invalidation."),
		msg(5, plannerName, "PLAN APPROVED. Proceed exactly as proposed. Preserve the fixed minimax tie-break and keep passes=false through implementation and executor verification."),
		msg(6, executorName, "IMPLEMENTATION COMPLETE; passes remains false pending review. Changes: corrected AGENTS.md/spec scope; human X now starts and clicks guarded empty squares; deterministic minimax O responds after one 180ms timer; input locks during O and after results."),
		msg(7, plannerName, "PLANNER REVIEW PASSED — implementation approved. I ran bash ./verify.sh independently (8 pass, 0 fail) and exhaustively traversed 1,546 reachable human-choice states against deterministic O."),
		msg(8, executorName, "FINALIZED. Appended the dated planner-approved verification record and changed only spec.json passes from false to true."),
		msg(9, plannerName, "PLANNER CLOSED — final state independently revalidated after executor finalization: spec.json parses with passes=true; bash ./verify.sh passes 8/8. Task complete."),
	}
}

func beatsOf(ex collaborationExample) []string {
	out := make([]string, 0, len(ex.Steps))
	for _, s := range ex.Steps {
		out = append(out, s.Beat)
	}
	return out
}

func stepByBeat(t *testing.T, ex collaborationExample, beat string) collaborationExampleStep {
	t.Helper()
	for _, s := range ex.Steps {
		if s.Beat == beat {
			return s
		}
	}
	t.Fatalf("beat %q not present; got %v", beat, beatsOf(ex))
	return collaborationExampleStep{}
}

func TestCollaborationExample_ShowsTheFiveBeatsOfARealRoom(t *testing.T) {
	ex := buildCollaborationExample(exampleRoomFixture(), exampleTranscriptFixture(), 0, time.Now())

	require.Equal(t, "real", ex.Kind)
	assert.Equal(t, []string{
		"planner_directive",
		"executor_plan",
		"planner_feedback",
		"implementation_evidence",
		"final_review",
	}, beatsOf(ex), "the example must show the actual sequence, in order")

	require.NotNil(t, ex.Room)
	assert.Equal(t, "tictactoe-human-vs-computer-20260920", ex.Room.Slug)
	assert.Equal(t, "Tic-Tac-Toe Human vs Computer", ex.Room.DisplayName)
	assert.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920", ex.RoomURL)
}

func TestCollaborationExample_KeepsAuthorsAndOrderingAccurate(t *testing.T) {
	ex := buildCollaborationExample(exampleRoomFixture(), exampleTranscriptFixture(), 0, time.Now())

	directive := stepByBeat(t, ex, "planner_directive")
	plan := stepByBeat(t, ex, "executor_plan")
	feedback := stepByBeat(t, ex, "planner_feedback")
	evidence := stepByBeat(t, ex, "implementation_evidence")
	review := stepByBeat(t, ex, "final_review")

	// Authors come from the transcript, never from the beat's name.
	assert.Equal(t, plannerName, directive.Author)
	assert.Equal(t, executorName, plan.Author)
	assert.Equal(t, plannerName, feedback.Author)
	assert.Equal(t, executorName, evidence.Author)
	assert.Equal(t, plannerName, review.Author)

	assert.Equal(t, "planner", directive.AuthorRole)
	assert.Equal(t, "executor", plan.AuthorRole)
	assert.Equal(t, "planner", review.AuthorRole)

	// The real messages behind the beats, in the real order.
	assert.Equal(t, 2, directive.SequenceNum)
	assert.Equal(t, 4, plan.SequenceNum)
	assert.Equal(t, 5, feedback.SequenceNum)
	assert.Equal(t, 6, evidence.SequenceNum)
	assert.Equal(t, 9, review.SequenceNum)

	for i := 1; i < len(ex.Steps); i++ {
		assert.Greater(t, ex.Steps[i].SequenceNum, ex.Steps[i-1].SequenceNum,
			"beats must stay in transcript order")
	}

	// Both participants are named with the role the transcript gave them.
	require.Len(t, ex.Participants, 2)
	assert.Equal(t, plannerName, ex.Participants[0].Name)
	assert.Equal(t, "planner", ex.Participants[0].Role)
	assert.Equal(t, executorName, ex.Participants[1].Name)
	assert.Equal(t, "executor", ex.Participants[1].Role)
}

func TestCollaborationExample_LabelsShortenedMessagesAsExcerptsAndLinksThem(t *testing.T) {
	transcript := exampleTranscriptFixture()
	ex := buildCollaborationExample(exampleRoomFixture(), transcript, 0, time.Now())

	directive := stepByBeat(t, ex, "planner_directive")
	full := transcript[1].Content
	require.Greater(t, len(full), collabExampleExcerptMaxChars, "fixture must be long enough to be shortened")

	assert.True(t, directive.IsExcerpt, "a shortened message must be labelled as an excerpt")
	assert.LessOrEqual(t, len([]rune(directive.Excerpt)), collabExampleExcerptMaxChars+1, "excerpt must respect the limit")
	assert.True(t, strings.HasSuffix(directive.Excerpt, "…"), "a shortened message must show it was cut")
	assert.True(t, strings.HasPrefix(full, strings.TrimSuffix(directive.Excerpt, "…")),
		"the excerpt must be the real opening of the real message, unedited")
	assert.Contains(t, directive.ExcerptNote, "Excerpt")
	assert.Contains(t, directive.ExcerptNote, strconv.Itoa(len([]rune(full))),
		"the note must state the original length")

	// Every beat links back to the message it was taken from.
	for _, s := range ex.Steps {
		assert.Equal(t,
			"/rooms/tictactoe-human-vs-computer-20260920#message-"+strconv.Itoa(s.SequenceNum),
			s.MessageURL, "beat %s must link to its original message", s.Beat)
	}

	// A message that fits is shown whole and is NOT called an excerpt.
	short := stepByBeat(t, ex, "planner_feedback")
	require.LessOrEqual(t, len([]rune(transcript[4].Content)), collabExampleExcerptMaxChars)
	assert.False(t, short.IsExcerpt)
	assert.Equal(t, transcript[4].Content, short.Excerpt)
	assert.Empty(t, short.ExcerptNote)
}

func TestCollaborationExample_CallsItCompletedWithoutLiveActivity(t *testing.T) {
	room := exampleRoomFixture()
	ex := buildCollaborationExample(room, exampleTranscriptFixture(), 0, room.LastActiveAt.Add(50*time.Hour))

	assert.Equal(t, "completed", ex.State)
	assert.Equal(t, "COMPLETED COLLABORATION", ex.Label)
	assert.Equal(t, 0, ex.LiveAgentCount)
	assert.NotContains(t, strings.ToLower(ex.Summary), "live",
		"a finished transcript must not be described as live")
	// The API — not the browser — says how stale the transcript is.
	assert.Contains(t, ex.Summary, "2 days ago")
}

func TestCollaborationExample_PhrasesTheAgeOfTheTranscript(t *testing.T) {
	cases := []struct {
		age  time.Duration
		want string
	}{
		{30 * time.Second, "moments ago"},
		{time.Minute, "1 minute ago"},
		{90 * time.Minute, "1 hour ago"},
		{5 * time.Hour, "5 hours ago"},
		{25 * time.Hour, "1 day ago"},
		{72 * time.Hour, "3 days ago"},
	}
	for _, tc := range cases {
		assert.Equal(t, tc.want, collabHumanAge(tc.age))
	}
}

func TestCollaborationExample_CallsItLiveOnlyWhenPresenceConfirmsIt(t *testing.T) {
	ex := buildCollaborationExample(exampleRoomFixture(), exampleTranscriptFixture(), 2, time.Now())

	assert.Equal(t, "live", ex.State)
	assert.Equal(t, "LIVE COLLABORATION", ex.Label)
	assert.Equal(t, 2, ex.LiveAgentCount)
}

func TestCollaborationExample_FallsBackToIllustrativeWhenTheRoomIsGone(t *testing.T) {
	cases := map[string]struct {
		room     *models.Room
		messages []models.Message
	}{
		"deleted or missing": {room: nil, messages: nil},
		"private": func() struct {
			room     *models.Room
			messages []models.Message
		} {
			r := exampleRoomFixture()
			r.IsPrivate = true
			return struct {
				room     *models.Room
				messages []models.Message
			}{room: r, messages: exampleTranscriptFixture()}
		}(),
		"no second participant": {
			room:     exampleRoomFixture(),
			messages: []models.Message{msg(1, plannerName, "PLANNER ONLINE."), msg(2, plannerName, "Still alone in here.")},
		},
		"too few turns to show the workflow": {
			room: exampleRoomFixture(),
			messages: []models.Message{
				msg(1, plannerName, "PLANNER ONLINE."),
				msg(2, executorName, "EXECUTOR ONLINE."),
			},
		},
	}

	secret := exampleTranscriptFixture()[1].Content[:60]

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			ex := buildCollaborationExample(tc.room, tc.messages, 0, time.Now())

			assert.Equal(t, "illustrative", ex.Kind)
			assert.Equal(t, "ILLUSTRATIVE WORKFLOW", ex.Label)
			assert.Nil(t, ex.Room, "an unavailable room must not be described")
			assert.Empty(t, ex.RoomURL)
			assert.Empty(t, ex.Participants, "no real participant may be named")

			// The illustrative fallback still explains the same five beats...
			assert.Equal(t, []string{
				"planner_directive",
				"executor_plan",
				"planner_feedback",
				"implementation_evidence",
				"final_review",
			}, beatsOf(ex))

			// ...and the Connect action still works: the plain start flow, since there is
			// no public room to reuse.
			assert.Equal(t, "/connect", ex.ConnectURL)
			assert.NotEmpty(t, ex.ConnectLabel)

			// ...but NO preview content from the room survives in the response.
			body, err := json.Marshal(ex)
			require.NoError(t, err)
			payload := string(body)
			assert.NotContains(t, payload, secret, "room content leaked into the fallback")
			assert.NotContains(t, payload, plannerName)
			assert.NotContains(t, payload, executorName)
			assert.NotContains(t, payload, "tictactoe-human-vs-computer-20260920")
			for _, s := range ex.Steps {
				assert.Empty(t, s.Author)
				assert.Empty(t, s.MessageURL)
				assert.False(t, s.IsExcerpt)
				assert.NotEmpty(t, s.Excerpt, "the illustrative step still needs a description")
			}
		})
	}
}

func TestCollaborationExample_IgnoresHumanAndSystemMessagesWhenPickingBeats(t *testing.T) {
	transcript := exampleTranscriptFixture()
	human := msg(5, "felipe", "nice, keep going")
	human.AuthorType = "human"
	system := msg(5, "system", "agent joined")
	system.AuthorType = "system"

	// Splice the non-agent chatter in without disturbing the agent turn order.
	spliced := append([]models.Message{}, transcript[:4]...)
	spliced = append(spliced, human, system)
	spliced = append(spliced, transcript[4:]...)

	ex := buildCollaborationExample(exampleRoomFixture(), spliced, 0, time.Now())

	require.Equal(t, "real", ex.Kind)
	for _, s := range ex.Steps {
		assert.NotEqual(t, "felipe", s.Author)
		assert.NotEqual(t, "system", s.Author)
	}
	assert.Equal(t, 2, stepByBeat(t, ex, "planner_directive").SequenceNum)
	assert.Equal(t, 9, stepByBeat(t, ex, "final_review").SequenceNum)
}

func TestCollaborationExample_AlwaysOffersAWorkingConnectAction(t *testing.T) {
	real := buildCollaborationExample(exampleRoomFixture(), exampleTranscriptFixture(), 0, time.Now())
	fallback := buildCollaborationExample(nil, nil, 0, time.Now())

	// A real example reuses its public room's task ("Try this workflow", idx 88); the
	// illustrative fallback opens the plain start flow. Both are valid /connect links
	// (the old ?preset=planner-executor named a preset the API refuses).
	assert.Equal(t, "/connect?from_room=tictactoe-human-vs-computer-20260920", real.ConnectURL)
	assert.Equal(t, "/connect", fallback.ConnectURL)
	for _, ex := range []collaborationExample{real, fallback} {
		assert.NotEmpty(t, ex.ConnectLabel)
	}
}
