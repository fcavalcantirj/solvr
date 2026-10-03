package handlers

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/fcavalcantirj/solvr/internal/models"
)

// GET /v1/connect — the API-owned contract behind every surface that starts a
// connection.
//
// The compact panel on the index and the full /connect page read THIS and
// render it. The API decides the heading, the field labels, which presets and
// visibilities exist, which one is selected, what each choice means in plain
// words, what the copy control is called, where the copied prompt must be
// pasted, and the prompt itself, character for character. The browser types
// into a field and shows what comes back; it never writes a prompt, never
// decides what public means, and never assembles an endpoint of its own.
//
// Nothing here is private and nothing requires an account: a logged-out
// visitor gets the whole contract.

const (
	// ConnectInstructionVersion is the version of the machine-readable connection
	// contract served by BOTH GET /v1/connect and GET /v1/rooms/{slug}/connect. It
	// is the single source of truth: bump it whenever the connection contract
	// changes — the endpoints an agent calls, the authentication sequence, or the
	// prompt shape — so a client can tell which contract it is following and a
	// contract change forces exactly one version bump across both surfaces.
	//
	// 1.1 (idx 88): the create-room body may carry source_room / source_post_id, and
	// every prompt says how to report completion with a clean room link.
	ConnectInstructionVersion = "1.1"

	// ConnectPresetPlanAndBuild is the default shape: one planner agent
	// directs, one executor agent builds.
	ConnectPresetPlanAndBuild = "plan-and-build"

	// ConnectPresetBuildAndReview is one builder that builds and one reviewer
	// that reviews and tests. No single agent directs the other.
	ConnectPresetBuildAndReview = "build-and-review"

	// ConnectPresetCollaborate is two peers in one room, no hierarchy.
	ConnectPresetCollaborate = "collaborate"

	// ConnectVisibilityPublic and ConnectVisibilityPrivate are the two room
	// visibilities a start flow may ask for.
	ConnectVisibilityPublic  = "public"
	ConnectVisibilityPrivate = "private"

	// ConnectTaskMaxChars bounds the optional task description. A task longer
	// than this is a document, not a prompt line, and is refused rather than
	// silently cut.
	ConnectTaskMaxChars = 2000

	// connectPageURL is the full start page the compact panel always points at.
	connectPageURL   = "/connect"
	connectPageLabel = "Open the full start page"

	// connectRoomsURL is where the example link falls back to when no showcase
	// room can be linked honestly.
	connectRoomsURL = "/rooms"
)

// ConnectOption is one choice the visitor can make, with the API's own
// explanation of what choosing it means.
type ConnectOption struct {
	Value       string `json:"value"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Selected    bool   `json:"selected"`
}

// ConnectTaskField is the single optional input on the start flow.
type ConnectTaskField struct {
	Label       string `json:"label"`
	Placeholder string `json:"placeholder"`
	Optional    bool   `json:"optional"`
	Note        string `json:"note"`
	MaxChars    int    `json:"max_chars"`
}

// ConnectSelection is what the contract was built for: the task as typed, the
// preset and visibility in force, and the connection-funnel flow id issued for
// this response. The browser reads FlowID to report its connection_started and
// starter_prompt_copied steps, and the same id is embedded in the copied prompt
// so the room's server steps join the same attempt.
type ConnectSelection struct {
	Task       string `json:"task"`
	Preset     string `json:"preset"`
	Visibility string `json:"visibility"`
	FlowID     string `json:"flow_id,omitempty"`
	// SourceRoom / SourcePostID name the validated source this flow was seeded from
	// (at most one). The prompt carries it into the create-room body so the new room
	// records its provenance; both are public identifiers, never a credential.
	SourceRoom   string `json:"source_room,omitempty"`
	SourcePostID string `json:"source_post_id,omitempty"`
}

// ConnectPrompt is the one thing the visitor copies.
//
// Instruction is the sentence shown beside the control — it names the agent
// that must receive the prompt. NextStep is what happens after the paste, so a
// visitor knows a second copy/paste is coming before they start. CopiedDetail is
// the confirmation shown ONLY after a successful copy: it names the agent the
// prompt is for and what comes back, so the feedback explains the next move
// rather than merely relabelling the button.
type ConnectPrompt struct {
	Key          string `json:"key"`
	Label        string `json:"label"`
	CopiedLabel  string `json:"copied_label"`
	CopiedDetail string `json:"copied_detail"`
	Instruction  string `json:"instruction"`
	NextStep     string `json:"next_step"`
	Text         string `json:"text"`
}

// ConnectStep is one of the two initial copy/paste actions.
type ConnectStep struct {
	Number int    `json:"number"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// ConnectExample links the real collaboration a visitor can read before
// starting. Kind is "real" (a public room that exists right now) or
// "directory" (no showcase room is available, so this opens the public list).
type ConnectExample struct {
	Kind   string `json:"kind"`
	URL    string `json:"url"`
	Label  string `json:"label"`
	Detail string `json:"detail"`
}

// ConnectAddAgentControl is the "Add another agent" optional control: a
// role-specific prompt the visitor can copy for a third, fourth, or Nth
// participant in the same room. SlugPlaceholder is the unmistakable token the
// agent substitutes for the real room slug; RolePrompt is the complete prompt
// to paste. Detail carries the guarantee that copying alone connects nothing.
type ConnectAddAgentControl struct {
	Label           string `json:"label"`
	Detail          string `json:"detail"`
	SlugPlaceholder string `json:"slug_placeholder"`
	RolePrompt      string `json:"role_prompt"`
}

// ConnectCustomizeSection offers advanced instructions and direct API examples
// under the Customize heading. It requires no participant count, model choice,
// category, or tags before starting — the visitor can read and copy freely.
type ConnectCustomizeSection struct {
	Key                  string   `json:"key"`
	Label                string   `json:"label"`
	Detail               string   `json:"detail"`
	ApiExamples          []string `json:"api_examples"`
	AdvancedInstructions []string `json:"advanced_instructions"`
}

// ConnectRequirements states, in the API's own words, what a client needs to
// run this flow and what it explicitly does NOT need. It is how the contract
// stays client-independent regardless of which agent product a visitor runs:
//
//   - Detail/NotNeeded pin that the only capability required is outbound HTTPS
//     — no provider SDK, model-vendor subscription, or shared filesystem.
//   - ClientExamples are named products (Claude Code, OpenClaw, Kimi Code)
//     offered as optional examples, never as a required onboarding choice, and
//     ClientExamplesNote refuses to claim tested compatibility for a client that
//     has not been verified end to end.
//   - MissingCapability + Help* give the honest failure: an agent that cannot
//     make HTTPS requests reports the missing capability and follows the help
//     link instead of claiming it connected.
type ConnectRequirements struct {
	Label              string   `json:"label"`
	Detail             string   `json:"detail"`
	NotNeeded          []string `json:"not_needed"`
	ClientExamples     []string `json:"client_examples"`
	ClientExamplesNote string   `json:"client_examples_note"`
	MissingCapability  string   `json:"missing_capability"`
	HelpURL            string   `json:"help_url"`
	HelpLabel          string   `json:"help_label"`
}

// ConnectSource records the published Post a start flow was seeded from, when a visitor
// arrives via "Discuss with agents" on a post. It carries the post's title and a link
// BACK to the post — an authorized content reference, never a copy of the post body — so
// the agents read the source through the API's own access rules. Only a publicly readable
// post ever produces a source; a draft, rejected, family, private, or missing post
// degrades to the ordinary contract with no source and no title leaked.
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
	InstructionVersion string                  `json:"instruction_version"`
	Heading            string                  `json:"heading"`
	Intro              string                  `json:"intro"`
	PageURL            string                  `json:"page_url"`
	PageLabel          string                  `json:"page_label"`
	TaskField          ConnectTaskField        `json:"task_field"`
	PresetsLabel       string                  `json:"presets_label"`
	Presets            []ConnectOption         `json:"presets"`
	VisibilityLabel    string                  `json:"visibility_label"`
	VisibilityOptions  []ConnectOption         `json:"visibility_options"`
	Selected           ConnectSelection        `json:"selected"`
	Prompt             ConnectPrompt           `json:"prompt"`
	Steps              []ConnectStep           `json:"steps"`
	Example            ConnectExample          `json:"example"`
	Note               string                  `json:"note"`
	Requirements       ConnectRequirements     `json:"requirements"`
	AddAgent           ConnectAddAgentControl  `json:"add_agent"`
	Customize          ConnectCustomizeSection `json:"customize"`
	// Source is set only when the flow was seeded from a published post or a public room.
	Source *ConnectSource `json:"source,omitempty"`
}

// connectRoomLookup is the slice of the room repository this handler needs.
type connectRoomLookup interface {
	GetBySlug(ctx context.Context, slug string) (*models.Room, error)
}

// connectPostLookup is the slice of the post repository the handler needs to seed a start
// flow from a published post. FindPublicPostRef returns only publicly readable posts, so
// the handler cannot expose protected content even if asked for it by id.
type connectPostLookup interface {
	FindPublicPostRef(ctx context.Context, id string) (postID, title string, err error)
}

// ConnectHandler serves GET /v1/connect.
type ConnectHandler struct {
	rooms       connectRoomLookup
	posts       connectPostLookup
	roomSources connectRoomSourceLookup
	exampleSlug string
}

// NewConnectHandler wires the handler to the room it offers as the example.
// The slug is the same one the homepage example uses, so the two surfaces
// never point at different collaborations.
func NewConnectHandler(rooms connectRoomLookup) *ConnectHandler {
	slug := os.Getenv("HOMEPAGE_EXAMPLE_ROOM_SLUG")
	if slug == "" {
		slug = DefaultCollabExampleRoomSlug
	}
	return &ConnectHandler{rooms: rooms, exampleSlug: slug}
}

// SetPostLookup enables seeding the start flow from a published post (?post=<id>). It is
// optional: with no lookup wired, ?post= is ignored and the ordinary contract is served.
func (h *ConnectHandler) SetPostLookup(posts connectPostLookup) {
	h.posts = posts
}

// newFlowID mints a non-secret connection-funnel identifier for one connect
// response. It is a random hex token (never a template placeholder), safe to
// paste into an agent prompt and to report from the browser. On the vanishingly
// rare chance randomness is unavailable, the funnel simply goes unattributed for
// that response rather than blocking the contract.
func newFlowID() string {
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		return ""
	}
	return "f_" + hex.EncodeToString(b)
}

// GetConnect handles GET /v1/connect (public, no auth).
//
//	?task=       optional, free text, bounded by ConnectTaskMaxChars
//	?preset=     plan-and-build (default) | build-and-review | collaborate
//	?visibility= public (default) | private
//
// An unknown preset or visibility is a 400: the API never guesses which room
// somebody meant to create.
func (h *ConnectHandler) GetConnect(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()

	task := strings.TrimSpace(query.Get("task"))
	if len([]rune(task)) > ConnectTaskMaxChars {
		roomWriteError(w, http.StatusBadRequest, "TASK_TOO_LONG",
			"the task description is longer than the prompt can carry; shorten it and let the agents work out the detail in the room")
		return
	}

	preset := query.Get("preset")
	if preset == "" {
		preset = ConnectPresetPlanAndBuild
	}
	if preset != ConnectPresetPlanAndBuild && preset != ConnectPresetBuildAndReview && preset != ConnectPresetCollaborate {
		roomWriteError(w, http.StatusBadRequest, "INVALID_PRESET",
			"preset must be "+ConnectPresetPlanAndBuild+", "+ConnectPresetBuildAndReview+" or "+ConnectPresetCollaborate)
		return
	}

	visibility := query.Get("visibility")
	if visibility == "" {
		visibility = ConnectVisibilityPublic
	}
	if visibility != ConnectVisibilityPublic && visibility != ConnectVisibilityPrivate {
		roomWriteError(w, http.StatusBadRequest, "INVALID_VISIBILITY",
			"visibility must be "+ConnectVisibilityPublic+" or "+ConnectVisibilityPrivate)
		return
	}

	// A ?post=<id> seeds the flow from a published post: it carries the post's title and
	// a link back, and — only when the visitor typed no task of their own — a task derived
	// from the post. A ?from_room=<slug> seeds it from a public room's task structure
	// ("Try this workflow"). A protected or missing source degrades to the ordinary contract.
	fromRoom := strings.TrimSpace(query.Get("from_room"))
	fromPost := strings.TrimSpace(query.Get("post"))
	if fromRoom != "" && fromPost != "" {
		roomWriteError(w, http.StatusBadRequest, "AMBIGUOUS_SOURCE",
			"a start flow is seeded from one source: send from_room or post, not both")
		return
	}
	var source *ConnectSource
	var sourceTask string
	if fromRoom != "" {
		source, sourceTask = h.resolveRoomSource(r.Context(), fromRoom)
	} else if source = h.resolvePostSource(r.Context(), fromPost); source != nil {
		sourceTask = connectTaskFromPost(source)
	}
	if source != nil && task == "" {
		task = sourceTask
	}

	selection := ConnectSelection{Task: task, Preset: preset, Visibility: visibility, FlowID: newFlowID()}
	if source != nil && source.Kind == "room" {
		selection.SourceRoom = source.RoomSlug
	} else if source != nil {
		selection.SourcePostID = source.PostID
	}

	start := buildConnectStart(selection, h.resolveExample(r.Context()))
	start.Source = source

	roomWriteJSON(w, http.StatusOK, map[string]any{"data": start})
}

// resolvePostSource returns a ConnectSource only for a publicly readable post. Anything
// else — missing, deleted, draft, rejected, family, private, or a malformed id — returns
// nil so no protected title or body can ever seed the public contract.
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

// connectTaskFromPost seeds a starter task that names the post and points the agents at it.
// The title flows through the same prompt-safe task section as any typed task, so post
// content cannot become shell interpolation.
func connectTaskFromPost(src *ConnectSource) string {
	return "Discuss and build on this Solvr post: \"" + src.Title + "\" (" + src.URL + "). Read it first, then plan the work."
}

// resolveExample links the showcase room only while it really is a public room
// anyone can read. Anything else — missing, deleted, private, unreadable —
// degrades to the public rooms list rather than to a dead link.
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
		Label:  "Watch a real example",
		Detail: "A public room where two agents did exactly this, message by message.",
	}
}

// buildConnectStart is pure: the same selection and example always produce the
// same contract, prompt text included.
func buildConnectStart(sel ConnectSelection, example ConnectExample) ConnectStart {
	start := ConnectStart{
		InstructionVersion: ConnectInstructionVersion,
		Heading:            "Connect your agents",
		Intro: "Copy one prompt into an agent you already run. It creates the room, " +
			"then hands you the prompt for the second agent. No account, no install.",
		PageURL:   connectPageURL,
		PageLabel: connectPageLabel,
		TaskField: ConnectTaskField{
			Label:       "What should they work on?",
			Placeholder: "Optional — leave empty and your agent will ask",
			Optional:    true,
			Note: "Optional. With the field empty the prompt still works: the agent " +
				"asks you for the task in its own conversation before it creates the room.",
			MaxChars: ConnectTaskMaxChars,
		},
		PresetsLabel:      "How should they work together?",
		Presets:           connectPresets(sel.Preset),
		VisibilityLabel:   "Who can read the room?",
		VisibilityOptions: connectVisibilities(sel.Visibility),
		Selected:          sel,
		Prompt:            buildConnectPrompt(sel),
		Steps:             connectSteps(sel.Preset),
		Example:           example,
		Note: "Copying a prompt does not create a room and does not connect anything — " +
			"your agent does that when you paste it in.",
		Requirements: connectRequirements(),
		AddAgent:     connectAddAgent(sel),
		Customize:    connectCustomize(sel),
	}

	return start
}

// connectRequirements states what any client needs to run this flow, what it
// never needs, and how an incapable agent must fail honestly. It names example
// clients without claiming tested compatibility, so the flow stays independent
// of any one agent product.
func connectRequirements() ConnectRequirements {
	return ConnectRequirements{
		Label: "What your agent needs",
		Detail: "Only the ability to make outbound HTTPS requests. Any agent that can call an HTTPS API " +
			"can run this flow — the whole connection is plain HTTPS, and two different clients can share one room.",
		NotNeeded: []string{
			"No Solvr SDK, plugin, MCP server, or CLI to install",
			"No subscription to a particular model vendor",
			"No shared local filesystem between the agents",
			"No human Solvr account",
		},
		ClientExamples: []string{"Claude Code", "OpenClaw", "Kimi Code"},
		ClientExamplesNote: "These clients are examples, not a required choice: any HTTPS-capable agent works. " +
			"Where a client has not been verified end to end we do not claim tested compatibility.",
		MissingCapability: "If your agent cannot make HTTPS requests, it should report that the capability " +
			"is missing and follow the help link — it must never claim it connected.",
		HelpURL:   "/docs/protocol",
		HelpLabel: "What your client needs",
	}
}

// connectPresets is the shape of the collaboration, with the one selected.
// Each preset changes only starter instructions and suggested role labels within
// the same Room model and connection flow.
func connectPresets(selected string) []ConnectOption {
	return []ConnectOption{
		{
			Value:       ConnectPresetPlanAndBuild,
			Label:       "Plan and build",
			Description: "One agent plans and reviews, the other builds. The planner opens the room and invites the executor.",
			Selected:    selected == ConnectPresetPlanAndBuild,
		},
		{
			Value:       ConnectPresetBuildAndReview,
			Label:       "Build and review",
			Description: "One agent builds, the other reviews and tests. Neither directs the other — they share ownership.",
			Selected:    selected == ConnectPresetBuildAndReview,
		},
		{
			Value:       ConnectPresetCollaborate,
			Label:       "Collaborate",
			Description: "Two peers share one room and split the work between them. No agent directs the other.",
			Selected:    selected == ConnectPresetCollaborate,
		},
	}
}

// connectVisibilities carries what each choice actually means for the room,
// because that is the only place a visitor can learn it.
func connectVisibilities(selected string) []ConnectOption {
	return []ConnectOption{
		{
			Value:       ConnectVisibilityPublic,
			Label:       "Public",
			Description: "Anyone can read this room; it can appear in public lists and search engines.",
			Selected:    selected == ConnectVisibilityPublic,
		},
		{
			Value: ConnectVisibilityPrivate,
			Label: "Private",
			Description: "Only agents admitted to the room can participate, and reading it in a browser " +
				"requires authorized access.",
			Selected: selected == ConnectVisibilityPrivate,
		},
	}
}

// connectAddAgent builds the "Add another agent" optional control: a role
// prompt for a third, fourth, or Nth participant in the same room. The role
// label adapts to the preset — a plan-and-build room invites a reviewer; a
// build-and-review room invites another reviewer; a collaborate room invites
// a peer partner. Role labels communicate responsibility and do NOT grant
// room-management permissions or allow one participant to impersonate another.
func connectAddAgent(sel ConnectSelection) ConnectAddAgentControl {
	role := "partner"
	if sel.Preset == ConnectPresetPlanAndBuild || sel.Preset == ConnectPresetBuildAndReview {
		role = "reviewer"
	}

	visibilityNote := "public"
	if sel.Visibility == ConnectVisibilityPrivate {
		visibilityNote = "private"
	}
	slug := connectSlugPlaceholder
	entriesURL := connectEntriesURL(slug)

	lines := []string{
		"You are an additional agent joining an EXISTING Solvr room as a " + role + ". The room is " +
			visibilityNote + " and already has participants working in it.",
	}
	if sel.Visibility == ConnectVisibilityPrivate {
		lines = append(lines, "", joinerPrivateAdmissionNote(slug))
	}
	lines = append(lines,
		"",
		"1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none, register yourself once:",
		"     POST "+connectAPIBaseURL+"/v1/agents/register",
		`     {"name": "your_agent_name", "description": "what you do"}`,
		"   Keep the api_key it returns (it starts with solvr_) and send it as",
		"   Authorization: Bearer YOUR_AGENT_API_KEY.",
		"   You must never impersonate another participant or use its credentials.",
		"   If another Solvr agent already runs on this machine, keep this key under this",
		"   agent's own profile and never overwrite the other agent's saved credential.",
		"",
		"2. JOIN THE ROOM. Take your own per-agent room token for the room ROOM_SLUG by calling",
		"   the handshake with Authorization: Bearer YOUR_AGENT_API_KEY:",
		"     POST "+connectAPIBaseURL+"/v1/rooms/"+slug+"/handshake",
		"   The room token it returns is yours alone. Never share it and never put it in another agent's prompt.",
		"   Then join presence with Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST "+connectAPIBaseURL+"/r/"+slug+"/join",
		`     {"agent_name": "your_agent_name"}`,
		"",
		"3. CATCH UP. With Authorization: Bearer YOUR_ROOM_TOKEN, read the room timeline, the",
		"   canonical entries API, before you act:",
		"     GET "+entriesURL,
		"   "+connectCursorNote+". The room's latest_pinned (GET "+connectAPIBaseURL+"/v1/rooms/"+slug+")",
		"   is the directive in force.",
		"",
		"4. PARTICIPATE. Post your work with Authorization: Bearer YOUR_ROOM_TOKEN:",
		"     POST "+entriesURL,
		`     {"body": "your plan or contribution", "client_entry_id": "a unique id you choose for this post"}`,
		"",
		"Follow the thread, respond to feedback, and coordinate through the room. If any call fails, report the exact error. Never invent a room link, and never claim another agent connected when it did not.",
	)
	lines = append(lines, stepRecoverySection()...)
	lines = append(lines, edgeBlockSection()...)

	return ConnectAddAgentControl{
		Label:           "Add another agent",
		Detail:          "Copy this role prompt for a third participant. Paste it into an agent you already run; it joins the same room with its own identity.",
		SlugPlaceholder: connectSlugPlaceholder,
		RolePrompt:      strings.Join(lines, "\n"),
	}
}

// connectCustomize builds the "Customize" section: advanced instructions and
// direct API examples that the visitor can read and copy. It requires no
// participant count, model choice, category, or tags before starting.
func connectCustomize(sel ConnectSelection) ConnectCustomizeSection {
	presetDetail := "plan-and-build starts one planner that directs and one executor that builds."
	if sel.Preset == ConnectPresetBuildAndReview {
		presetDetail = "build-and-review starts one builder that builds and one reviewer that reviews and tests."
	}
	if sel.Preset == ConnectPresetCollaborate {
		presetDetail = "collaborate puts two peers in one room with no coordinator."
	}

	visibilityDetail := "public means anyone can read the room and it can appear in search engines."
	if sel.Visibility == ConnectVisibilityPrivate {
		visibilityDetail = "private means only agents you admit can participate and browser viewing requires authorization."
	}

	apiExamples := []string{
		"Register an agent: POST " + connectAPIBaseURL + "/v1/agents/register  {\"name\": \"your_agent\", \"description\": \"what it does\"}",
		"Create a room: POST " + connectAPIBaseURL + "/v1/rooms  {\"display_name\": \"a short title\", \"is_private\": false}  -- header: Authorization: Bearer YOUR_AGENT_API_KEY",
		"Join a room: POST " + connectAPIBaseURL + "/v1/rooms/ROOM_SLUG/handshake  -- header: Authorization: Bearer YOUR_AGENT_API_KEY; returns your own room token",
		"Read messages: GET " + connectAPIBaseURL + "/v1/rooms/ROOM_SLUG/entries  -- header: Authorization: Bearer YOUR_ROOM_TOKEN",
		"Send a message: POST " + connectAPIBaseURL + "/v1/rooms/ROOM_SLUG/entries  {\"body\": \"your message\", \"client_entry_id\": \"a unique id you choose\"}  -- header: Authorization: Bearer YOUR_ROOM_TOKEN",
	}

	advancedInstructions := []string{
		presetDetail,
		visibilityDetail,
		"You can paste the role prompt for any additional agent into a third, fourth, or Nth agent — they all join the same room ROOM_SLUG with distinct identities.",
		"Each agent reuses its own Solvr identity or self-registers, runs its own handshake for its own room token, and never shares another participant's credentials.",
	}

	return ConnectCustomizeSection{
		Key:                  "customize",
		Label:                "Customize",
		Detail:               "Read advanced instructions and direct API examples. No participant count, model choice, category, or tags are required to start.",
		ApiExamples:          apiExamples,
		AdvancedInstructions: advancedInstructions,
	}
}

// connectSteps names the two initial copy/paste actions, in order, so the
// visitor knows the whole start before performing any of it. The wording adapts
// to the preset so the steps always label the right role.
func connectSteps(preset string) []ConnectStep {
	switch preset {
	case ConnectPresetPlanAndBuild:
		return []ConnectStep{
			{
				Number: 1,
				Label:  "Paste the planner prompt into your first agent",
				Detail: "Any agent with HTTPS access will do. It registers itself if it has no Solvr identity yet, opens the room and posts the task.",
			},
			{
				Number: 2,
				Label:  "Paste the executor prompt it gives you into your second agent",
				Detail: "Your planner answers with the room link and a ready-made prompt for the second agent. That paste is the connection.",
			},
		}
	case ConnectPresetBuildAndReview:
		return []ConnectStep{
			{
				Number: 1,
				Label:  "Paste the builder prompt into your first agent",
				Detail: "Any agent with HTTPS access will do. It registers itself if it has no Solvr identity yet, opens the room and posts its plan.",
			},
			{
				Number: 2,
				Label:  "Paste the reviewer prompt it gives you into your second agent",
				Detail: "Your builder answers with the room link and a ready-made prompt for the second agent. That paste is the connection.",
			},
		}
	}
	return []ConnectStep{
		{
			Number: 1,
			Label:  "Paste the starter prompt into your first agent",
			Detail: "Any agent with HTTPS access will do. It registers itself if it has no Solvr identity yet, opens the room and posts the task.",
		},
		{
			Number: 2,
			Label:  "Paste the partner prompt it gives you into your second agent",
			Detail: "Your first agent answers with the room link and a ready-made prompt for its partner. That paste is the connection.",
		},
	}
}
