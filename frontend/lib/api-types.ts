import type { APIAgentMeResponse } from './api-agent-types';

// Solvr API Types
// Extracted from api.ts to keep files under 800 lines

export * from './status-types';
export * from './api-search-types';
export * from './api-seo-types';
export * from './api-history-types';

export interface APIAuthor {
  id: string;
  type: 'agent' | 'human';
  display_name: string;
  avatar_url?: string;
}

export interface APIPost {
  id: string;
  type: 'post';  // every post is type post (idx 68)
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
  reply_count?: number;  // Canonical unified count of all contributions (server-computed)
  crystallization_cid?: string;
  crystallized_at?: string;
  user_vote?: 'up' | 'down' | null;
  original_language?: string;
  original_title?: string;
  original_description?: string;
  // Canonical post-state fields (server-owned). Present on canonical reads; the
  // client only displays them and never decides publication itself.
  publication_state?: 'draft' | 'published' | 'archived';
  moderation_state?: 'pending' | 'approved' | 'rejected';
  visibility?: 'public' | 'family';
  source_room_id?: string | null;
}

// One canonical Reply — the unified contribution model. A reader never chooses
// approach/answer/response/comment; the server returns the same shape for all.
export interface APIReply {
  id: string;
  post_id: string;
  parent_reply_id?: string | null;
  author: APIAuthor;
  body: string;
  score: number;
  upvotes: number;
  downvotes: number;
  created_at: string;
  updated_at: string;
  legacy_type?: string;
  legacy_id?: string | null;
}

export interface APIRepliesResponse {
  data: APIReply[];
  // Cursor pages carry has_more/next_cursor; numbered pages (?page=, task idx 81)
  // carry page/per_page/total_pages.
  meta: {
    total: number;
    page?: number;
    per_page?: number;
    total_pages?: number;
    has_more?: boolean;
    next_cursor?: string;
  };
}

// GET /v1/replies?author_type=&author_id=: one author's replies, newest first, each with its post.
export interface APIReplyPost {
  id: string;
  type: string;
  title: string;
}

export interface APIAuthoredReply extends APIReply {
  post: APIReplyPost;
}

export interface APIAuthoredRepliesResponse {
  data: APIAuthoredReply[];
  meta: { total: number; has_more: boolean; next_cursor?: string };
}

export interface APIPostsResponse {
  data: APIPost[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    // Pages at this per_page: 0 for an empty list (SPEC.md 27.2). GET /v1/posts sends it;
    // GET /v1/me/posts, which shares this shape, does not.
    total_pages?: number;
    has_more: boolean;
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
  type?: 'post' | 'all';
  status?: string;
  page?: number;
  per_page?: number;
  sort?: 'new' | 'hot' | 'top';
  timeframe?: 'today' | 'week' | 'month';
}

export interface SearchParams {
  q: string;
  type?: 'post' | 'all';
  status?: string;
  tags?: string;
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
  // No type: every post is type post, and the API refuses the retired legacy types and
  // problem-only fields with 400 LEGACY_FIELD_RETIRED (idx 68).
  title: string;
  description: string;
  tags?: string[];
  visibility?: 'public' | 'family';
}

export interface UpdatePostData {
  title?: string;
  description?: string;
  tags?: string[];
}

export interface APICreatePostResponse {
  data: {
    id: string;
    type: 'post';
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
  posted_today: number;
  humans_count: number;
  total_posts: number;
  total_contributions: number;
  crystallized_posts: number;
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

export interface PublicSearchStatsData {
  total_searches_7d: number;
  agent_searches_7d: number;
  human_searches_7d: number;
  trending_queries: Array<{ query: string; count: number }>;
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

// The homepage overview contract and the domain modules below live in their own
// files (keeping each under the 800-line limit) and are re-exported here so
// '@/lib/api-types' stays the single import point for API shapes.
export * from './homepage-types';
export * from './connect-types';
export * from './room-viewer-types';
export * from './api-content-types';
export * from './api-agent-types';
export * from './api-room-types';
