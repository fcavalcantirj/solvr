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
// The sentence as the API serves it (GET /v1/connect, /v1/connect/examples and
// /v1/rooms/{slug}/connect): its plain text, what Copy copies, and the same text cut
// into segments whose texts concatenate exactly to it. A page renders the segments by
// kind; it never composes a word of the sentence.
export type APIPromptSegmentKind = 'text' | 'link' | 'role' | 'intent' | 'visibility' | 'handoff';

export interface APIPromptSegment {
  kind: APIPromptSegmentKind;
  text: string;
  // role: 'a' receives this sentence, 'b' is the agent it hands off to.
  side?: 'a' | 'b';
  // intent: true when nothing was typed and the API filled its neutral phrase.
  empty?: boolean;
  // visibility: the room visibility this sentence asks for.
  value?: 'public' | 'private';
}

export interface APIPrompt {
  text: string;
  segments: APIPromptSegment[];
  word_count: number;
}

// One use case and its filled sentence.
export interface APIConnectPreset {
  value: string;
  label: string;
  selected?: boolean;
  // The one line on what happens after the copy.
  next: string;
  prompt: APIPrompt;
}

export interface APIConnectIntentField {
  label: string;
  placeholder: string;
  max_chars: number;
}

export interface APIConnectSelection {
  intent: string;
  preset: string;
  visibility: string;
  flow_id?: string;
  source_room?: string;
  source_post_id?: string;
}

export interface APIConnectSource {
  kind: 'room' | 'post' | string;
  room_slug?: string;
  post_id?: string;
  title: string;
  url: string;
  detail: string;
}

export interface APIFunnelSourceRef {
  kind: 'room' | 'post';
  ref: string;
}

export interface APIFunnelEventInput {
  event: string;
  flow_id?: string;
  preset?: string;
  role?: string;
  entry_surface?: string;
  instruction_version?: string;
  source?: APIFunnelSourceRef;
}

export interface APIConnectExample {
  kind: string;
  url: string;
  label: string;
  detail: string;
}

// The closing cell beside the use cases: they are examples, not a limit.
export interface APIConnectMore {
  label: string;
  detail: string;
}

export interface APIConnectStart {
  instruction_version: string;
  heading: string;
  intent_field: APIConnectIntentField;
  selected: APIConnectSelection;
  // Every use case's sentence, filled with the same intent and visibility.
  presets: APIConnectPreset[];
  // The selected use case's sentence and line.
  prompt: APIPrompt;
  next: string;
  more: APIConnectMore;
  example: APIConnectExample;
  source?: APIConnectSource;
}

export interface APIConnectStartResponse {
  data: APIConnectStart;
}

export interface APIConnectExamples {
  instruction_version: string;
  presets: APIConnectPreset[];
}

export interface APIConnectExamplesResponse {
  data: APIConnectExamples;
}

export interface ConnectStartParams {
  intent?: string;
  preset?: string;
  visibility?: string;
  from_room?: string;
  post?: string;
}

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
