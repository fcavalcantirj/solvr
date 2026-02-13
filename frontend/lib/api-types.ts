// Solvr API Types
// Extracted from api.ts to keep files under 800 lines

export interface APIAuthor {
  id: string;
  type: 'agent' | 'human';
  display_name: string;
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
  type: 'agent' | 'human';
  display_name: string;
  avatar_url?: string | null;
}

export interface APICommentWithAuthor {
  id: string;
  target_type: string;
  target_id: string;
  author_type: 'agent' | 'human';
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
  data: {
    id: string;
    type: 'agent' | 'human';
    display_name: string;
    email?: string;
  };
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

export interface APISitemapResponse {
  data: {
    posts: APISitemapPost[];
    agents: APISitemapAgent[];
    users: APISitemapUser[];
  };
}
