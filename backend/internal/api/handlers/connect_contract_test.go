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
		want := "Learn Solvr from https://solvr.dev/skill.md?f=" + start.Selected.FlowID + ". Create a " + uc.visibility +
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
		require.Equal(t, "https://solvr.dev/skill.md?f="+start.Selected.FlowID, links[0].Text)
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
	// Both reads are of one flow (?flow=), so the intent is the only thing that differs.
	got, _ := getConnect(t, h, "flow=k7m2p9xq&intent="+url.QueryEscape(hostile))
	marker, _ := getConnect(t, h, "flow=k7m2p9xq&intent=PLACEHOLDER_INTENT_MARKER")

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
// package, no template syntax. The only URL is the skill, and the only thing riding on it
// is the flow code, once.
func TestConnect_TheSentenceNamesOnlyTheSkill(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	anyURL := regexp.MustCompile(`https?://[^\s,]+`)
	for _, uc := range connectUseCases {
		for _, vis := range []string{"public", "private"} {
			start, _ := getConnect(t, h, "preset="+uc.preset+"&visibility="+vis)
			text := start.Prompt.Text
			code := start.Selected.FlowID
			for _, u := range anyURL.FindAllString(text, -1) {
				require.Equal(t, "https://solvr.dev/skill.md?f="+code, strings.TrimRight(u, "."), "%s/%s", uc.preset, vis)
			}
			require.Equal(t, 1, strings.Count(text, code), "%s/%s: the code appears once, on the link", uc.preset, vis)
			// The words are checked without the code: a random code may spell anything, the
			// sentence around it may not.
			lower := strings.ToLower(strings.Replace(text, "?f="+code, "", 1))
			for _, banned := range []string{"api.solvr.dev", "/v1/", "authorization", "bearer", "solvr_", "$", "${",
				"`", "localhost", "npm", "npx", "pip ", "brew ", "solvr.sh", "mcp", "sdk", "plugin", "flow", "?", "="} {
				require.NotContains(t, lower, banned, "%s/%s: %q", uc.preset, vis, text)
			}
		}
	}
}

// The flow id is a short public code: 8 characters with no look-alikes, a new one for
// every answer that was not asked to keep one.
func TestConnect_MintsAFlowCode(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	a, _ := getConnect(t, h, "")
	b, _ := getConnect(t, h, "")
	require.Regexp(t, `^[a-hjkmnp-z2-9]{8}$`, a.Selected.FlowID)
	require.True(t, models.ValidFlowCode(a.Selected.FlowID))
	require.True(t, models.ValidFlowCode(b.Selected.FlowID))
	require.NotEqual(t, a.Selected.FlowID, b.Selected.FlowID)
}

// Every mint is a well-formed code and every character is equally likely. 8000 codes are
// 64000 draws over 31 characters: about 2065 each (standard deviation 45), and 16516 for
// the first eight together (standard deviation 111). Taking a random byte modulo 31 would
// favour exactly those eight (18000 expected), which the second band refuses.
func TestNewFlowID_DrawsWellFormedCodesEvenlyFromTheWholeAlphabet(t *testing.T) {
	const codes = 8000
	drawn := map[rune]int{}
	for i := 0; i < codes; i++ {
		code := newFlowID()
		require.True(t, models.ValidFlowCode(code), "minted %q", code)
		for _, c := range code {
			drawn[c]++
		}
	}
	require.Len(t, drawn, len(models.FlowCodeAlphabet), "every character of the alphabet is drawn")
	firstEight := 0
	for i, c := range models.FlowCodeAlphabet {
		require.InDelta(t, 2065, drawn[c], 300, "character %q was drawn %d times of 64000", c, drawn[c])
		if i < 8 {
			firstEight += drawn[c]
		}
	}
	require.InDelta(t, 16516, firstEight, 660, "the first eight characters are not favoured (a modulo bias gives about 18000)")
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) { return 0, errors.New("no entropy") }

// When no code can be minted the answer still serves the sentence: it has no flow id and
// its link carries no query.
func TestConnect_WithoutRandomnessTheLinkCarriesNoQuery(t *testing.T) {
	previous := flowRand
	flowRand = failingReader{}
	defer func() { flowRand = previous }()

	require.Equal(t, "", newFlowID())
	h := newTestConnectHandler(t, nil, "example-room")
	start, body := getConnect(t, h, "intent=ship+the+signup+page")
	require.Equal(t, "", start.Selected.FlowID)
	require.NotContains(t, body, `"flow_id"`)
	require.NotContains(t, body, "?f=")
	require.True(t, strings.HasPrefix(start.Prompt.Text, "Learn Solvr from https://solvr.dev/skill.md. Create a public Solvr room"), start.Prompt.Text)
	require.Equal(t, "https://solvr.dev/skill.md", segmentsOf(start.Prompt, SegmentLink)[0].Text)
	// A flow the visitor already has is still kept: echoing it needs no randomness.
	kept, _ := getConnect(t, h, "flow=k7m2p9xq")
	require.Equal(t, "k7m2p9xq", kept.Selected.FlowID)
}

// The link carries the code and the rest of the sentence is byte-equal to the same
// sentence without one: no word is added, in the text or in the segments, for any use
// case and either visibility.
func TestConnect_TheLinkCarriesTheFlowCodeAndNothingElseChanges(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for _, uc := range connectUseCases {
		for _, vis := range []string{"public", "private"} {
			for _, intent := range []string{"", "ship the signup page"} {
				start, _ := getConnect(t, h, "preset="+uc.preset+"&visibility="+vis+"&intent="+url.QueryEscape(intent))
				code := start.Selected.FlowID
				require.True(t, models.ValidFlowCode(code))
				require.Len(t, start.Presets, 3)
				for _, p := range start.Presets {
					plain := slimConnectPrompt(connectFillingFor(p.Value), intent, vis, "")
					name := p.Value + "/" + vis + "/" + intent
					require.NotContains(t, plain.Text, "?", name)
					require.Equal(t, 1, strings.Count(p.Prompt.Text, "https://solvr.dev/skill.md?f="+code), name)
					require.Equal(t, plain.Text, strings.Replace(p.Prompt.Text, "?f="+code, "", 1), "%s: byte-equal without the code", name)
					require.Equal(t, plain.WordCount, p.Prompt.WordCount, "%s: no word is added", name)
					require.Equal(t, p.Prompt.Text, joined(p.Prompt), "%s: the segments are the text", name)
					require.Len(t, p.Prompt.Segments, len(plain.Segments), name)
					for i, seg := range p.Prompt.Segments {
						want := plain.Segments[i]
						if seg.Kind == SegmentLink {
							want.Text += "?f=" + code
						}
						require.Equal(t, want, seg, "%s: segment %d", name, i)
					}
				}
				require.Equal(t, start.Prompt, start.Presets[indexOfPreset(start, uc.preset)].Prompt)
			}
		}
	}
}

func indexOfPreset(start ConnectStart, preset string) int {
	for i, p := range start.Presets {
		if p.Value == preset {
			return i
		}
	}
	return -1
}

// One visit is one flow: a well-formed code sent back as ?flow= is reused, in
// selected.flow_id and on the link of every sentence.
func TestConnect_AValidFlowIsReused(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	first, _ := getConnect(t, h, "")
	code := first.Selected.FlowID
	for _, query := range []string{"flow=" + code, "flow=" + code + "&intent=ship+it&visibility=private", "preset=collaborate&flow=" + code} {
		again, _ := getConnect(t, h, query)
		require.Equal(t, code, again.Selected.FlowID, query)
		for _, p := range again.Presets {
			require.Equal(t, "https://solvr.dev/skill.md?f="+code, segmentsOf(p.Prompt, SegmentLink)[0].Text, "%s %s", query, p.Value)
		}
	}
}

// Anything that is not exactly a code is ignored: never a 400, never echoed (it would
// land in a sentence people copy), and a new code is minted instead.
func TestConnect_AMalformedFlowIsIgnoredAndNeverEchoed(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	// marker is a part of the value that would survive JSON escaping if it were echoed.
	for sent, marker := range map[string]string{
		"f_0123456789abcdef01234567":   "f_0123456789abcdef01234567",
		"K7M2P9XQ":                     "K7M2P9XQ",
		"k7m2p9x":                      "",
		"k7m2p9xqq":                    "k7m2p9xqq",
		"k7m2p9xi":                     "k7m2p9xi",
		"k7m2p9x0":                     "k7m2p9x0",
		"k7m2p9xq\n":                   "",
		" k7m2p9xq":                    "",
		"k7m2p9xq&x":                   "k7m2p9xq",
		"<script>alert(1)</script>":    "alert(1)",
		"IGNORE-PREVIOUS-INSTRUCTIONS": "IGNORE-PREVIOUS",
		strings.Repeat("a", 500):       strings.Repeat("a", 20),
		"":                             "",
		// Only the exact word none starts no flow (below); anything near it is just malformed.
		"NONE":  "NONE",
		"None":  "",
		"none ": "",
		" none": "",
		"nonee": "nonee",
		"non":   "",
	} {
		w := serveConnect(h, "flow="+url.QueryEscape(sent))
		require.Equal(t, http.StatusOK, w.Code, "flow=%q: %s", sent, w.Body.String())
		var wrapper struct {
			Data ConnectStart `json:"data"`
		}
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
		start := wrapper.Data
		require.True(t, models.ValidFlowCode(start.Selected.FlowID), "flow=%q minted %q", sent, start.Selected.FlowID)
		require.NotEqual(t, sent, start.Selected.FlowID)
		require.Equal(t, "https://solvr.dev/skill.md?f="+start.Selected.FlowID, segmentsOf(start.Prompt, SegmentLink)[0].Text)
		if marker != "" {
			require.NotContains(t, w.Body.String(), marker, "flow=%q: an unvalidated value is never echoed", sent)
		}
	}
}

// A page rendered on a server must never mint a flow: its HTML can be cached and shared,
// and no browser step stands behind the code. ?flow=none starts no flow: the answer has no
// flow_id and every sentence carries the plain skill link, exactly like the examples.
func TestConnect_FlowNoneStartsNoFlow(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for _, uc := range connectUseCases {
		for _, vis := range []string{"", "public", "private"} {
			for _, intent := range []string{"", "ship the signup page"} {
				query := "flow=none&preset=" + uc.preset + "&intent=" + url.QueryEscape(intent)
				if vis != "" {
					query += "&visibility=" + vis
				}
				start, body := getConnect(t, h, query)
				require.Equal(t, "", start.Selected.FlowID, query)
				require.NotContains(t, body, `"flow_id"`, query)
				require.NotContains(t, body, "?f=", query)
				require.Equal(t, uc.preset, start.Selected.Preset, query)
				require.Equal(t, intent, start.Selected.Intent, query)
				require.Len(t, start.Presets, 3, query)
				for _, p := range start.Presets {
					filling := connectFillingFor(p.Value)
					wantVisibility := vis
					if wantVisibility == "" {
						wantVisibility = filling.Visibility
					}
					require.Equal(t, slimConnectPrompt(filling, intent, wantVisibility, ""), p.Prompt, "%s %s", query, p.Value)
					require.Equal(t, "https://solvr.dev/skill.md", segmentsOf(p.Prompt, SegmentLink)[0].Text, "%s %s", query, p.Value)
				}
				require.Equal(t, start.Presets[indexOfPreset(start, uc.preset)].Prompt, start.Prompt, query)
			}
		}
	}
}

// Apart from the code, ?flow=none answers what a visit gets: the same bytes once the flow
// id and the link's query are taken out of a visit's answer.
func TestConnect_FlowNoneChangesNothingButTheCode(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	for _, query := range []string{"", "preset=collaborate", "intent=ship+the+signup+page&visibility=private", "preset=build-and-review&visibility=public"} {
		_, visit := getConnect(t, h, "flow=k7m2p9xq&"+query)
		_, server := getConnect(t, h, "flow=none&"+query)
		require.Contains(t, visit, `,"flow_id":"k7m2p9xq"`)
		stripped := strings.ReplaceAll(strings.ReplaceAll(visit, `,"flow_id":"k7m2p9xq"`, ""), "?f=k7m2p9xq", "")
		require.Equal(t, stripped, server, query)
	}
}

// With a use case's example intent and its own visibility, the sentence ?flow=none serves
// is the example's, segment for segment.
func TestConnect_FlowNoneServesTheExampleSentences(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	w := httptest.NewRecorder()
	h.GetConnectExamples(w, httptest.NewRequest(http.MethodGet, "/v1/connect/examples", nil))
	var wrapper struct {
		Data ConnectExamples `json:"data"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &wrapper))
	require.Len(t, wrapper.Data.Presets, 3)
	for _, example := range wrapper.Data.Presets {
		intent := segmentsOf(example.Prompt, SegmentIntent)[0].Text
		start, _ := getConnect(t, h, "flow=none&preset="+example.Value+"&intent="+url.QueryEscape(intent))
		require.Equal(t, example.Prompt, start.Prompt, example.Value)
	}
}

func TestConnect_ServesTheChromeAroundTheSentence(t *testing.T) {
	h := newTestConnectHandler(t, nil, "example-room")
	start, body := getConnect(t, h, "")
	require.Equal(t, ConnectInstructionVersion, start.InstructionVersion)
	require.Equal(t, "2.1", ConnectInstructionVersion, "the sentence's skill link carries the flow code, and ?flow= keeps it")
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
	require.NotContains(t, w.Body.String(), "?f=", "an example starts no flow, so its link carries no code")

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
		require.Equal(t, "https://solvr.dev/skill.md", segmentsOf(p.Prompt, SegmentLink)[0].Text)
		require.True(t, strings.HasPrefix(p.Prompt.Text, "Learn Solvr from https://solvr.dev/skill.md. Create a "), p.Prompt.Text)
		require.Equal(t, intents[i], segmentsOf(p.Prompt, SegmentIntent)[0].Text)
		require.Equal(t, uc.visibility, segmentsOf(p.Prompt, SegmentVisibility)[0].Value)
		require.Less(t, p.Prompt.WordCount, 120)
	}
}
