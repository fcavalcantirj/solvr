package handlers

import (
	"regexp"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// The one sentence a human pastes (v1.3.5, lane S).
//
// Every use case is the SAME sentence; only a few words change, and those words decide
// what the two agents do. The API owns every character: it serves the sentence as plain
// text (what Copy copies) and as segments whose texts concatenate exactly to that text,
// so a page can give the deciding words their own look without ever composing a word.
// Everything the agent needs beyond the sentence — identity, the handshake, admission,
// the directive pin, recovery — lives in https://solvr.dev/skill.md, which the sentence
// points at first.

const (
	// connectAPIBaseURL and connectAppBaseURL are the production origins an agent and a
	// reader respectively need.
	connectAPIBaseURL = "https://api.solvr.dev"
	connectAppBaseURL = "https://solvr.dev"

	// connectSkillURL is where the sentence sends an agent to learn Solvr.
	connectSkillURL = connectAppBaseURL + "/skill.md"

	// connectSkillFlowParam is the query parameter that carries the flow code on the
	// skill link of a sentence that creates a room. The skill tells the agent to send
	// the code back as flow_id when it creates the room.
	connectSkillFlowParam = "f"

	// ConnectIntentMaxChars bounds the intent: it is a phrase in a sentence, not a
	// document. The agents work out the detail in the room.
	ConnectIntentMaxChars = 200

	// connectEmptyIntent fills the intent when the visitor typed none, so the copied
	// sentence still works: the agent asks what to work on.
	connectEmptyIntent = "work on what I tell you next"

	// connectBJoins is what every second agent is asked to do first, in every preset.
	connectBJoins = " to install the Solvr skill and join your room, "
)

// Segment kinds. A page renders each by kind; it never writes text of its own.
const (
	SegmentText       = "text"
	SegmentLink       = "link"
	SegmentRole       = "role"
	SegmentIntent     = "intent"
	SegmentVisibility = "visibility"
	SegmentHandoff    = "handoff"
)

// PromptSegment is one run of the sentence.
type PromptSegment struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
	// Side is set on a role: "a" receives this sentence, "b" is the agent it hands off to.
	Side string `json:"side,omitempty"`
	// Empty is set on an intent the visitor did not type (the neutral phrase).
	Empty bool `json:"empty,omitempty"`
	// Value is set on a visibility: the room visibility this sentence asks for.
	Value string `json:"value,omitempty"`
}

// SlimPrompt is the sentence: its plain text, its segments, and its length in words.
type SlimPrompt struct {
	Text      string          `json:"text"`
	Segments  []PromptSegment `json:"segments"`
	WordCount int             `json:"word_count"`
}

type promptBuilder struct{ segs []PromptSegment }

func (b *promptBuilder) add(s PromptSegment) *promptBuilder {
	if s.Text != "" {
		b.segs = append(b.segs, s)
	}
	return b
}

func (b *promptBuilder) text(t string) *promptBuilder {
	return b.add(PromptSegment{Kind: SegmentText, Text: t})
}

func (b *promptBuilder) build() SlimPrompt {
	var sb strings.Builder
	for _, s := range b.segs {
		sb.WriteString(s.Text)
	}
	text := sb.String()
	return SlimPrompt{Text: text, Segments: b.segs, WordCount: len(strings.Fields(text))}
}

// connectFilling is one use case: the words that change in the shared sentence.
type connectFilling struct {
	Preset string
	Label  string
	// Key names the agent that receives the sentence (funnel role).
	Key string
	// RoleA receives the sentence and creates the room; RoleB is handed a sentence by it.
	RoleA, RoleB string
	// Job is what RoleB does, after it installs the skill and joins.
	Job string
	// Visibility is the room this use case asks for when the visitor chose none.
	Visibility string
	// ExampleIntent is the intent the guides and the home page show.
	ExampleIntent string
	// Next is the one line on what happens after the copy.
	Next string
}

// connectFillings are the three use cases, in the order every surface lists them. The
// example intents are close in rendered width, so the guides index can line the three
// sentences up word for word without leaving gaps.
var connectFillings = []connectFilling{
	{
		Preset: ConnectPresetPlanAndBuild, Label: "Plan & execute", Key: "planner",
		RoleA: "PLANNER", RoleB: "EXECUTOR",
		Job:        "follow your orders, post its doubts, and post a summary when it's done",
		Visibility: ConnectVisibilityPublic, ExampleIntent: "ship the signup page",
		Next: "Paste it into your planner. Paste its answer into your executor. Watch them in the room.",
	},
	{
		Preset: ConnectPresetCollaborate, Label: "Share context", Key: "learner",
		RoleA: "LEARNER", RoleB: "EXPERT",
		Job:        "answer everything you ask about it until you can work on it alone",
		Visibility: ConnectVisibilityPrivate, ExampleIntent: "learn our billing code",
		Next: "Paste it into the agent that needs to learn. Paste its answer into the one that knows. Watch them in the room.",
	},
	{
		Preset: ConnectPresetBuildAndReview, Label: "Build & review", Key: "builder",
		RoleA: "BUILDER", RoleB: "REVIEWER",
		Job:        "review and test each change you post, and approve or reject it",
		Visibility: ConnectVisibilityPublic, ExampleIntent: "add API rate limiting",
		Next: "Paste it into your builder. Paste its answer into your reviewer. Watch them in the room.",
	},
}

// connectFillingFor returns the use case for a preset key (callers validate the key).
func connectFillingFor(preset string) connectFilling {
	for _, f := range connectFillings {
		if f.Preset == preset {
			return f
		}
	}
	return connectFillings[0]
}

// normalizeIntent trims the intent and folds every run of whitespace (newlines
// included) into one space: it is a phrase inside a sentence.
func normalizeIntent(intent string) string {
	return strings.Join(strings.Fields(intent), " ")
}

// slimConnectPrompt fills the shared sentence for one use case.
//
//	Learn Solvr from [link]. Create a [visibility] Solvr room to [intent], join it as the
//	[ROLE A], and [answer me with a prompt for the] [ROLE B] to install the Solvr skill and
//	join your room, [B's job].
//
// A private room adds one sentence: the second agent gives its id to the human, who
// passes it to the first agent to admit.
//
// This sentence asks an agent to CREATE a room, so its skill link carries the flow code
// of the visit that served it (connectSkillLink). The code adds no word: every other
// character is the same with and without it. flowID is empty for the example sentences
// (GET /v1/connect/examples), which start no flow.
func slimConnectPrompt(f connectFilling, intent, visibility, flowID string) SlimPrompt {
	b := &promptBuilder{}
	b.text("Learn Solvr from ").
		add(PromptSegment{Kind: SegmentLink, Text: connectSkillLink(flowID)}).
		text(". Create a ").
		add(PromptSegment{Kind: SegmentVisibility, Text: visibility, Value: visibility}).
		text(" Solvr room to ")
	if intent = normalizeIntent(intent); intent == "" {
		b.add(PromptSegment{Kind: SegmentIntent, Text: connectEmptyIntent, Empty: true})
	} else {
		b.add(PromptSegment{Kind: SegmentIntent, Text: intent})
	}
	b.text(", join it as the ").
		add(PromptSegment{Kind: SegmentRole, Text: f.RoleA, Side: "a"}).
		text(", and ").
		add(PromptSegment{Kind: SegmentHandoff, Text: "answer me with a prompt for the "}).
		add(PromptSegment{Kind: SegmentRole, Text: f.RoleB, Side: "b"}).
		text(connectBJoins).
		text(f.Job).
		text(".")
	if visibility == ConnectVisibilityPrivate {
		b.text(" It's private, so the ").
			add(PromptSegment{Kind: SegmentRole, Text: f.RoleB, Side: "b"}).
			text(" gives me its agent id for you to admit.")
	}
	return b.build()
}

// connectSkillLink is the skill link of a sentence that creates a room: the skill, with
// the flow code riding on it when there is one. Only a well-formed code is ever put in
// the link, whatever the caller holds.
func connectSkillLink(flowID string) string {
	if !models.ValidFlowCode(flowID) {
		return connectSkillURL
	}
	return connectSkillURL + "?" + connectSkillFlowParam + "=" + flowID
}

// roomRoleJobs is what each role does in a room it joins (GET /v1/rooms/{slug}/connect).
var roomRoleJobs = map[string]string{
	"executor":     "follow the orders pinned there, post your doubts, and post a summary when you're done",
	"reviewer":     "review and test each change posted there, and approve or reject it",
	"expert":       "answer everything you're asked there until the asker can work on it alone",
	"learner":      "ask what you need there, one question at a time, until you can work on it alone",
	"planner":      "post the plan there, pin it as the directive, and direct the work",
	"builder":      "post each change there for review, and act on what the reviewer says",
	"collaborator": "help with the work pinned there, and post what you did",
}

// roomRoleCustomJob is the job of any other role label: the label names the job.
const roomRoleCustomJob = "do that job there, and post what you did"

// roomRolePattern bounds a custom role label: it is shown in capitals inside a sentence.
var roomRolePattern = regexp.MustCompile(`^[a-z][a-z-]{1,23}$`)

// validRoomRole reports whether a ?role= value can be put in a sentence.
func validRoomRole(role string) bool {
	return roomRolePattern.MatchString(role)
}

// slimRoomPrompt is the sentence for an agent joining a real room in a role, in the same
// shape as the first agent's: the room's title is the intent.
//
//	Learn Solvr from [link]. Join the [visibility] Solvr room "[title]" at [room link] as
//	the [ROLE], read it, and [job].
//
// It joins a room and creates none, so its skill link is always the plain one: the flow
// code is only for the call that creates a room.
func slimRoomPrompt(room *models.Room, role string) SlimPrompt {
	visibility := ConnectVisibilityPublic
	if room.IsPrivate {
		visibility = ConnectVisibilityPrivate
	}
	job, ok := roomRoleJobs[role]
	if !ok {
		job = roomRoleCustomJob
	}
	b := &promptBuilder{}
	b.text("Learn Solvr from ").
		add(PromptSegment{Kind: SegmentLink, Text: connectSkillURL}).
		text(". Join the ").
		add(PromptSegment{Kind: SegmentVisibility, Text: visibility, Value: visibility}).
		text(" Solvr room ")
	if title := normalizeIntent(room.DisplayName); title != "" {
		b.text(`"`).add(PromptSegment{Kind: SegmentIntent, Text: title}).text(`" at `)
	}
	b.add(PromptSegment{Kind: SegmentLink, Text: connectAppBaseURL + "/rooms/" + room.Slug}).
		text(" as the ").
		add(PromptSegment{Kind: SegmentRole, Text: strings.ToUpper(role), Side: "b"}).
		text(", read it, and ").
		text(job).
		text(".")
	if room.IsPrivate {
		b.text(" It's private, so give me your agent id first and I'll get you admitted.")
	}
	return b.build()
}

// connectEntriesURL is the canonical room timeline (the directive in force links into it).
func connectEntriesURL(slug string) string {
	return connectAPIBaseURL + "/v1/rooms/" + slug + "/entries"
}
