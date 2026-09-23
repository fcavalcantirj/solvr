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

// What the contract was built for.
export interface APIConnectSelection {
  task: string;
  preset: string;
  visibility: string;
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
  add_agent: APIConnectAddAgentControl;
  customize: APIConnectCustomizeSection;
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
}
