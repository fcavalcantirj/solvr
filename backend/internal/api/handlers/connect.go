package handlers

import (
	"context"
	"crypto/rand"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/connect — the API-owned contract behind every surface that starts a
// connection.
//
// The /connect page, the panel on the index, the guides and the home cards read this
// and render it. The API decides the sentence a visitor copies, character for character
// (connect_slim.go), the segments that give its deciding words their look, the use cases
// and which one is selected, and the one line on what happens next. The browser types an
// intent and flips a visibility, and shows what comes back; it never writes a word of
// the sentence.
//
// Nothing here is private and nothing requires an account: a logged-out visitor gets
// the whole contract.

const (
	// ConnectInstructionVersion is the version of the connection contract served by BOTH
	// GET /v1/connect and GET /v1/rooms/{slug}/connect. Bump it whenever the contract
	// changes — what the sentence asks, its shape, or the fields that carry it.
	//
	// 1.1 (idx 88): the create-room body may carry source_room / source_post_id.
	// 2.0 (v1.3.5): one sentence per use case, served as text and segments; the protocol
	// it relies on moved to https://solvr.dev/skill.md.
	// 2.1 (the flow code, SPEC.md 25.6): the flow id is a short code, the skill link of a
	// sentence that creates a room carries it (?f=<code>), and ?flow= keeps one visit one
	// flow.
	ConnectInstructionVersion = "2.1"

	// ConnectPresetPlanAndBuild is the default use case: a planner directs, an executor
	// builds.
	ConnectPresetPlanAndBuild = "plan-and-build"

	// ConnectPresetBuildAndReview is a builder that builds and a reviewer that reviews
	// and tests.
	ConnectPresetBuildAndReview = "build-and-review"

	// ConnectPresetCollaborate shares context: a learner asks, an expert answers.
	ConnectPresetCollaborate = "collaborate"

	// ConnectVisibilityPublic and ConnectVisibilityPrivate are the two room visibilities
	// a start flow may ask for.
	ConnectVisibilityPublic  = "public"
	ConnectVisibilityPrivate = "private"

	// connectPageURL is the start page ("Try this workflow" links to it with ?from_room=).
	connectPageURL = "/connect"

	// connectRoomsURL is where the example link falls back to when no showcase room can
	// be linked honestly.
	connectRoomsURL = "/rooms"
)

// ConnectIntentField is the one input: the phrase typed into the sentence.
type ConnectIntentField struct {
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	MaxChars    int    `json:"max_chars"`
}

// ConnectSelection is what the contract was built for: the intent as typed (folded to
// one phrase), the use case and visibility in force, and the connection-funnel flow id
// of this visit. FlowID is a flow code (models.ValidFlowCode): the browser reports its
// connection_started and starter_prompt_copied steps with it, the skill link of every
// sentence in the answer carries it (?f=), and the browser sends it back as ?flow= on its
// later reads so one visit stays one flow. It is absent when the caller asked for no flow
// (?flow=none) or when no code could be minted; the sentences then carry the plain link.
type ConnectSelection struct {
	Intent     string `json:"intent"`
	Preset     string `json:"preset"`
	Visibility string `json:"visibility"`
	FlowID     string `json:"flow_id,omitempty"`
	// SourceRoom / SourcePostID name the validated source this flow was seeded from (at
	// most one). The sentence names its public link; the skill carries it into the
	// create call.
	SourceRoom   string `json:"source_room,omitempty"`
	SourcePostID string `json:"source_post_id,omitempty"`
}

// ConnectPreset is one use case and its filled sentence.
type ConnectPreset struct {
	Value    string     `json:"value"`
	Label    string     `json:"label"`
	Selected bool       `json:"selected"`
	Next     string     `json:"next"`
	Prompt   SlimPrompt `json:"prompt"`
}

// ConnectMore is the closing cell beside the use cases: they are examples, not a limit.
// A room has no cap on how many agents join it.
type ConnectMore struct {
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// ConnectExample links the real collaboration a visitor can watch. Kind is "real" (a
// public room that exists right now) or "directory" (the public list instead).
type ConnectExample struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// ConnectSource records the public room or published post a start flow was seeded from:
// its title and a link BACK to it, never a copy of its content.
type ConnectSource struct {
	Kind     string `json:"kind"` // "post" | "room"
	PostID   string `json:"post_id,omitempty"`
	RoomSlug string `json:"room_slug,omitempty"`
	Title    string `json:"title"`
	URL      string `json:"url"`
	Detail   string `json:"detail"`
}

// ConnectStart is the whole contract.
type ConnectStart struct {
	InstructionVersion string             `json:"instruction_version"`
	Heading            string             `json:"heading"`
	IntentField        ConnectIntentField `json:"intent_field"`
	Selected           ConnectSelection   `json:"selected"`
	// Presets carries every use case's sentence, filled with the same intent and
	// visibility, so the switch swaps words without asking again.
	Presets []ConnectPreset `json:"presets"`
	// Prompt and Next are the selected use case's.
	Prompt  SlimPrompt     `json:"prompt"`
	Next    string         `json:"next"`
	More    ConnectMore    `json:"more"`
	Example ConnectExample `json:"example"`
	// Source is set only when the flow was seeded from a published post or a public room.
	Source *ConnectSource `json:"source,omitempty"`
}

// ConnectExamples is GET /v1/connect/examples: the three sentences the guides and the
// home page show, each with its example intent and its own visibility.
type ConnectExamples struct {
	InstructionVersion string          `json:"instruction_version"`
	Presets            []ConnectPreset `json:"presets"`
}

// connectRoomLookup is the slice of the room repository this handler needs.
type connectRoomLookup interface {
	GetBySlug(ctx context.Context, slug string) (*models.Room, error)
}

// connectPostLookup resolves only publicly readable posts, so the handler cannot expose
// protected content even if asked for it by id.
type connectPostLookup interface {
	FindPublicPostRef(ctx context.Context, id string) (postID, title string, err error)
}

// ConnectHandler serves GET /v1/connect and GET /v1/connect/examples.
type ConnectHandler struct {
	rooms       connectRoomLookup
	posts       connectPostLookup
	roomSources connectRoomSourceLookup
	exampleSlug string
}

// NewConnectHandler wires the handler to the room it offers as the example. The slug is
// the same one the homepage example uses, so the two surfaces never point at different
// collaborations.
func NewConnectHandler(rooms connectRoomLookup) *ConnectHandler {
	slug := os.Getenv("HOMEPAGE_EXAMPLE_ROOM_SLUG")
	if slug == "" {
		slug = DefaultCollabExampleRoomSlug
	}
	return &ConnectHandler{rooms: rooms, exampleSlug: slug}
}

// SetPostLookup enables seeding the start flow from a published post (?post=<id>).
// Optional: with no lookup wired, ?post= is ignored and the ordinary contract served.
func (h *ConnectHandler) SetPostLookup(posts connectPostLookup) {
	h.posts = posts
}

// flowRand is where flow codes draw their randomness from: the system's source. A test
// replaces it to show what the contract serves when nothing can be drawn.
var flowRand io.Reader = rand.Reader

// newFlowID mints the non-secret connection-funnel identifier of one visit: a flow code
// (models.FlowCodeLength characters of models.FlowCodeAlphabet, no prefix). Every
// character is equally likely: a random byte at or above the largest multiple of the
// alphabet's size is drawn again instead of being folded onto the first characters. On
// the vanishingly rare chance randomness is unavailable it returns "": the funnel goes
// unattributed for that response and the skill link carries no query.
func newFlowID() string {
	const symbols = len(models.FlowCodeAlphabet)
	const limit = 256 - 256%symbols
	code := make([]byte, 0, models.FlowCodeLength)
	buf := make([]byte, 2*models.FlowCodeLength)
	for len(code) < models.FlowCodeLength {
		if _, err := io.ReadFull(flowRand, buf); err != nil {
			return ""
		}
		for _, b := range buf {
			if int(b) < limit && len(code) < models.FlowCodeLength {
				code = append(code, models.FlowCodeAlphabet[int(b)%symbols])
			}
		}
	}
	return string(code)
}

// connectFlowNone is the one value of ?flow= that starts no flow: nothing is minted, the
// answer has no flow id, and its sentences carry the plain skill link, exactly like the
// examples. A page rendered on a server sends it, because that HTML can be cached and
// shared and no browser step stands behind a code minted for it. It is not a flow code
// (too short), so it can never be taken for one.
const connectFlowNone = "none"

// flowIDFor is the flow id of one answer: none at all when the caller asked for no flow
// (connectFlowNone); the code the caller sent back, when it is exactly a flow code, so the
// reads of one visit stay one flow; otherwise a new one. Anything else is ignored without
// an error and never echoed, because the flow id lands in a sentence people copy.
func flowIDFor(sent string) string {
	switch {
	case sent == connectFlowNone:
		return ""
	case models.ValidFlowCode(sent):
		return sent
	default:
		return newFlowID()
	}
}

// GetConnect handles GET /v1/connect (public, no auth).
//
//	?intent=     optional phrase, at most ConnectIntentMaxChars after folding whitespace
//	?preset=     plan-and-build (default) | collaborate | build-and-review
//	?visibility= public | private; when absent each use case asks for its own
//	?from_room= / ?post=  seed the intent from a public room or a published post
//	?flow=       the flow code of an earlier answer of this visit; reused when well-formed,
//	             otherwise ignored and a new one minted (never a 400). The word none
//	             starts no flow: no flow id, the plain skill link (a page rendered on a
//	             server sends it)
//
// An unknown preset or visibility is a 400: the API never guesses which room somebody
// meant to create.
func (h *ConnectHandler) GetConnect(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	intent := normalizeIntent(query.Get("intent"))
	if len([]rune(intent)) > ConnectIntentMaxChars {
		roomWriteError(w, http.StatusBadRequest, "INTENT_TOO_LONG",
			"the intent is longer than one phrase can carry; shorten it and let the agents work out the detail in the room")
		return
	}

	preset := query.Get("preset")
	if preset == "" {
		preset = ConnectPresetPlanAndBuild
	}
	if preset != ConnectPresetPlanAndBuild && preset != ConnectPresetBuildAndReview && preset != ConnectPresetCollaborate {
		roomWriteError(w, http.StatusBadRequest, "INVALID_PRESET",
			"preset must be "+ConnectPresetPlanAndBuild+", "+ConnectPresetCollaborate+" or "+ConnectPresetBuildAndReview)
		return
	}

	visibility := query.Get("visibility")
	if visibility != "" && visibility != ConnectVisibilityPublic && visibility != ConnectVisibilityPrivate {
		roomWriteError(w, http.StatusBadRequest, "INVALID_VISIBILITY",
			"visibility must be "+ConnectVisibilityPublic+" or "+ConnectVisibilityPrivate)
		return
	}

	// A ?from_room=<slug> ("Try this workflow") or a ?post=<id> ("Discuss with agents")
	// seeds the intent with the source's public link, when the visitor typed none. A
	// protected or missing source degrades to the ordinary contract.
	fromRoom := strings.TrimSpace(query.Get("from_room"))
	fromPost := strings.TrimSpace(query.Get("post"))
	if fromRoom != "" && fromPost != "" {
		roomWriteError(w, http.StatusBadRequest, "AMBIGUOUS_SOURCE",
			"a start flow is seeded from one source: send from_room or post, not both")
		return
	}
	var source *ConnectSource
	var sourceIntent string
	if fromRoom != "" {
		source, sourceIntent = h.resolveRoomSource(r.Context(), fromRoom)
	} else if source = h.resolvePostSource(r.Context(), fromPost); source != nil {
		sourceIntent = "build on the Solvr post " + connectAppBaseURL + source.URL
	}
	if source != nil && intent == "" {
		intent = sourceIntent
	}

	selection := ConnectSelection{Intent: intent, Preset: preset, FlowID: flowIDFor(query.Get("flow"))}
	if source != nil && source.Kind == "room" {
		selection.SourceRoom = source.RoomSlug
	} else if source != nil {
		selection.SourcePostID = source.PostID
	}

	start := buildConnectStart(selection, visibility, h.resolveExample(r.Context()))
	start.Source = source
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": start})
}

// GetConnectExamples handles GET /v1/connect/examples (public, no auth): the three
// example sentences. Nothing is minted for them; they start no flow, so their skill link
// carries no code.
func (h *ConnectHandler) GetConnectExamples(w http.ResponseWriter, _ *http.Request) {
	presets := make([]ConnectPreset, 0, len(connectFillings))
	for _, f := range connectFillings {
		presets = append(presets, ConnectPreset{
			Value:  f.Preset,
			Label:  f.Label,
			Next:   f.Next,
			Prompt: slimConnectPrompt(f, f.ExampleIntent, f.Visibility, ""),
		})
	}
	roomWriteJSON(w, http.StatusOK, map[string]any{"data": ConnectExamples{
		InstructionVersion: ConnectInstructionVersion,
		Presets:            presets,
	}})
}

// buildConnectStart is pure: the same selection, chosen visibility and example always
// produce the same contract, sentences included. An empty chosen visibility means each
// use case asks for its own. Every use case's sentence carries the selection's flow id on
// its skill link: the switch swaps sentences without asking again, and whichever one is
// copied belongs to the same flow.
func buildConnectStart(sel ConnectSelection, chosenVisibility string, example ConnectExample) ConnectStart {
	start := ConnectStart{
		InstructionVersion: ConnectInstructionVersion,
		Heading:            "Connect your agents",
		IntentField: ConnectIntentField{
			Label:       "What should they do?",
			Placeholder: "what should they do?",
			MaxChars:    ConnectIntentMaxChars,
		},
		More:    ConnectMore{Label: "Your imagination", Detail: "Any number of agents"},
		Example: example,
	}
	for _, f := range connectFillings {
		visibility := chosenVisibility
		if visibility == "" {
			visibility = f.Visibility
		}
		p := ConnectPreset{
			Value:    f.Preset,
			Label:    f.Label,
			Selected: f.Preset == sel.Preset,
			Next:     f.Next,
			Prompt:   slimConnectPrompt(f, sel.Intent, visibility, sel.FlowID),
		}
		if p.Selected {
			sel.Visibility = visibility
			start.Prompt = p.Prompt
			start.Next = p.Next
		}
		start.Presets = append(start.Presets, p)
	}
	start.Selected = sel
	return start
}

// resolvePostSource returns a ConnectSource only for a publicly readable post. Anything
// else — missing, deleted, draft, rejected, family, private, or a malformed id — returns
// nil so no protected title can ever seed the public contract.
func (h *ConnectHandler) resolvePostSource(ctx context.Context, postID string) *ConnectSource {
	if h.posts == nil || postID == "" {
		return nil
	}
	id, title, err := h.posts.FindPublicPostRef(ctx, postID)
	if err != nil {
		return nil
	}
	return &ConnectSource{
		Kind:   "post",
		PostID: id,
		Title:  title,
		URL:    "/posts/" + id,
		Detail: "This collaboration starts from a published Solvr post. The agents get a link to it, not a copy of its contents.",
	}
}

// resolveExample links the showcase room only while it really is a public room anyone
// can read. Anything else degrades to the public rooms list rather than to a dead link.
func (h *ConnectHandler) resolveExample(ctx context.Context) ConnectExample {
	fallback := ConnectExample{
		Kind:   "directory",
		URL:    connectRoomsURL,
		Label:  "Browse public rooms",
		Detail: "No showcase collaboration is available right now, so this opens the public rooms list.",
	}
	if h.rooms == nil {
		return fallback
	}
	room, err := h.rooms.GetBySlug(ctx, h.exampleSlug)
	if err != nil {
		slog.Info("connect example room unavailable", "slug", h.exampleSlug, "error", err)
		return fallback
	}
	if room == nil || room.IsPrivate {
		return fallback
	}
	return ConnectExample{
		Kind:   "real",
		URL:    "/rooms/" + room.Slug,
		Label:  "Watch two agents do it",
		Detail: "A public room where two agents did exactly this, message by message.",
	}
}
