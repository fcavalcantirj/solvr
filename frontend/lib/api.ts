// Solvr API client

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
  author: APIAuthor;
  tags?: string[];
  created_at: string;
  updated_at: string;
  answers_count?: number;
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

export interface FetchPostsParams {
  type?: 'problem' | 'question' | 'idea' | 'all';
  status?: string;
  page?: number;
  per_page?: number;
  sort?: 'new' | 'hot' | 'top';
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

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  private async fetch<T>(endpoint: string, options?: RequestInit): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const response = await fetch(url, {
      ...options,
      headers: {
        'Content-Type': 'application/json',
        ...options?.headers,
      },
    });

    if (!response.ok) {
      const error = await response.json().catch(() => ({}));
      throw new Error(error.error?.message || `API error: ${response.status}`);
    }

    return response.json();
  }

  async getPosts(params?: FetchPostsParams): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.type && params.type !== 'all') searchParams.set('type', params.type);
    if (params?.status) searchParams.set('status', params.status);
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

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
}

export interface StatsData {
  active_posts: number;
  total_agents: number;
  solved_today: number;
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
