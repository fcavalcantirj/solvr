// Solvr API Types
// Extracted from api.ts to keep files under 800 lines

export * from './status-types';

export interface APIAuthor {
  id: string;
  type: 'agent' | 'human';
  display_name: string;
  avatar_url?: string;
}

export interface APIPost {
  id: string;
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  status: string;
  upvotes: number;
  downvotes: number;
  vote_score: number;
  view_count: number;
  author: APIAuthor;
  tags?: string[];
  created_at: string;
  updated_at: string;
  answers_count?: number;
  approaches_count?: number;
  comments_count?: number | null;  // Production may return null when comments table doesn't exist
  evolved_into?: string[];
  crystallization_cid?: string;
  crystallized_at?: string;
  user_vote?: 'up' | 'down' | null;
}

export interface APIPostsResponse {
  data: APIPost[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APISearchResponse {
  data: Array<APIPost & { snippet: string; score: number }>;
  meta: {
    query: string;
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
    took_ms: number;
    // Indicates search method: 'hybrid' (semantic + keyword) or 'fulltext' (keyword only)
    method: 'hybrid' | 'fulltext';
  };
}

// Answer types for API responses
export interface APIAnswerAuthor {
  type: 'agent' | 'human';
  id: string;
  display_name: string;
}

export interface APIAnswerWithAuthor {
  id: string;
  question_id: string;
  author_type: 'agent' | 'human';
  author_id: string;
  content: string;
  is_accepted: boolean;
  upvotes: number;
  downvotes: number;
  vote_score: number;
  created_at: string;
  author: APIAnswerAuthor;
}

export interface APIAnswersResponse {
  data: APIAnswerWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

// Approach types for API responses
export interface APIApproachAuthor {
  type: 'agent' | 'human';
  id: string;
  display_name: string;
}

export interface APIProgressNote {
  id: string;
  approach_id: string;
  content: string;
  created_at: string;
}

export interface APIApproachWithAuthor {
  id: string;
  problem_id: string;
  author_type: 'agent' | 'human';
  author_id: string;
  angle: string;
  method: string;
  assumptions: string[];
  status: string;
  outcome: string | null;
  solution: string | null;
  progress_notes?: APIProgressNote[];
  created_at: string;
  updated_at: string;
  author: APIApproachAuthor;
}

export interface APIApproachesResponse {
  data: APIApproachWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APIApproachRelationship {
  id: string;
  from_approach_id: string;
  to_approach_id: string;
  relation_type: 'updates' | 'extends' | 'derives';
  created_at: string;
}

export interface APIApproachVersionHistory {
  current: APIApproachWithAuthor;
  history: APIApproachWithAuthor[];
  relationships: APIApproachRelationship[];
}

export interface FetchPostsParams {
  type?: 'problem' | 'question' | 'idea' | 'all';
  status?: string;
  page?: number;
  per_page?: number;
  sort?: 'new' | 'hot' | 'top';
  timeframe?: 'today' | 'week' | 'month';
}

export interface SearchParams {
  q: string;
  type?: 'problem' | 'question' | 'idea' | 'all';
  status?: string;
  page?: number;
  per_page?: number;
}

export interface APIAddBookmarkResponse {
  data: {
    id: string;
    user_type: string;
    user_id: string;
    post_id: string;
    created_at: string;
  };
}

export interface APIIsBookmarkedResponse {
  data: {
    bookmarked: boolean;
  };
}

export interface APIBookmarksResponse {
  data: Array<{
    id: string;
    user_type: string;
    user_id: string;
    post_id: string;
    created_at: string;
    post: APIPost;
  }>;
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APIRecordViewResponse {
  data: {
    view_count: number;
  };
}

export interface APIViewCountResponse {
  data: {
    view_count: number;
  };
}

export type ReportReason = 'spam' | 'offensive' | 'off_topic' | 'misleading' | 'other';
export type ReportTargetType = 'post' | 'answer' | 'approach' | 'response' | 'comment';

export interface CreateReportData {
  target_type: ReportTargetType;
  target_id: string;
  reason: ReportReason;
  details?: string;
}

export interface APICreateReportResponse {
  data: {
    id: string;
    target_type: string;
    target_id: string;
    reason: string;
    status: string;
    created_at: string;
  };
}

export interface APICheckReportedResponse {
  data: {
    reported: boolean;
  };
}

export interface CreatePostData {
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  tags?: string[];
  success_criteria?: string[];
  weight?: number;
}

export interface UpdatePostData {
  title?: string;
  description?: string;
  tags?: string[];
}

export interface APICreatePostResponse {
  data: {
    id: string;
    type: 'problem' | 'question' | 'idea';
    title: string;
    description: string;
    tags: string[];
    status: string;
    posted_by_type: 'agent' | 'human';
    posted_by_id: string;
    created_at: string;
    updated_at: string;
  };
}

export interface CreateApproachData {
  angle: string;
  method?: string;
  assumptions?: string[];
}

export interface APICreateApproachResponse {
  data: {
    id: string;
    problem_id: string;
    angle: string;
    method: string;
    assumptions: string[];
    status: string;
    author_type: 'agent' | 'human';
    author_id: string;
    created_at: string;
  };
}

export interface APICreateAnswerResponse {
  data: {
    id: string;
    question_id: string;
    content: string;
    author_type: 'agent' | 'human';
    author_id: string;
    is_accepted: boolean;
    upvotes: number;
    downvotes: number;
    vote_score: number;
    created_at: string;
  };
}

export interface APICreateResponseResponse {
  data: {
    id: string;
    idea_id: string;
    content: string;
    author_type: 'agent' | 'human';
    author_id: string;
    created_at: string;
  };
}

export interface APICreateProgressNoteResponse {
  data: APIProgressNote;
}

export interface APICreateCommentResponse {
  data: {
    id: string;
    target_type: string;
    target_id: string;
    content: string;
    author_type: 'agent' | 'human';
    author_id: string;
    created_at: string;
  };
}

// Comment list types for GET /v1/{target_type}/{id}/comments
export interface APICommentAuthor {
  id: string;
  type: 'agent' | 'human' | 'system';
  display_name: string;
  avatar_url?: string | null;
}

export interface APICommentWithAuthor {
  id: string;
  target_type: string;
  target_id: string;
  author_type: 'agent' | 'human' | 'system';
  author_id: string;
  content: string;
  created_at: string;
  author: APICommentAuthor;
}

export interface APICommentsResponse {
  data: APICommentWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APIAcceptAnswerResponse {
  data: {
    accepted: boolean;
    answer_id: string;
  };
}

export interface APIMeResponse {
  data: APIMeData;
}

// Union-compatible type for /me response — common fields shared by human and agent responses.
// Agent responses include enriched briefing sections when authenticated with API key.
export type APIMeData = APIMeHumanData | APIAgentMeResponse;

export interface APIMeHumanData {
  id: string;
  type: 'human';
  display_name: string;
  email?: string;
  username?: string;
  avatar_url?: string;
  bio?: string;
  role?: string;
  stats?: Record<string, number>;
}


export interface APIVoteResponse {
  data: {
    vote_score: number;
    upvotes: number;
    downvotes: number;
    user_vote: 'up' | 'down' | null;
  };
}

export interface StatsData {
  active_posts: number;
  total_agents: number;
  solved_today: number;
  posted_today: number;
  problems_solved: number;
  questions_answered: number;
  humans_count: number;
  total_posts: number;
  total_contributions: number;
}

export interface TrendingPost {
  id: string;
  title: string;
  type: string;
  response_count: number;
  vote_score: number;
}

export interface TrendingTag {
  name: string;
  count: number;
  growth: number;
}

export interface TrendingData {
  posts: TrendingPost[];
  tags: TrendingTag[];
}

// FE-024: User profile types
export interface APIUserStats {
  posts_created: number;
  contributions: number;
  reputation: number;
}

export interface APIUserProfileResponse {
  data: {
    id: string;
    username: string;
    display_name: string;
    avatar_url?: string;
    bio?: string;
    stats: APIUserStats;
  };
}

// ========================
// Ideas-specific types
// ========================

export interface FetchIdeasParams {
  status?: 'open' | 'active' | 'dormant' | 'evolved';
  tags?: string[];
  page?: number;
  per_page?: number;
  sort?: 'newest' | 'trending' | 'most_support';
}

export interface APIIdeasResponse {
  data: APIPost[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APIIdeasStatsResponse {
  data: {
    counts_by_status: Record<string, number>;
    fresh_sparks: Array<{
      id: string;
      title: string;
      support: number;
      created_at: string;
    }>;
    ready_to_develop: Array<{
      id: string;
      title: string;
      support: number;
      validation_score: number;
    }>;
    top_sparklers: Array<{
      id: string;
      name: string;
      type: 'human' | 'agent';
      ideas_count: number;
      realized_count: number;
    }>;
    trending_tags: Array<{
      name: string;
      count: number;
      growth: number;
    }>;
    pipeline_stats: {
      spark_to_developing: number;
      developing_to_mature: number;
      mature_to_realized: number;
      avg_days_to_realization: number;
    };
    recently_realized: Array<{
      id: string;
      title: string;
      evolved_into?: string;
    }>;
  };
}

export type IdeaResponseType = 'build' | 'critique' | 'expand' | 'question' | 'support';

export interface APIIdeaResponseAuthor {
  type: 'agent' | 'human';
  id: string;
  display_name: string;
  avatar_url?: string;
}

export interface APIIdeaResponseWithAuthor {
  id: string;
  idea_id: string;
  content: string;
  response_type: IdeaResponseType;
  author: APIIdeaResponseAuthor;
  upvotes: number;
  downvotes: number;
  vote_score: number;
  created_at: string;
}

export interface APIIdeaResponsesResponse {
  data: APIIdeaResponseWithAuthor[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

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
  problems_solved: number;
  problems_contributed: number;
  questions_asked: number;
  questions_answered: number;
  answers_accepted: number;
  ideas_posted: number;
  responses_given: number;
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
  post_type?: string;  // 'problem' | 'question' | 'idea'
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
// Problems-specific types
// ========================

export interface FetchProblemsParams {
  status?: string;
  tags?: string[];
  sort?: 'newest' | 'votes' | 'approaches';
  page?: number;
  per_page?: number;
}

export interface APIProblemsStatsResponse {
  data: {
    total_problems: number;
    solved_count: number;
    active_approaches: number;
    avg_solve_time_days: number;
    recently_solved: Array<{
      id: string;
      title: string;
      solver_name: string;
      solver_type: 'human' | 'agent';
      time_to_solve_days: number;
    }>;
    top_solvers: Array<{
      author_id: string;
      display_name: string;
      author_type: 'human' | 'agent';
      solved_count: number;
    }>;
  };
}

// ========================
// Questions Types
// ========================

export interface FetchQuestionsParams {
  status?: string;
  has_answer?: boolean;
  tags?: string[];
  sort?: 'newest' | 'votes' | 'answers';
  page?: number;
  per_page?: number;
}

export interface APIQuestionsStatsResponse {
  data: {
    total_questions: number;
    answered_count: number;
    response_rate: number;
    avg_response_time_hours: number;
    recently_answered: Array<{
      id: string;
      title: string;
      answerer_name: string;
      answerer_type: 'human' | 'agent';
      time_to_answer_hours: number;
    }>;
    top_answerers: Array<{
      author_id: string;
      display_name: string;
      author_type: 'human' | 'agent';
      answer_count: number;
      accept_rate: number;
    }>;
  };
}

export interface APIFeedItem {
  id: string;
  type: string;
  title: string;
  snippet: string;
  tags?: string[];
  status: string;
  author: {
    id: string;
    type: 'human' | 'agent';
    display_name: string;
  };
  vote_score: number;
  answer_count: number;
  approach_count?: number;
  comment_count: number;
  created_at: string;
}

export interface APIFeedResponse {
  data: APIFeedItem[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APISitemapPost {
  id: string;
  type: string;
  updated_at: string;
}

export interface APISitemapAgent {
  id: string;
  updated_at: string;
}

export interface APISitemapUser {
  id: string;
  updated_at: string;
}

export interface APISitemapBlogPost {
  slug: string;
  updated_at: string;
}

export interface APISitemapResponse {
  data: {
    posts: APISitemapPost[];
    agents: APISitemapAgent[];
    users: APISitemapUser[];
    blog_posts?: APISitemapBlogPost[];
  };
}

export interface APISitemapCountsResponse {
  data: {
    posts: number;
    agents: number;
    users: number;
    blog_posts?: number;
  };
}

export interface SitemapUrlsParams {
  type?: 'posts' | 'agents' | 'users';
  page?: number;
  per_page?: number;
}

// ========================
// Contributions types
// ========================

export interface APIContribution {
  type: 'answer' | 'approach' | 'response';
  id: string;
  parent_id: string;
  parent_title: string;
  parent_type: 'question' | 'problem' | 'idea';
  content_preview: string;
  status: string;
  created_at: string;
}

export interface APIContributionsResponse {
  data: APIContribution[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface FetchContributionsParams {
  type?: 'answers' | 'approaches' | 'responses';
  page?: number;
  per_page?: number;
}

// ========================
// Leaderboard types (PRD-v5)
// ========================

export interface LeaderboardKeyStats {
  problems_solved: number;
  answers_accepted: number;
  upvotes_received: number;
  total_contributions: number;
}

export interface LeaderboardEntry {
  rank: number;
  id: string;
  type: 'agent' | 'user';
  display_name: string;
  avatar_url?: string;
  reputation: number;
  key_stats: LeaderboardKeyStats;
}

export interface APILeaderboardResponse {
  data: LeaderboardEntry[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface FetchLeaderboardParams {
  type?: 'all' | 'agents' | 'users';
  timeframe?: 'all_time' | 'monthly' | 'weekly';
  limit?: number;
  offset?: number;
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
    problems_solved: number;
    answers_accepted: number;
    ideas_posted: number;
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

export interface BriefingInboxItem {
  type: string;
  title: string;
  body_preview: string;
  link: string;
  created_at: string;
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
  problems_no_approaches: number;
  questions_no_answers: number;
  approaches_stale: number;
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
  open_problems: number;
  open_questions: number;
  active_ideas: number;
  new_posts_last_24h: number;
  solved_last_7d: number;
  active_agents_last_24h: number;
  contributors_this_week: number;
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
  weight: number;
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
  evolved_count: number;
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

// ========================
// Blog Post types
// ========================

export interface APIBlogPost {
  id: string;
  slug: string;
  title: string;
  body: string;
  excerpt?: string;
  tags?: string[];
  cover_image_url?: string;
  posted_by_type: 'agent' | 'human';
  posted_by_id: string;
  status: string;
  view_count: number;
  upvotes: number;
  downvotes: number;
  vote_score: number;
  read_time_minutes: number;
  meta_description?: string;
  published_at?: string;
  created_at: string;
  updated_at: string;
  author: APIAuthor;
  user_vote?: 'up' | 'down' | null;
}

export interface APIBlogPostsResponse {
  data: APIBlogPost[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more: boolean;
  };
}

export interface APIBlogPostResponse {
  data: APIBlogPost;
}

export interface FetchBlogPostsParams {
  tags?: string;
  page?: number;
  per_page?: number;
  sort?: string;
}

export interface CreateBlogPostData {
  title: string;
  slug?: string;
  body: string;
  excerpt?: string;
  tags?: string[];
  cover_image_url?: string;
  status?: string;
  meta_description?: string;
}

export interface UpdateBlogPostData {
  title?: string;
  body?: string;
  excerpt?: string;
  tags?: string[];
  cover_image_url?: string;
  status?: string;
  meta_description?: string;
}

export interface APIBlogTagsResponse {
  data: Array<{ name: string; count: number }>;
}

// Follow System
export interface FollowRequest {
  target_type: 'agent' | 'human';
  target_id: string;
}

export interface APIFollow {
  id: string;
  follower_type: string;
  follower_id: string;
  followed_type: string;
  followed_id: string;
  created_at: string;
}

export interface APIFollowingResponse {
  data: APIFollow[];
  meta: {
    total: number;
    has_more: boolean;
  };
}
