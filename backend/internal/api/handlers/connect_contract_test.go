package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/connect is the one API-owned contract behind every surface that starts a
// connection (v1.3.5, lane S). It serves ONE sentence per use case — the same sentence,
// with only a few words changed — as plain text and as segments, and the pages render
// the segments by kind. They never compose a word of their own.

type fakeConnectRooms struct {
	room  *models.Room
	err   error
	asked string
}

func (f *fakeConnectRooms) GetBySlug(_ context.Context, slug string) (*models.Room, error) {
	f.asked = slug
	if f.err != nil {
		return nil, f.err
	}
	return f.room, nil
}

func publicExampleRoom(slug string) *models.Room {
	return &models.Room{Slug: slug, DisplayName: "Tic-Tac-Toe Human vs Computer", IsPrivate: false}
}

func newTestConnectHandler(t *testing.T, rooms connectRoomLookup, slug string) *ConnectHandler {
	t.Helper()
	t.Setenv("HOMEPAGE_EXAMPLE_ROOM_SLUG", slug)
	return NewConnectHandler(rooms)
}

func serveConnect(h *ConnectHandler, query string) *httptest.ResponseRecorder {
	target := "/v1/connect"
	if query != "" {
		target += "?" + query
	}
	w := httptest.NewRecorder()
	h.GetConnect(w, httptest.NewRequest(http.MethodGet, target, nil))
	return w
}

func getConnect(t *testing.T, h *ConnectHandler, query string) (ConnectStart, string) {
	t.Helper()
	w := serveConnect(h, query)
	body := w.Body.String()
	require.Equal(t, http.StatusOK, w.Code, "body: %s", body)
	var wrapper struct {
		Data ConnectStart `json:"data"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &wrapper), "body: %s", body)
	return wrapper.Data, body
}

// joined is what the segments say, end to end.
func joined(p SlimPrompt) string {
	var sb strings.Builder
	for _, s := range p.Segments {
		sb.WriteString(s.Text)
	}
	return sb.String()
}

func segmentsOf(p SlimPrompt, kind string) []PromptSegment {
	var out []PromptSegment
	for _, s := range p.Segments {
		if s.Kind == kind {
			out = append(out, s)
		}
	}
	return out
}

var connectUseCases = []struct {
	preset, label, roleA, roleB, job, visibility string
}{
	{"plan-and-build", "Plan & execute", "PLANNER", "EXECUTOR",
		"follow your orders, post its doubts, and post a summary when it's done", "public"},
	{"collaborate", "Share context", "LEARNER", "EXPERT",
		"answer everything you ask about it until you can work on it alone", "private"},
	{"build-and-review", "Build & review", "BUILDER", "REVIEWER",
		"review and test each change you post, and approve or reject it", "public"},
}

func TestConnect_EveryUseCaseFillsTheSameSentence(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for _, uc := range connectUseCases {
		start, body := getConnect(t, h, "preset="+uc.preset+"&intent=ship+the+signup+page")
		text := start.Prompt.Text
		want := "Learn Solvr from https://solvr.dev/skill.md. Create a " + uc.visibility +
			" Solvr room to ship the signup page, join it as the " + uc.roleA +
			", and answer me with a prompt for the " + uc.roleB +
			" to install the Solvr skill and join your room, " + uc.job + "."
		require.True(t, strings.HasPrefix(text, want), "%s: got %q (body %s)", uc.preset, text, body)
		require.Equal(t, uc.preset, start.Selected.Preset)
		require.Equal(t, uc.visibility, start.Selected.Visibility, "%s's own visibility when none is chosen", uc.preset)
		require.Equal(t, start.Prompt.Text, joined(start.Prompt), "%s: the segments are the text", uc.preset)
		require.Equal(t, len(strings.Fields(text)), start.Prompt.WordCount)
		require.Less(t, start.Prompt.WordCount, 120, "%s: one sentence, not a testament", uc.preset)
	}
}

func TestConnect_TheDecidingWordsAreTheirOwnSegments(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for _, uc := range connectUseCases {
		start, _ := getConnect(t, h, "preset="+uc.preset+"&visibility=public&intent=ship+the+signup+page")
		p := start.Prompt
		links := segmentsOf(p, SegmentLink)
		require.Len(t, links, 1)
		require.Equal(t, "https://solvr.dev/skill.md", links[0].Text)
		vis := segmentsOf(p, SegmentVisibility)
		require.Len(t, vis, 1)
		require.Equal(t, "public", vis[0].Text)
		require.Equal(t, "public", vis[0].Value)
		intents := segmentsOf(p, SegmentIntent)
		require.Len(t, intents, 1)
		require.Equal(t, "ship the signup page", intents[0].Text)
		require.False(t, intents[0].Empty)
		roles := segmentsOf(p, SegmentRole)
		require.Len(t, roles, 2)
		require.Equal(t, PromptSegment{Kind: SegmentRole, Text: uc.roleA, Side: "a"}, roles[0])
		require.Equal(t, PromptSegment{Kind: SegmentRole, Text: uc.roleB, Side: "b"}, roles[1])
		hand := segmentsOf(p, SegmentHandoff)
		require.Len(t, hand, 1)
		require.Equal(t, "answer me with a prompt for the ", hand[0].Text)
	}
}

func TestConnect_PrivateAddsOneSentenceAboutTheJoiningAgentsID(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for _, uc := range connectUseCases {
		private, _ := getConnect(t, h, "preset="+uc.preset+"&visibility=private")
		require.True(t, strings.HasSuffix(private.Prompt.Text,
			" It's private, so the "+uc.roleB+" gives me its agent id for you to admit."), private.Prompt.Text)
		require.Equal(t, private.Prompt.Text, joined(private.Prompt))
		public, _ := getConnect(t, h, "preset="+uc.preset+"&visibility=public")
		require.NotContains(t, public.Prompt.Text, "private")
		require.NotContains(t, public.Prompt.Text, "agent id")
	}
}

func TestConnect_NoIntentFillsTheNeutralPhraseAndMarksIt(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	start, _ := getConnect(t, h, "")
	intents := segmentsOf(start.Prompt, SegmentIntent)
	require.Len(t, intents, 1)
	require.Equal(t, "work on what I tell you next", intents[0].Text)
	require.True(t, intents[0].Empty, "the page shows it as the blank to fill")
	require.Equal(t, "", start.Selected.Intent)
	require.Contains(t, start.Prompt.Text, "Solvr room to work on what I tell you next, join it as the PLANNER")
}

// A typed intent appears exactly once, inside its own segment, and nothing else in the
// sentence changes with it — however hostile its characters.
func TestConnect_TheIntentIsInsertedVerbatimAndNothingElseChanges(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	hostile := `Fix the "flaky" login test; it fails ~1 in 5 runs $(rm -rf ~) ` + "`whoami`" + ` ${HOME}`
	got, _ := getConnect(t, h, "intent="+url.QueryEscape(hostile))
	marker, _ := getConnect(t, h, "intent=PLACEHOLDER_INTENT_MARKER")

	require.Equal(t, 1, strings.Count(got.Prompt.Text, hostile))
	require.Equal(t, hostile, segmentsOf(got.Prompt, SegmentIntent)[0].Text)
	require.Equal(t, strings.Replace(marker.Prompt.Text, "PLACEHOLDER_INTENT_MARKER", hostile, 1), got.Prompt.Text)
	require.Equal(t, hostile, got.Selected.Intent)
}

func TestConnect_TheIntentIsOnePhrase(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	start, _ := getConnect(t, h, "intent="+url.QueryEscape("  ship\nthe   signup\tpage  "))
	require.Equal(t, "ship the signup page", start.Selected.Intent)
	require.Contains(t, start.Prompt.Text, "Solvr room to ship the signup page, join")
}

func TestConnect_RefusesAnIntentTooLongForASentence(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	ok := strings.Repeat("a", ConnectIntentMaxChars)
	require.Equal(t, http.StatusOK, serveConnect(h, "intent="+ok).Code)
	w := serveConnect(h, "intent="+ok+"b")
	require.Equal(t, http.StatusBadRequest, w.Code)
	require.Contains(t, w.Body.String(), "INTENT_TOO_LONG")
}

func TestConnect_RefusesAnUnknownPresetOrVisibilityInsteadOfGuessing(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for query, code := range map[string]string{
		"preset=solo": "INVALID_PRESET", "preset=builder": "INVALID_PRESET", "visibility=unlisted": "INVALID_VISIBILITY",
	} {
		w := serveConnect(h, query)
		require.Equal(t, http.StatusBadRequest, w.Code, query)
		require.Contains(t, w.Body.String(), code, query)
	}
}

// The switch on /connect swaps use cases without asking again: every use case's sentence
// arrives with the contract, filled with the same intent and visibility.
func TestConnect_EveryUseCaseArrivesForTheSwitch(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	start, _ := getConnect(t, h, "preset=collaborate&visibility=public&intent=ship+the+signup+page")
	require.Len(t, start.Presets, 3)
	selected := 0
	for i, uc := range connectUseCases {
		p := start.Presets[i]
		require.Equal(t, uc.preset, p.Value)
		require.Equal(t, uc.label, p.Label)
		require.NotEmpty(t, p.Next)
		require.Contains(t, p.Prompt.Text, "join it as the "+uc.roleA)
		require.Contains(t, p.Prompt.Text, "Create a public Solvr room to ship the signup page")
		if p.Selected {
			selected++
			require.Equal(t, start.Prompt, p.Prompt, "the selected use case is the contract's prompt")
			require.Equal(t, start.Next, p.Next)
		}
	}
	require.Equal(t, 1, selected)
	require.Equal(t, "collaborate", start.Selected.Preset)
}

// A visitor who has not chosen a visibility sees each use case's own; a chosen one
// applies to every use case, so flipping it never depends on which pair is showing.
func TestConnect_AChosenVisibilityAppliesToEveryUseCase(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	unchosen, _ := getConnect(t, h, "")
	for i, uc := range connectUseCases {
		require.Equal(t, uc.visibility, segmentsOf(unchosen.Presets[i].Prompt, SegmentVisibility)[0].Value, uc.preset)
	}
	chosen, _ := getConnect(t, h, "visibility=private")
	for _, p := range chosen.Presets {
		require.Equal(t, "private", segmentsOf(p.Prompt, SegmentVisibility)[0].Value, p.Value)
	}
}

// The sentence teaches nothing but itself: no endpoint, no credential, no install of any
// package, no template syntax, no funnel id. The only URL is the skill.
func TestConnect_TheSentenceNamesOnlyTheSkill(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	anyURL := regexp.MustCompile(`https?://[^\s,]+`)
	for _, uc := range connectUseCases {
		for _, vis := range []string{"public", "private"} {
			start, _ := getConnect(t, h, "preset="+uc.preset+"&visibility="+vis)
			text := start.Prompt.Text
			for _, u := range anyURL.FindAllString(text, -1) {
				require.Equal(t, "https://solvr.dev/skill.md", strings.TrimRight(u, "."), "%s/%s", uc.preset, vis)
			}
			lower := strings.ToLower(text)
			for _, banned := range []string{"api.solvr.dev", "/v1/", "authorization", "bearer", "solvr_", "$", "${",
				"`", "localhost", "npm", "npx", "pip ", "brew ", "solvr.sh", "mcp", "sdk", "plugin", "flow"} {
				require.NotContains(t, lower, banned, "%s/%s: %q", uc.preset, vis, text)
			}
			require.NotContains(t, text, start.Selected.FlowID)
		}
	}
}

// The browser still reports its own funnel steps with a server-minted flow id; the id
// never rides in the sentence.
func TestConnect_MintsAFlowIDForTheBrowserOnly(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	a, _ := getConnect(t, h, "")
	b, _ := getConnect(t, h, "")
	require.Regexp(t, `^f_[0-9a-f]{24}$`, a.Selected.FlowID)
	require.NotEqual(t, a.Selected.FlowID, b.Selected.FlowID)
	require.NotContains(t, a.Prompt.Text, a.Selected.FlowID)
}

func TestConnect_ServesTheChromeAroundTheSentence(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	start, body := getConnect(t, h, "")
	require.Equal(t, ConnectInstructionVersion, start.InstructionVersion)
	require.Equal(t, "2.0", ConnectInstructionVersion, "the prompt shape changed: one sentence and its segments")
	require.Equal(t, "Connect your agents", start.Heading)
	require.Equal(t, "What should they do?", start.IntentField.Label)
	require.Equal(t, ConnectIntentMaxChars, start.IntentField.MaxChars)
	require.Equal(t, "Paste it into your planner. Paste its answer into your executor. Watch them in the room.", start.Next)
	require.Equal(t, ConnectMore{Label: "Your imagination", Detail: "Any number of agents"}, start.More)
	// What only explained the long prompt is gone (v0: no backward compatibility).
	for _, gone := range []string{`"steps"`, `"note"`, `"requirements"`, `"add_agent"`, `"customize"`,
		`"task_field"`, `"intro"`, `"instruction"`, `"next_step"`, `"copied_detail"`, `"visibility_options"`} {
		require.NotContains(t, body, gone)
	}
	lower := strings.ToLower(body)
	for _, secret := range []string{"api_key\":", "room_token\":", "owner_id", "solvr_rt_"} {
		require.NotContains(t, lower, secret)
	}
}

func TestConnect_LinksTheRealExampleOrFallsBackToTheRoomList(t *testing.T) {
	rooms := &fakeConnectRooms{room: publicExampleRoom("tictactoe-human-vs-computer-20260920")}
	start, _ := getConnect(t, newTestConnectHandler(t, rooms, "tictactoe-human-vs-computer-20260920"), "")
	require.Equal(t, "real", start.Example.Kind)
	require.Equal(t, "/rooms/tictactoe-human-vs-computer-20260920", start.Example.URL)
	require.NotEmpty(t, start.Example.Label)

	for name, r := range map[string]connectRoomLookup{
		"missing": &fakeConnectRooms{err: errors.New("room not found")},
		"private": &fakeConnectRooms{room: &models.Room{Slug: "secret", IsPrivate: true}},
		"none":    nil,
	} {
		start, _ := getConnect(t, newTestConnectHandler(t, r, "example-room"), "")
		require.Equal(t, "directory", start.Example.Kind, name)
		require.Equal(t, "/rooms", start.Example.URL, name)
		require.NotEmpty(t, start.Prompt.Text, "%s: the sentence never depends on the example room", name)
	}
}

// GET /v1/connect/examples: the three sentences the guides and the home page show, filled
// with their example intents and their own visibility. Nothing is minted for them.
func TestConnectExamples_ServeTheThreeExampleSentences(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	w := httptest.NewRecorder()
	h.GetConnectExamples(w, httptest.NewRequest(http.MethodGet, "/v1/connect/examples", nil))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NotContains(t, w.Body.String(), "flow")

	var wrapper struct {
		Data ConnectExamples `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	ex := wrapper.Data
	require.Equal(t, ConnectInstructionVersion, ex.InstructionVersion)
	require.Len(t, ex.Presets, 3)
	intents := []string{"ship the signup page", "learn our billing code", "add API rate limiting"}
	for i, uc := range connectUseCases {
		p := ex.Presets[i]
		require.Equal(t, uc.preset, p.Value)
		require.Equal(t, uc.label, p.Label)
		require.NotEmpty(t, p.Next)
		require.Equal(t, p.Prompt.Text, joined(p.Prompt))
		require.Equal(t, intents[i], segmentsOf(p.Prompt, SegmentIntent)[0].Text)
		require.Equal(t, uc.visibility, segmentsOf(p.Prompt, SegmentVisibility)[0].Value)
		require.Less(t, p.Prompt.WordCount, 120)
	}
}
