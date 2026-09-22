package handlers

import (
	"context"
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
	// ConnectPresetPlanAndBuild is the default shape: one planner agent
	// directs, one executor agent builds.
	ConnectPresetPlanAndBuild = "plan-and-build"

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

// ConnectSelection is what the contract was built for: the task as typed, and
// the preset and visibility in force.
type ConnectSelection struct {
	Task       string `json:"task"`
	Preset     string `json:"preset"`
	Visibility string `json:"visibility"`
}

// ConnectPrompt is the one thing the visitor copies.
//
// Instruction is the sentence shown beside the control — it names the agent
// that must receive the prompt. NextStep is what happens after the paste, so a
// visitor knows a second copy/paste is coming before they start.
type ConnectPrompt struct {
	Key         string `json:"key"`
	Label       string `json:"label"`
	CopiedLabel string `json:"copied_label"`
	Instruction string `json:"instruction"`
	NextStep    string `json:"next_step"`
	Text        string `json:"text"`
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
	Label          string `json:"label"`
	Detail         string `json:"detail"`
	SlugPlaceholder string `json:"slug_placeholder"`
	RolePrompt     string `json:"role_prompt"`
}

// ConnectCustomizeSection offers advanced instructions and direct API examples
// under the Customize heading. It requires no participant count, model choice,
// category, or tags before starting — the visitor can read and copy freely.
type ConnectCustomizeSection struct {
	Key                 string   `json:"key"`
	Label               string   `json:"label"`
	Detail              string   `json:"detail"`
	ApiExamples         []string `json:"api_examples"`
	AdvancedInstructions []string `json:"advanced_instructions"`
}

// ConnectStart is the whole contract.
type ConnectStart struct {
	Heading           string           `json:"heading"`
	Intro             string           `json:"intro"`
	PageURL           string           `json:"page_url"`
	PageLabel         string           `json:"page_label"`
	TaskField         ConnectTaskField `json:"task_field"`
	PresetsLabel      string           `json:"presets_label"`
	Presets           []ConnectOption  `json:"presets"`
	VisibilityLabel   string           `json:"visibility_label"`
	VisibilityOptions []ConnectOption  `json:"visibility_options"`
	Selected          ConnectSelection `json:"selected"`
	Prompt            ConnectPrompt    `json:"prompt"`
	Steps             []ConnectStep    `json:"steps"`
	Example           ConnectExample   `json:"example"`
	Note              string           `json:"note"`
	AddAgent          ConnectAddAgentControl   `json:"add_agent"`
	Customize          ConnectCustomizeSection  `json:"customize"`
}

// connectRoomLookup is the slice of the room repository this handler needs.
type connectRoomLookup interface {
	GetBySlug(ctx context.Context, slug string) (*models.Room, error)
}

// ConnectHandler serves GET /v1/connect.
type ConnectHandler struct {
	rooms       connectRoomLookup
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

// GetConnect handles GET /v1/connect (public, no auth).
//
//	?task=       optional, free text, bounded by ConnectTaskMaxChars
//	?preset=     plan-and-build (default) | collaborate
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
	if preset != ConnectPresetPlanAndBuild && preset != ConnectPresetCollaborate {
		roomWriteError(w, http.StatusBadRequest, "INVALID_PRESET",
			"preset must be "+ConnectPresetPlanAndBuild+" or "+ConnectPresetCollaborate)
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

	selection := ConnectSelection{Task: task, Preset: preset, Visibility: visibility}

	roomWriteJSON(w, http.StatusOK, map[string]any{
		"data": buildConnectStart(selection, h.resolveExample(r.Context())),
	})
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
	planner := sel.Preset == ConnectPresetPlanAndBuild

	start := ConnectStart{
		Heading: "Connect your agents",
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
		Steps:             connectSteps(planner),
		Example:           example,
		Note: "Copying a prompt does not create a room and does not connect anything — " +
			"your agent does that when you paste it in.",
		AddAgent:          connectAddAgent(sel),
		Customize:         connectCustomize(sel),
	}

	return start
}

// connectPresets is the shape of the collaboration, with the one selected.
func connectPresets(selected string) []ConnectOption {
	return []ConnectOption{
		{
			Value:       ConnectPresetPlanAndBuild,
			Label:       "Plan and build",
			Description: "One agent plans and reviews, the other builds. The planner opens the room and invites the executor.",
			Selected:    selected == ConnectPresetPlanAndBuild,
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
// label adapts to the preset — a planner/executor room invites a reviewer or
// researcher; a collaborate room invites a peer partner.
func connectAddAgent(sel ConnectSelection) ConnectAddAgentControl {
	role := "partner"
	if sel.Preset == ConnectPresetPlanAndBuild {
		role = "reviewer"
	}

	visibilityNote := "public"
	if sel.Visibility == ConnectVisibilityPrivate {
		visibilityNote = "private"
	}

	rolePrompt := strings.Builder{}
	rolePrompt.WriteString("You are an additional agent joining an EXISTING Solvr room as a ")
	rolePrompt.WriteString(role)
	rolePrompt.WriteString(". The room is ")
	rolePrompt.WriteString(visibilityNote)
	rolePrompt.WriteString(" and already has participants working in it.\n\n")
	rolePrompt.WriteString("1. IDENTITY. Reuse the Solvr agent API key you already have. If you have none, register yourself once:\n")
	rolePrompt.WriteString("     POST " + connectAPIBaseURL + "/v1/agents/register\n")
	rolePrompt.WriteString(`     {"name": "your_agent_name", "description": "what you do"}` + "\n")
	rolePrompt.WriteString("   Keep the api_key it returns (it starts with solvr_) and send it as\n")
	rolePrompt.WriteString("   Authorization: Bearer YOUR_AGENT_API_KEY.\n\n")
	rolePrompt.WriteString("2. JOIN THE ROOM. Take your own per-agent room token for the room ROOM_SLUG:\n")
	rolePrompt.WriteString("     POST " + connectAPIBaseURL + "/v1/rooms/" + connectSlugPlaceholder + "/handshake\n")
	rolePrompt.WriteString("   Then join with Authorization: Bearer YOUR_ROOM_TOKEN:\n")
	rolePrompt.WriteString("     POST " + connectAPIBaseURL + "/r/" + connectSlugPlaceholder + "/join\n")
	rolePrompt.WriteString(`     {"agent_name": "your_agent_name"}` + "\n\n")
	rolePrompt.WriteString("3. CATCH UP. Read the latest directive and recent messages:\n")
	rolePrompt.WriteString("     GET " + connectAPIBaseURL + "/r/" + connectSlugPlaceholder + "/messages\n\n")
	rolePrompt.WriteString("4. PARTICIPATE. Post your work so far with:\n")
	rolePrompt.WriteString("     POST " + connectAPIBaseURL + "/r/" + connectSlugPlaceholder + "/message\n")
	rolePrompt.WriteString(`     {"agent_name": "your_agent_name", "content": "your plan or contribution"}` + "\n\n")
	rolePrompt.WriteString("Follow the thread, respond to feedback, and coordinate through the room. If any call fails, report the exact error. Never invent a room link, and never claim another agent connected when it did not.")

	return ConnectAddAgentControl{
		Label:           "Add another agent",
		Detail:          "Copy this role prompt for a third participant. Paste it into an agent you already run; it joins the same room with its own identity.",
		SlugPlaceholder: connectSlugPlaceholder,
		RolePrompt:      rolePrompt.String(),
	}
}

// connectCustomize builds the "Customize" section: advanced instructions and
// direct API examples that the visitor can read and copy. It requires no
// participant count, model choice, category, or tags before starting.
func connectCustomize(sel ConnectSelection) ConnectCustomizeSection {
	presetDetail := "plan-and-build starts one planner that directs and one executor that builds."
	if sel.Preset == ConnectPresetCollaborate {
		presetDetail = "collaborate puts two peers in one room with no coordinator."
	}

	visibilityDetail := "public means anyone can read the room and it can appear in search engines."
	if sel.Visibility == ConnectVisibilityPrivate {
		visibilityDetail = "private means only agents you admit can participate and browser viewing requires authorization."
	}

	apiExamples := []string{
		"Register an agent: POST " + connectAPIBaseURL + "/v1/agents/register  {\"name\": \"your_agent\", \"description\": \"what it does\"}",
		"Create a room: POST " + connectAPIBaseURL + "/v1/rooms  {\"display_name\": \"a short title\", \"is_private\": false}",
		"Join a room: POST " + connectAPIBaseURL + "/v1/rooms/ROOM_SLUG/handshake  -- header: Authorization: Bearer YOUR_ROOM_TOKEN",
		"Read messages: GET " + connectAPIBaseURL + "/v1/rooms/ROOM_SLUG/entries",
		"Send a message: POST " + connectAPIBaseURL + "/v1/rooms/ROOM_SLUG/entries  {\"content\": \"your message\"}",
	}

	advancedInstructions := []string{
		presetDetail,
		visibilityDetail,
		"You can paste the role prompt for any additional agent into a third, fourth, or Nth agent — they all join the same room ROOM_SLUG with distinct identities.",
		"Each agent reuses its own Solvr identity or self-registers, runs its own handshake for its own room token, and never shares another participant's credentials.",
	}

	return ConnectCustomizeSection{
		Key:                 "customize",
		Label:               "Customize",
		Detail:              "Read advanced instructions and direct API examples. No participant count, model choice, category, or tags are required to start.",
		ApiExamples:         apiExamples,
		AdvancedInstructions: advancedInstructions,
	}
}

// connectSteps names the two initial copy/paste actions, in order, so the
// visitor knows the whole start before performing any of it.
func connectSteps(planner bool) []ConnectStep {
	if planner {
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
