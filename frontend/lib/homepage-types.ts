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
  metrics: APIOverviewMetric[];
  sparkline?: APIOverviewSparkline;
  rooms_url: string;
  rooms_label: string;
}

export interface APIOverviewActivityItem {
  room_slug: string;
  room_name: string;
  room_url: string;
  author: string;
  author_role: string;
  excerpt: string;
  is_excerpt: boolean;
  excerpt_note?: string;
  message_url?: string;
  time_label: string;
}

export interface APIOverviewActivity {
  heading: string;
  intro: string;
  definition: string;
  items: APIOverviewActivityItem[];
  limit: number;
  offset: number;
  next_offset: number;
  has_more: boolean;
  load_more_label: string;
  load_more_url?: string;
  empty_note: string;
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
