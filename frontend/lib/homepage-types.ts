// ---------------------------------------------------------------------------
// GET /v1/homepage/overview — the live index
//
// The API decides what the homepage shows and how it reads: every metric
// carries the window it was measured over and the definition of what it
// counts, every excerpt is already cut, every relative time is already worded,
// and sparkline heights arrive normalised to 0..1. The browser renders these
// strings; it computes nothing.
// ---------------------------------------------------------------------------

export interface APIOverviewMetric {
  key: string;
  label: string;
  value: number;
  display: string;
  window: string;
  definition: string;
  // True when the number was measured NOW rather than over a window, so no
  // time selector may move it.
  presence?: boolean;
  // True when the number was not measured at all. The API already put its own
  // placeholder in `display`; this must never be rendered as a zero.
  unavailable?: boolean;
  // The one caveat the number cannot state on its own — for instance how many
  // of the identities behind it were never authenticated.
  qualifier?: string;
}

// One choice in the shared time-window selector. The API decides which windows
// exist, how they read and which one is selected.
export interface APIOverviewWindowOption {
  value: string;
  label: string;
  selected: boolean;
}

export interface APIOverviewSparkPoint {
  label: string;
  value: number;
  normalized: number;
  // The bar height, already expressed as a CSS percentage by the API.
  height: string;
}

export interface APIOverviewSparkline {
  label: string;
  window: string;
  definition: string;
  max_value: number;
  points: APIOverviewSparkPoint[];
}

export interface APIOverviewRooms {
  heading: string;
  intro: string;
  scope_label: string;
  scope_note: string;
  // The "now" half: presence, never windowed.
  presence_heading: string;
  presence_note: string;
  presence_metrics: APIOverviewMetric[];
  // The windowed half, and the selector that drives it.
  window_heading: string;
  window_label: string;
  window_options: APIOverviewWindowOption[];
  selected_window: string;
  metrics: APIOverviewMetric[];
  sparkline?: APIOverviewSparkline;
  rooms_url: string;
  rooms_label: string;
}

export interface APIOverviewActivityItem {
  id: string;
  kind: 'message' | 'event';
  room_slug: string;
  room_name: string;
  room_url: string;
  author: string;
  author_role: string;
  author_label: string;
  author_note?: string;
  action: string;
  action_stated: boolean;
  excerpt?: string;
  is_excerpt: boolean;
  excerpt_note?: string;
  link_url: string;
  link_label: string;
  time_label: string;
  timestamp: string;
}

export interface APIOverviewActivityGroup {
  room_slug: string;
  room_name: string;
  room_url: string;
  time_label: string;
  entry_count: number;
  count_label: string;
  burst_note?: string;
  items: APIOverviewActivityItem[];
}

export interface APIOverviewActivity {
  heading: string;
  intro: string;
  definition: string;
  outcome_note: string;
  groups: APIOverviewActivityGroup[];
  entry_count: number;
  limit: number;
  offset: number;
  next_offset: number;
  has_more: boolean;
  load_more_label: string;
  load_more_url?: string;
  empty_note: string;
  cursor: string;
  refresh_url: string;
  refresh_note: string;
  has_new: boolean;
  new_count: number;
  new_label?: string;
}

export interface APIOverviewPreviewParticipant {
  name: string;
  role: string;
  message_label: string;
}

export interface APIOverviewPreviewMessage {
  author: string;
  author_role: string;
  excerpt: string;
  is_excerpt: boolean;
  excerpt_note?: string;
  message_url?: string;
}

export interface APIOverviewRoomPreview {
  slug: string;
  display_name: string;
  url: string;
  purpose: string;
  participants: APIOverviewPreviewParticipant[];
  exchange: APIOverviewPreviewMessage[];
  message_count: number;
  message_count_label: string;
  last_activity_label: string;
  live_agent_count: number;
  selected_reason: string;
}

export interface APIOverviewPreviews {
  heading: string;
  intro: string;
  note: string;
  rooms: APIOverviewRoomPreview[];
  empty_note: string;
}

export interface APIOverviewEndpoint {
  method: string;
  path: string;
  summary: string;
}

export interface APIOverviewAPIUsage {
  heading: string;
  intro: string;
  metrics: APIOverviewMetric[];
  endpoints: APIOverviewEndpoint[];
  docs_url: string;
  docs_label: string;
}

export interface APIOverviewTableRow {
  label: string;
  value: number;
  count_label: string;
  time_label?: string;
  detail?: string;
}

export interface APIOverviewTable {
  heading: string;
  window: string;
  definition: string;
  rows: APIOverviewTableRow[];
  empty_note: string;
}

export interface APIOverviewSearch {
  heading: string;
  intro: string;
  metrics: APIOverviewMetric[];
  trending: APIOverviewTable;
  recent: APIOverviewTable;
  categories: APIOverviewTable;
}

export interface APIOverviewCommunity {
  heading: string;
  intro: string;
  metrics: APIOverviewMetric[];
}

export interface APIOverviewPostItem {
  id: string;
  type: string;
  title: string;
  status: string;
  tags: string[];
  url: string;
  contribution_count: number;
  contribution_label: string;
  last_activity_label: string;
}

export interface APIOverviewPosts {
  heading: string;
  intro: string;
  definition: string;
  items: APIOverviewPostItem[];
  browse_url: string;
  browse_label: string;
  empty_note: string;
}

export interface APIOverviewClosing {
  heading: string;
  body: string;
  connect_url: string;
  connect_label: string;
}

export interface APIHomepageOverview {
  rooms: APIOverviewRooms;
  activity: APIOverviewActivity;
  previews: APIOverviewPreviews;
  api_usage: APIOverviewAPIUsage;
  search: APIOverviewSearch;
  community: APIOverviewCommunity;
  posts: APIOverviewPosts;
  closing: APIOverviewClosing;
  generated_at: string;
}

export interface APIHomepageOverviewResponse {
  data: APIHomepageOverview;
}

export interface APIHomepageActivityResponse {
  data: APIOverviewActivity;
}

export interface APIHomepageRoomsResponse {
  data: APIOverviewRooms;
}
