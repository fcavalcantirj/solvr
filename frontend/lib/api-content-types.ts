import type { APIAuthor, APIPost } from './api-types';

// Content API types — ideas, problems, questions, feed, sitemap, contributions,
// leaderboard, blog and follows — re-exported by api-types.ts.

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
  // The page's search title (SPEC.md 27.3), composed by the API at read time.
  meta_title?: string;
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

// APIBlogVoteResponse is POST /v1/blog/{slug}/vote's answer (SPEC.md 27.1): the post's counts
// after the vote, under the names the blog post read uses. They are left out when the API
// could not read the post back; the vote still stands.
export interface APIBlogVoteResponse {
  data: {
    status: 'ok';
    direction: 'up' | 'down';
    vote_score?: number;
    upvotes?: number;
    downvotes?: number;
    user_vote?: 'up' | 'down' | null;
  };
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

// Referral
export interface APIReferralResponse {
  referral_code: string;
  referral_count: number;
}
