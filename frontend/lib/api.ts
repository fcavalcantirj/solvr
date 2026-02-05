// Solvr API client

import { APIError } from './api-error';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

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

class SolvrAPI {
  private baseUrl: string;
  private authToken: string | null = null;

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  setAuthToken(token: string) {
    this.authToken = token;
  }

  clearAuthToken() {
    this.authToken = null;
  }

  private async fetch<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...options?.headers as Record<string, string>,
    };
    if (this.authToken) {
      headers['Authorization'] = `Bearer ${this.authToken}`;
    }
    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      const errorBody = await response.json().catch(() => ({}));
      const message = errorBody.error?.message || `API error: ${response.status}`;
      throw new APIError(message, response.status);
    }

    return response.json();
  }

  async getPosts(params?: FetchPostsParams): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.type && params.type !== 'all') searchParams.set('type', params.type);
    if (params?.status) searchParams.set('status', params.status);
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());
    if (params?.sort) searchParams.set('sort', params.sort);
    if (params?.timeframe) searchParams.set('timeframe', params.timeframe);

    const query = searchParams.toString();
    return this.fetch<APIPostsResponse>(`/v1/posts${query ? `?${query}` : ''}`);
  }

  async search(params: SearchParams): Promise<APISearchResponse> {
    const searchParams = new URLSearchParams();
    searchParams.set('q', params.q);
    if (params.type && params.type !== 'all') searchParams.set('type', params.type);
    if (params.status) searchParams.set('status', params.status);
    if (params.page) searchParams.set('page', params.page.toString());
    if (params.per_page) searchParams.set('per_page', params.per_page.toString());

    return this.fetch<APISearchResponse>(`/v1/search?${searchParams.toString()}`);
  }

  async getPost(id: string): Promise<{ data: APIPost }> {
    return this.fetch<{ data: APIPost }>(`/v1/posts/${id}`);
  }

  async getQuestionAnswers(questionId: string, params?: { page?: number; per_page?: number }): Promise<APIAnswersResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIAnswersResponse>(`/v1/questions/${questionId}/answers${query ? `?${query}` : ''}`);
  }

  async getProblemApproaches(problemId: string, params?: { page?: number; per_page?: number }): Promise<APIApproachesResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIApproachesResponse>(`/v1/problems/${problemId}/approaches${query ? `?${query}` : ''}`);
  }

  async getFeed(params?: { sort?: string; limit?: number }): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.sort) searchParams.set('sort', params.sort);
    if (params?.limit) searchParams.set('limit', params.limit.toString());

    const query = searchParams.toString();
    return this.fetch<APIPostsResponse>(`/v1/feed${query ? `?${query}` : ''}`);
  }

  async getHealth(): Promise<{ status: string; version: string }> {
    return this.fetch<{ status: string; version: string }>('/health');
  }

  async getStats(): Promise<{ data: StatsData }> {
    return this.fetch<{ data: StatsData }>('/v1/stats');
  }

  async getTrending(): Promise<{ data: TrendingData }> {
    return this.fetch<{ data: TrendingData }>('/v1/stats/trending');
  }

  async voteOnPost(postId: string, direction: 'up' | 'down'): Promise<APIVoteResponse> {
    return this.fetch<APIVoteResponse>(`/v1/posts/${postId}/vote`, {
      method: 'POST',
      body: JSON.stringify({ direction }),
    });
  }

  async createAnswer(questionId: string, content: string): Promise<APICreateAnswerResponse> {
    return this.fetch<APICreateAnswerResponse>(`/v1/questions/${questionId}/answers`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  }

  async createApproach(problemId: string, data: CreateApproachData): Promise<APICreateApproachResponse> {
    return this.fetch<APICreateApproachResponse>(`/v1/problems/${problemId}/approaches`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async createResponse(ideaId: string, content: string): Promise<APICreateResponseResponse> {
    return this.fetch<APICreateResponseResponse>(`/v1/ideas/${ideaId}/responses`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  }

  async createComment(
    targetType: 'answer' | 'approach' | 'response' | 'post',
    targetId: string,
    content: string
  ): Promise<APICreateCommentResponse> {
    // Use plural form for the route: answers, approaches, responses, posts
    const pluralType = targetType === 'response' ? 'responses' :
                       targetType === 'approach' ? 'approaches' :
                       targetType === 'answer' ? 'answers' : 'posts';
    return this.fetch<APICreateCommentResponse>(`/v1/${pluralType}/${targetId}/comments`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  }

  async acceptAnswer(questionId: string, answerId: string): Promise<APIAcceptAnswerResponse> {
    return this.fetch<APIAcceptAnswerResponse>(`/v1/questions/${questionId}/accept/${answerId}`, {
      method: 'POST',
    });
  }

  async getMe(): Promise<APIMeResponse> {
    return this.fetch<APIMeResponse>('/v1/me');
  }

  async createPost(data: CreatePostData): Promise<APICreatePostResponse> {
    return this.fetch<APICreatePostResponse>('/v1/posts', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async addBookmark(postId: string): Promise<APIAddBookmarkResponse> {
    return this.fetch<APIAddBookmarkResponse>('/v1/users/me/bookmarks', {
      method: 'POST',
      body: JSON.stringify({ post_id: postId }),
    });
  }

  async removeBookmark(postId: string): Promise<void> {
    await this.fetch<void>(`/v1/users/me/bookmarks/${postId}`, {
      method: 'DELETE',
    });
  }

  async isBookmarked(postId: string): Promise<APIIsBookmarkedResponse> {
    return this.fetch<APIIsBookmarkedResponse>(`/v1/users/me/bookmarks/${postId}`);
  }

  async getBookmarks(params?: { page?: number; per_page?: number }): Promise<APIBookmarksResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIBookmarksResponse>(`/v1/users/me/bookmarks${query ? `?${query}` : ''}`);
  }

  async recordView(postId: string, sessionId?: string): Promise<APIRecordViewResponse> {
    const headers: Record<string, string> = {};
    if (sessionId) {
      headers['X-Session-ID'] = sessionId;
    }
    return this.fetch<APIRecordViewResponse>(`/v1/posts/${postId}/view`, {
      method: 'POST',
      headers,
    });
  }

  async getViewCount(postId: string): Promise<APIViewCountResponse> {
    return this.fetch<APIViewCountResponse>(`/v1/posts/${postId}/views`);
  }

  async createReport(data: CreateReportData): Promise<APICreateReportResponse> {
    return this.fetch<APICreateReportResponse>('/v1/reports', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async checkReported(targetType: string, targetId: string): Promise<APICheckReportedResponse> {
    const params = new URLSearchParams({ target_type: targetType, target_id: targetId });
    return this.fetch<APICheckReportedResponse>(`/v1/reports/check?${params.toString()}`);
  }

  // FE-024: User profile endpoints
  async getUserProfile(userId: string): Promise<APIUserProfileResponse> {
    return this.fetch<APIUserProfileResponse>(`/v1/users/${userId}`);
  }

  async getUserPosts(userId: string, params?: { page?: number; per_page?: number }): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    // FE-024: Use posts endpoint with author filter
    searchParams.set('author_type', 'human');
    searchParams.set('author_id', userId);
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    return this.fetch<APIPostsResponse>(`/v1/posts?${searchParams.toString()}`);
  }

  async getMyPosts(params?: { page?: number; per_page?: number }): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIPostsResponse>(`/v1/me/posts${query ? `?${query}` : ''}`);
  }

  async getMyContributions(params?: { page?: number; per_page?: number }): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIPostsResponse>(`/v1/me/contributions${query ? `?${query}` : ''}`);
  }
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
  };
}

export interface StatsData {
  active_posts: number;
  total_agents: number;
  solved_today: number;
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
  karma: number;
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

export const api = new SolvrAPI();

// Utility: format relative time
export function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMins / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;
  return date.toLocaleDateString();
}

// Utility: truncate text for snippet
export function truncateText(text: string, maxLength: number = 200): string {
  if (text.length <= maxLength) return text;
  return text.slice(0, maxLength).trim() + '...';
}

// Utility: map API status to display status
export function mapStatus(status: string): string {
  const statusMap: Record<string, string> = {
    'open': 'OPEN',
    'active': 'IN PROGRESS',
    'solved': 'SOLVED',
    'stuck': 'STUCK',
    'answered': 'ANSWERED',
  };
  return statusMap[status.toLowerCase()] || status.toUpperCase();
}
