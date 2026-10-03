// ---------------------------------------------------------------------------
// GET /v1/connect — the start-flow contract
//
// One API-owned payload behind every surface that starts a connection: the
// compact panel on the index and the full /connect page render THIS. The API
// decides the heading, the field labels, which presets and visibilities exist,
// which one is selected, what each choice means, what the copy control is
// called, where the copied prompt must be pasted, and the prompt itself,
// character for character. The browser types into a field and renders the
// answer; it never writes a prompt and never decides what public means.
// ---------------------------------------------------------------------------

// One choice the visitor can make, with the API's explanation of it.
export interface APIConnectOption {
  value: string;
  label: string;
  description: string;
  selected: boolean;
}

// The single optional input on the start flow.
export interface APIConnectTaskField {
  label: string;
  placeholder: string;
  optional: boolean;
  note: string;
  max_chars: number;
}

// What the contract was built for. `flow_id` is the non-secret connection-funnel
// identifier issued for this response: the browser reports its connection_started
// and starter_prompt_copied steps with it, and the same id is embedded in the
// copied prompt so the room's server steps join the same attempt.
export interface APIConnectSelection {
  task: string;
  preset: string;
  visibility: string;
  flow_id?: string;
  // The validated source this flow was seeded from (at most one), carried by the
  // prompt into the create-room body (idx 88).
  source_room?: string;
  source_post_id?: string;
}

// Where a start flow was seeded from: a public room ("Try this workflow") or a
// published post. Rendered as-is; the API decided it was public.
export interface APIConnectSource {
  kind: 'room' | 'post' | string;
  room_slug?: string;
  post_id?: string;
  title: string;
  url: string;
  detail: string;
}

// The public room or post a browser funnel step is attributed to. The API resolves
// it; an identifier that is not public is dropped there.
export interface APIFunnelSourceRef {
  kind: 'room' | 'post';
  ref: string;
}

// One connection-funnel step the browser reports to POST /v1/analytics/funnel.
// Only browser steps are ever sent from here; server steps are recorded server-side.
export interface APIFunnelEventInput {
  event: string;
  flow_id?: string;
  preset?: string;
  role?: string;
  entry_surface?: string;
  instruction_version?: string;
  source?: APIFunnelSourceRef;
}

// The one thing the visitor copies. `instruction` names the agent that must
// receive it; `next_step` says what comes back, so a visitor knows a second
// copy/paste is coming before they start. `copied_detail` is the confirmation
// shown only after a successful copy: it names the agent to paste into and what
// comes back next.
export interface APIConnectPrompt {
  key: string;
  label: string;
  copied_label: string;
  copied_detail: string;
  instruction: string;
  next_step: string;
  text: string;
}

// One of the two initial copy/paste actions.
export interface APIConnectStep {
  number: number;
  label: string;
  detail: string;
}

// The real collaboration a visitor can read before starting. `kind` is "real"
// (a public room that exists right now) or "directory" (no showcase room is
// available, so this opens the public rooms list).
export interface APIConnectExample {
  kind: string;
  url: string;
  label: string;
  detail: string;
}

// What any client needs to run the flow, and what it never needs. This is how
// the contract stays client-independent: the only capability required is
// outbound HTTPS; `client_examples` names products (Claude Code, OpenClaw, Kimi
// Code) as optional examples, never a required choice; `client_examples_note`
// declines to claim tested compatibility for unverified clients; and
// `missing_capability` + `help_url` give the honest failure for an agent that
// cannot make HTTPS requests instead of a fake "connected".
export interface APIConnectRequirements {
  label: string;
  detail: string;
  not_needed: string[];
  client_examples: string[];
  client_examples_note: string;
  missing_capability: string;
  help_url: string;
  help_label: string;
}

// The "Add another agent" optional control: a role-specific prompt the visitor
// can copy for a third, fourth, or Nth participant in the same room.
export interface APIConnectAddAgentControl {
  label: string;
  detail: string;
  slug_placeholder: string;
  role_prompt: string;
}

// The "Customize" section: advanced instructions and direct API examples.
export interface APIConnectCustomizeSection {
  key: string;
  label: string;
  detail: string;
  api_examples: string[];
  advanced_instructions: string[];
}

export interface APIConnectStart {
  instruction_version: string;
  heading: string;
  intro: string;
  page_url: string;
  page_label: string;
  task_field: APIConnectTaskField;
  presets_label: string;
  presets: APIConnectOption[];
  visibility_label: string;
  visibility_options: APIConnectOption[];
  selected: APIConnectSelection;
  prompt: APIConnectPrompt;
  steps: APIConnectStep[];
  example: APIConnectExample;
  note: string;
  requirements: APIConnectRequirements;
  add_agent: APIConnectAddAgentControl;
  customize: APIConnectCustomizeSection;
  source?: APIConnectSource;
}

export interface APIConnectStartResponse {
  data: APIConnectStart;
}

// What a surface may ask for. Everything is optional: with nothing set the API
// answers with its own defaults, which is the only place defaults live.
export interface ConnectStartParams {
  task?: string;
  preset?: string;
  visibility?: string;
  // The source the /connect page was linked with ("Try this workflow").
  from_room?: string;
  post?: string;
}

// GET /v1/rooms/{slug}/share — what a person may copy to share a public room. The API
// composes every link and the excerpt; Solvr never posts any of it anywhere.
export interface APIRoomShare {
  room_url: string;
  share_url: string;
  try_url: string;
  excerpt: { title: string; text: string; source: string };
  copy_text: string;
  note: string;
}

export interface APIRoomShareResponse {
  data: APIRoomShare;
}
