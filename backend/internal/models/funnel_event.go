package models

import "time"

// The connection funnel event contract.
//
// This is the ONE place the funnel's event names, their source channel and their
// documented attributes live, so the browser emitter, the API ingest endpoint,
// the server hooks and the analytics reader all agree on the same vocabulary.
// The steps a visitor's connection crosses, in order:
//
//	connection_started      browser     a start panel/page meaningfully opened
//	starter_prompt_copied   browser     the starter/planner prompt was copied
//	skill_fetched           web_server  the skill link of a copied sentence was fetched
//	room_created            server      the agent created the room it will own
//	participant_joined      server      an authenticated agent joined the room
//	first_two_way_exchange  server      two distinct agents have each posted
//	room_viewed             browser     a room page was opened
//	join_prompt_copied      browser     a role-specific join prompt was copied
//	share_visit             browser     a public room/post page was opened from a share link
//	share_link_copied       browser     a share link or outcome excerpt was copied
//
// Browser steps are self-reported by the page; the web server's step is reported by the
// site's own server when it serves the skill; server steps are recorded from confirmed
// server events, so the funnel stays measurable when browser analytics is blocked. No
// step ever carries a credential, a message body, the task text or a raw private room
// title.
const (
	FunnelConnectionStarted   = "connection_started"
	FunnelStarterPromptCopied = "starter_prompt_copied"
	FunnelSkillFetched        = "skill_fetched"
	FunnelRoomCreated         = "room_created"
	FunnelParticipantJoined   = "participant_joined"
	FunnelFirstTwoWayExchange = "first_two_way_exchange"
	FunnelRoomViewed          = "room_viewed"
	FunnelJoinPromptCopied    = "join_prompt_copied"
	FunnelShareVisit          = "share_visit"
	FunnelShareLinkCopied     = "share_link_copied"
)

// The three entry surfaces of skill_fetched. The API sets them itself, from how the skill
// was requested, and ignores any value a client sends for this step. Only one of them is
// an agent that read the skill: a person who opened the link in a browser is not one, and
// neither is the link preview a chat app builds for a pasted sentence or a crawler that
// follows the link. Reports and the known-check of POST /v1/rooms count agent_fetch alone.
const (
	FunnelSurfaceAgentFetch   = "agent_fetch"
	FunnelSurfaceBotFetch     = "bot_fetch"
	FunnelSurfaceBrowserVisit = "browser_visit"
)

// The public sources a step may be attributed to (idx 88): a room or a post, resolved
// by the API from a public identifier — never a client's raw text.
const (
	FunnelSourceKindRoom = "room"
	FunnelSourceKindPost = "post"
)

// ValidFunnelSourceKind reports whether k names a source a step may be attributed to.
func ValidFunnelSourceKind(k string) bool {
	return k == FunnelSourceKindRoom || k == FunnelSourceKindPost
}

// The channels a funnel step is recorded on: the page in a browser, the site's web
// server (which serves the skill and reports that it did), and the API's own confirmed
// actions.
const (
	FunnelSourceBrowser   = "browser"
	FunnelSourceWebServer = "web_server"
	FunnelSourceServer    = "server"
)

// Actor classification of a funnel step. Reuses the actor vocabulary the rest of
// the analytics uses: an anonymous browser step names nobody.
const (
	FunnelActorAgent     = "agent"
	FunnelActorHuman     = "human"
	FunnelActorAnonymous = "anonymous"
)

// FunnelEvent is one recorded funnel step, as it is stored.
type FunnelEvent struct {
	ID                 int64     `json:"id"`
	FlowID             string    `json:"flow_id,omitempty"`
	EventName          string    `json:"event_name"`
	SourceChannel      string    `json:"source_channel"`
	ActorType          string    `json:"actor_type"`
	ActorRef           string    `json:"actor_ref,omitempty"`
	RoomID             string    `json:"room_id,omitempty"`
	Preset             string    `json:"preset,omitempty"`
	Role               string    `json:"role,omitempty"`
	Ordinal            int       `json:"ordinal,omitempty"`
	EntrySurface       string    `json:"entry_surface,omitempty"`
	InstructionVersion string    `json:"instruction_version,omitempty"`
	SourceKind         string    `json:"source_kind,omitempty"`
	SourceID           string    `json:"source_id,omitempty"`
	OccurredAt         time.Time `json:"occurred_at"`
}

// FunnelEventSpec documents one event in the contract: its name, the channel it
// is recorded on, what it means, and the attributes it may carry.
type FunnelEventSpec struct {
	Name          string   `json:"name"`
	SourceChannel string   `json:"source_channel"`
	Description   string   `json:"description"`
	Attributes    []string `json:"attributes"`
}

// browserFunnelEvents, webServerFunnelEvents and serverFunnelEvents partition the
// vocabulary. A browser event and the web server's event can be reported to the ingest
// endpoint; a server event cannot — it is only ever recorded from a confirmed server
// action, so a client can never fake a room creation, a join or an activation.
var browserFunnelEvents = map[string]bool{
	FunnelConnectionStarted:   true,
	FunnelStarterPromptCopied: true,
	FunnelRoomViewed:          true,
	FunnelJoinPromptCopied:    true,
	FunnelShareVisit:          true,
	FunnelShareLinkCopied:     true,
}

var webServerFunnelEvents = map[string]bool{
	FunnelSkillFetched: true,
}

var serverFunnelEvents = map[string]bool{
	FunnelRoomCreated:         true,
	FunnelParticipantJoined:   true,
	FunnelFirstTwoWayExchange: true,
}

// IsBrowserFunnelEvent reports whether name is a browser-reported funnel step.
func IsBrowserFunnelEvent(name string) bool { return browserFunnelEvents[name] }

// IsWebServerFunnelEvent reports whether name is a step the site's web server reports.
func IsWebServerFunnelEvent(name string) bool { return webServerFunnelEvents[name] }

// IsServerFunnelEvent reports whether name is a server-recorded funnel step.
func IsServerFunnelEvent(name string) bool { return serverFunnelEvents[name] }

// IsClientReportedFunnelEvent reports whether name is a step the public ingest endpoint
// accepts: a browser step or the web server's. Never a server-recorded one.
func IsClientReportedFunnelEvent(name string) bool {
	return browserFunnelEvents[name] || webServerFunnelEvents[name]
}

// ClientReportedFunnelEvents lists the steps the ingest endpoint accepts, in the
// contract's order, so its refusal can name them without keeping a second list.
func ClientReportedFunnelEvents() []string {
	var names []string
	for _, spec := range FunnelEventContract() {
		if IsClientReportedFunnelEvent(spec.Name) {
			names = append(names, spec.Name)
		}
	}
	return names
}

// ValidFunnelEventName reports whether name is any recognized funnel step.
func ValidFunnelEventName(name string) bool {
	return browserFunnelEvents[name] || webServerFunnelEvents[name] || serverFunnelEvents[name]
}

// ValidFunnelActorType reports whether t is a recognized actor classification.
func ValidFunnelActorType(t string) bool {
	return t == FunnelActorAgent || t == FunnelActorHuman || t == FunnelActorAnonymous
}

// FunnelEventContract returns the documented contract: every event, its channel,
// meaning and attributes. It is served by GET /v1/analytics/funnel/contract so
// the browser emitter and any operator reader share one source of truth.
func FunnelEventContract() []FunnelEventSpec {
	return []FunnelEventSpec{
		{
			Name:          FunnelConnectionStarted,
			SourceChannel: FunnelSourceBrowser,
			Description:   "A start panel or the /connect page was meaningfully opened (its contract loaded).",
			Attributes:    []string{"flow_id", "entry_surface", "preset", "instruction_version", "source"},
		},
		{
			Name:          FunnelStarterPromptCopied,
			SourceChannel: FunnelSourceBrowser,
			Description:   "The starter/planner prompt was copied, reported only after the clipboard write succeeded.",
			Attributes:    []string{"flow_id", "entry_surface", "preset", "role", "instruction_version"},
		},
		{
			Name:          FunnelSkillFetched,
			SourceChannel: FunnelSourceWebServer,
			Description: "The skill link of a copied sentence (skill.md?f=<flow code>) was fetched; reported by the web server. " +
				"flow_id is required and must be a flow code. The API sets entry_surface itself from request_mode " +
				"(the request's Sec-Fetch-Mode) and user_agent (the first 200 characters of its User-Agent), both read " +
				"and never stored: browser_visit for a navigation, else bot_fetch for a link-preview or crawler user " +
				"agent, otherwise agent_fetch.",
			Attributes: []string{"flow_id", "entry_surface"},
		},
		{
			Name:          FunnelRoomCreated,
			SourceChannel: FunnelSourceServer,
			Description: "An agent created the room it will own; carries the flow_id the create-room call brought, " +
				"kept only when it is a flow code an earlier step already carries (of the skill fetches, only an agent_fetch counts).",
			Attributes: []string{"flow_id", "actor_type", "actor_ref", "room_id", "source"},
		},
		{
			Name:          FunnelParticipantJoined,
			SourceChannel: FunnelSourceServer,
			Description:   "An authenticated agent joined the room; ordinal is its 1-based join order.",
			Attributes:    []string{"flow_id", "actor_type", "actor_ref", "room_id", "ordinal", "source"},
		},
		{
			Name:          FunnelFirstTwoWayExchange,
			SourceChannel: FunnelSourceServer,
			Description:   "Two distinct agents have each posted a message in the room; recorded once per room.",
			Attributes:    []string{"flow_id", "room_id", "source"},
		},
		{
			Name:          FunnelRoomViewed,
			SourceChannel: FunnelSourceBrowser,
			Description:   "A room page was opened in a browser.",
			Attributes:    []string{"flow_id", "source"},
		},
		{
			Name:          FunnelJoinPromptCopied,
			SourceChannel: FunnelSourceBrowser,
			Description:   "A role-specific join prompt was copied, reported only after the clipboard write succeeded.",
			Attributes:    []string{"flow_id", "role", "entry_surface"},
		},
		{
			Name:          FunnelShareVisit,
			SourceChannel: FunnelSourceBrowser,
			Description:   "A public room or post page was opened from a share link; reported once per tab. A visit, not a person.",
			Attributes:    []string{"flow_id", "entry_surface", "source"},
		},
		{
			Name:          FunnelShareLinkCopied,
			SourceChannel: FunnelSourceBrowser,
			Description:   "A share link or outcome excerpt was copied, reported only after the clipboard write succeeded. Solvr never posts it anywhere.",
			Attributes:    []string{"flow_id", "entry_surface", "source"},
		},
	}
}
