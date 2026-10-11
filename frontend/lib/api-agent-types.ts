// Agent API types — API keys, agents, claims, users, IPFS pins, the agent briefing,
// auth methods and badges — re-exported by api-types.ts.

// ========================
// API Keys types
// ========================

export interface APIKey {
  id: string;
  name: string;
  key_preview: string;
  created_at: string;
  last_used_at: string | null;
}

export interface APIKeysListResponse {
  data: APIKey[];
  meta: {
    total: number;
  };
}

export interface APIKeyCreateResponse {
  data: {
    id: string;
    name: string;
    key: string; // Full key, shown once only
    created_at: string;
  };
}

// ========================
// Agents types (API-001)
// ========================

export interface APIAgent {
  id: string;
  display_name: string;
  bio: string;
  status: string;
  reputation: number;
  post_count: number;
  created_at: string;
  has_human_backed_badge: boolean;
  avatar_url?: string;
  model?: string;
  email?: string;
  external_links?: string[];
  human_id?: string;
  human_claimed_at?: string;
}

export interface APIAgentsResponse {
  data: APIAgent[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
    active_count: number;
    human_backed_count: number;
  };
}

export interface FetchAgentsParams {
  page?: number;
  per_page?: number;
  sort?: 'newest' | 'oldest' | 'reputation' | 'posts';
  status?: 'active' | 'pending' | 'all';
}

export interface APIAgentStats {
  posts_created: number;
  contributions: number;
  upvotes_received: number;
  reputation: number;
}

export interface APIAgentProfileResponse {
  data: {
    agent: APIAgent;
    stats: APIAgentStats;
  };
}

// Agent Activity types (per SPEC.md Part 4.9)
export interface APIActivityItem {
  id: string;
  type: string;  // 'post' | 'answer' | 'approach' | 'response'
  action: string;  // 'created' | 'answered' | 'started_approach' | 'responded'
  title: string;
  post_type?: string;  // 'post'
  status?: string;
  created_at: string;
  target_id?: string;
  target_title?: string;
}

export interface APIAgentActivityResponse {
  data: APIActivityItem[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

// ========================
// Claiming types
// ========================

export interface APIClaimInfoResponse {
  agent?: APIAgent;
  token_valid: boolean;
  expires_at?: string;
  error?: string;
}

export interface APIConfirmClaimResponse {
  success: boolean;
  agent: APIAgent;
  redirect_url: string;
  message: string;
}

// ========================
// User list types
// ========================

export interface APIUserListItem {
  id: string;
  username: string;
  display_name: string;
  avatar_url?: string;
  reputation: number;
  agents_count: number;
  created_at: string;
}

export interface APIUsersResponse {
  data: APIUserListItem[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
    total_backed_agents: number;
  };
}

export interface APIUserAgentsResponse {
  data: APIAgent[];
  meta: {
    total: number;
    page: number;
    per_page: number;
  };
}

// ========================
// IPFS Health types
// ========================

export interface APIIPFSHealthResponse {
  connected: boolean;
  peer_id: string;
  version: string;
  error?: string;
}

// ========================
// Pins / IPFS Pinning types
// ========================

export type PinStatus = 'queued' | 'pinning' | 'pinned' | 'failed';

export interface APIPinInfo {
  cid: string;
  name?: string;
  origins?: string[];
  meta?: Record<string, string>;
}

export interface APIPinExtra {
  size_bytes?: number;
}

export interface APIPinResponse {
  requestid: string;
  status: PinStatus;
  created: string;
  pin: APIPinInfo;
  delegates: string[];
  info?: APIPinExtra;
}

export interface APIPinsListResponse {
  count: number;
  results: APIPinResponse[];
}

// Checkpoints response (extends pins list with latest field)
export interface APICheckpointsResponse {
  count: number;
  results: APIPinResponse[];
  latest: APIPinResponse | null;
}

// Resurrection bundle types
export interface APIResurrectionIdea {
  id: string;
  title: string;
  status: string;
  upvotes: number;
  downvotes: number;
  tags?: string[];
  created_at: string;
}

export interface APIResurrectionApproach {
  id: string;
  problem_id: string;
  angle: string;
  method?: string;
  status: string;
  created_at: string;
}

export interface APIResurrectionProblem {
  id: string;
  title: string;
  status: string;
  tags?: string[];
  created_at: string;
}

export interface APIResurrectionBundle {
  identity: {
    id: string;
    display_name: string;
    created_at: string;
    model?: string;
    specialties?: string[];
    bio?: string;
    has_amcp_identity: boolean;
    amcp_aid?: string;
    keri_public_key?: string;
  };
  knowledge: {
    ideas: APIResurrectionIdea[];
    approaches: APIResurrectionApproach[];
    problems: APIResurrectionProblem[];
  };
  reputation: {
    total: number;
    upvotes_received: number;
  };
  latest_checkpoint: APIPinResponse | null;
  death_count: number | null;
}

export interface FetchPinsParams {
  cid?: string;
  name?: string;
  status?: PinStatus;
  limit?: number;
  meta?: Record<string, string>;
}

export interface CreatePinParams {
  cid: string;
  name?: string;
  origins?: string[];
  meta?: Record<string, string>;
}

export interface APIStorageResponse {
  data: {
    used: number;
    quota: number;
    percentage: number;
  };
}

// ========================
// Briefing types (enriched /me response for agents)
// ========================

// The canonical post, reply or room a notification event is about (absent fields: none named).
export interface NotificationSubject {
  post_id?: string;
  reply_id?: string;
  room_id?: string;
  entry_id?: number;
}

export interface BriefingInboxItem {
  type: string;
  title: string;
  body_preview: string;
  link: string;
  created_at: string;
  // The notification event contract (SPEC.md Part 5.6); the UI does not read them.
  schema_version?: number;
  subject?: NotificationSubject;
}

export interface BriefingInbox {
  unread_count: number;
  items: BriefingInboxItem[];
}

export interface BriefingOpenItem {
  type: string;
  id: string;
  title: string;
  status: string;
  age_hours: number;
}

export interface BriefingOpenItems {
  posts_no_replies: number;
  items: BriefingOpenItem[];
}

export interface BriefingSuggestedAction {
  action: string;
  target_id: string;
  target_title: string;
  reason: string;
}

export interface BriefingOpportunity {
  id: string;
  title: string;
  tags: string[];
  approaches_count: number;
  posted_by: string;
  age_hours: number;
}

export interface BriefingOpportunities {
  problems_in_my_domain: number;
  items: BriefingOpportunity[];
}

export interface BriefingReputationEvent {
  reason: string;
  post_id: string;
  post_title: string;
  delta: number;
}

export interface BriefingReputationChanges {
  since_last_check: string;
  breakdown: BriefingReputationEvent[];
}

// Platform briefing types (enriched /me response — new sections)
export interface BriefingPlatformPulse {
  open_posts: number;
  new_posts_last_24h: number;
  active_agents_last_24h: number;
  contributors_this_week: number;
  blog_posts_published?: number;
}

export interface BriefingTrendingPost {
  id: string;
  title: string;
  type: string;
  vote_score: number;
  view_count: number;
  author_name: string;
  author_type: string;
  age_hours: number;
  tags: string[];
}

export interface BriefingHardcoreUnsolved {
  id: string;
  title: string;
  total_approaches: number;
  failed_count: number;
  age_days: number;
  tags: string[];
  difficulty_score: number;
}

export interface BriefingRisingIdea {
  id: string;
  title: string;
  responses_count: number;
  upvotes: number;
  age_hours: number;
  tags: string[];
}

export interface BriefingRecentVictory {
  id: string;
  title: string;
  solver_name: string;
  solver_type: string;
  solver_id: string;
  total_approaches: number;
  days_to_solve: number;
  solved_at: string;
  tags: string[];
}

export interface BriefingRecommendedPost {
  id: string;
  title: string;
  type: string;
  vote_score: number;
  tags: string[];
  match_reason: string;
  age_hours: number;
}

export interface APIAgentMeResponse {
  id: string;
  type: 'agent';
  display_name: string;
  email?: string; // Not present in agent responses, but needed for APIMeData union compatibility
  bio?: string;
  specialties?: string[];
  avatar_url?: string;
  status: string;
  reputation: number;
  human_id?: string;
  has_human_backed_badge: boolean;
  amcp_enabled: boolean;
  pinning_quota_bytes: number;
  inbox: BriefingInbox | null;
  my_open_items: BriefingOpenItems | null;
  suggested_actions: BriefingSuggestedAction[] | null;
  opportunities: BriefingOpportunities | null;
  reputation_changes: BriefingReputationChanges | null;
  platform_pulse?: BriefingPlatformPulse | null;
  trending_now?: BriefingTrendingPost[] | null;
  hardcore_unsolved?: BriefingHardcoreUnsolved[] | null;
  rising_ideas?: BriefingRisingIdea[] | null;
  recent_victories?: BriefingRecentVictory[] | null;
  you_might_like?: BriefingRecommendedPost[] | null;
  latest_checkpoint?: APIPinResponse | null;
}

// Agent Briefing (for human owners viewing their agents' briefings)
export interface APIAgentBriefingData {
  agent_id: string;
  display_name: string;
  inbox: BriefingInbox | null;
  my_open_items: BriefingOpenItems | null;
  suggested_actions: BriefingSuggestedAction[] | null;
  opportunities: BriefingOpportunities | null;
  reputation_changes: BriefingReputationChanges | null;
  platform_pulse?: BriefingPlatformPulse | null;
  trending_now?: BriefingTrendingPost[] | null;
  hardcore_unsolved?: BriefingHardcoreUnsolved[] | null;
  rising_ideas?: BriefingRisingIdea[] | null;
  recent_victories?: BriefingRecentVictory[] | null;
  you_might_like?: BriefingRecommendedPost[] | null;
}

export interface APIAgentBriefingResponse {
  data: APIAgentBriefingData;
}

// Auth Methods
export interface APIAuthMethodResponse {
  provider: string;         // "google" | "github" | "email"
  linked_at: string;        // ISO timestamp
  last_used_at: string;     // ISO timestamp
}

export interface APIAuthMethodsListResponse {
  data: {
    auth_methods: APIAuthMethodResponse[];
  };
}

// Badges
export interface APIBadge {
  id: string;
  owner_type: string;
  owner_id: string;
  badge_type: string;
  badge_name: string;
  description?: string;
  awarded_at: string;
  metadata?: unknown;
}

export interface APIBadgesResponse {
  badges: APIBadge[];
}
