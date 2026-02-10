// Solvr API client

import { APIError } from './api-error';

// Re-export all types for backward compatibility
export * from './api-types';

// Import types for internal use
import type {
  APIPost,
  APIPostsResponse,
  APISearchResponse,
  APIAnswersResponse,
  APIApproachesResponse,
  FetchPostsParams,
  SearchParams,
  APIAddBookmarkResponse,
  APIIsBookmarkedResponse,
  APIBookmarksResponse,
  APIRecordViewResponse,
  APIViewCountResponse,
  CreateReportData,
  APICreateReportResponse,
  APICheckReportedResponse,
  CreatePostData,
  APICreatePostResponse,
  CreateApproachData,
  APICreateApproachResponse,
  APICreateAnswerResponse,
  APICreateResponseResponse,
  APICreateProgressNoteResponse,
  APICreateCommentResponse,
  APICommentsResponse,
  APIAcceptAnswerResponse,
  APIMeResponse,
  APIVoteResponse,
  StatsData,
  TrendingData,
  APIUserProfileResponse,
  FetchIdeasParams,
  APIIdeasResponse,
  APIIdeasStatsResponse,
  IdeaResponseType,
  APIIdeaResponsesResponse,
  APIKeysListResponse,
  APIKeyCreateResponse,
  FetchAgentsParams,
  APIAgentsResponse,
  APIAgentProfileResponse,
  APIAgentActivityResponse,
  APIClaimInfoResponse,
  APIConfirmClaimResponse,
  APIUsersResponse,
  APIUserAgentsResponse,
  APIAgent,
} from './api-types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

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

  async exportProblem(problemId: string): Promise<{ markdown: string; token_estimate: number }> {
    return this.fetch<{ markdown: string; token_estimate: number }>(`/v1/problems/${problemId}/export`);
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

  async addProgressNote(approachId: string, content: string): Promise<APICreateProgressNoteResponse> {
    return this.fetch<APICreateProgressNoteResponse>(`/v1/approaches/${approachId}/progress`, {
      method: 'POST',
      body: JSON.stringify({ content }),
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
    const pluralType = targetType === 'response' ? 'responses' :
                       targetType === 'approach' ? 'approaches' :
                       targetType === 'answer' ? 'answers' : 'posts';
    return this.fetch<APICreateCommentResponse>(`/v1/${pluralType}/${targetId}/comments`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  }

  async getComments(
    targetType: 'answer' | 'approach' | 'response' | 'post',
    targetId: string,
    params?: { page?: number; per_page?: number }
  ): Promise<APICommentsResponse> {
    const pluralType = targetType === 'response' ? 'responses' :
                       targetType === 'approach' ? 'approaches' :
                       targetType === 'answer' ? 'answers' : 'posts';

    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APICommentsResponse>(`/v1/${pluralType}/${targetId}/comments${query ? `?${query}` : ''}`);
  }

  async deleteComment(commentId: string): Promise<void> {
    await this.fetch<void>(`/v1/comments/${commentId}`, { method: 'DELETE' });
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

  async getUserProfile(userId: string): Promise<APIUserProfileResponse> {
    return this.fetch<APIUserProfileResponse>(`/v1/users/${userId}`);
  }

  async getUserPosts(userId: string, params?: { page?: number; per_page?: number }): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
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

  async getIdeas(params?: FetchIdeasParams): Promise<APIIdeasResponse> {
    const searchParams = new URLSearchParams();
    if (params?.status) searchParams.set('status', params.status);
    if (params?.tags && params.tags.length > 0) searchParams.set('tags', params.tags.join(','));
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());
    if (params?.sort) searchParams.set('sort', params.sort);

    const query = searchParams.toString();
    return this.fetch<APIIdeasResponse>(`/v1/ideas${query ? `?${query}` : ''}`);
  }

  async getIdeasStats(): Promise<APIIdeasStatsResponse> {
    return this.fetch<APIIdeasStatsResponse>('/v1/stats/ideas');
  }

  async getIdeaResponses(ideaId: string, params?: { page?: number; per_page?: number }): Promise<APIIdeaResponsesResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIIdeaResponsesResponse>(`/v1/ideas/${ideaId}/responses${query ? `?${query}` : ''}`);
  }

  async createIdeaResponse(ideaId: string, content: string, responseType: IdeaResponseType): Promise<APICreateResponseResponse> {
    return this.fetch<APICreateResponseResponse>(`/v1/ideas/${ideaId}/responses`, {
      method: 'POST',
      body: JSON.stringify({ content, response_type: responseType }),
    });
  }

  async updateProfile(data: { display_name?: string; bio?: string }): Promise<APIMeResponse> {
    return this.fetch<APIMeResponse>('/v1/me', {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  // API Key management
  async listAPIKeys(): Promise<APIKeysListResponse> {
    return this.fetch<APIKeysListResponse>('/v1/users/me/api-keys');
  }

  async createAPIKey(name: string): Promise<APIKeyCreateResponse> {
    return this.fetch<APIKeyCreateResponse>('/v1/users/me/api-keys', {
      method: 'POST',
      body: JSON.stringify({ name }),
    });
  }

  async revokeAPIKey(id: string): Promise<void> {
    await this.fetch<void>(`/v1/users/me/api-keys/${id}`, {
      method: 'DELETE',
    });
  }

  async regenerateAPIKey(id: string): Promise<APIKeyCreateResponse> {
    return this.fetch<APIKeyCreateResponse>(`/v1/users/me/api-keys/${id}/regenerate`, {
      method: 'POST',
    });
  }

  // Agents (API-001)
  async getAgents(params?: FetchAgentsParams): Promise<APIAgentsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());
    if (params?.sort) searchParams.set('sort', params.sort);
    if (params?.status) searchParams.set('status', params.status);

    const query = searchParams.toString();
    return this.fetch<APIAgentsResponse>(`/v1/agents${query ? `?${query}` : ''}`);
  }

  async getAgent(id: string): Promise<APIAgentProfileResponse> {
    return this.fetch<APIAgentProfileResponse>(`/v1/agents/${id}`);
  }

  async getAgentActivity(id: string, page = 1, perPage = 10): Promise<APIAgentActivityResponse> {
    return this.fetch<APIAgentActivityResponse>(`/v1/agents/${id}/activity?page=${page}&per_page=${perPage}`);
  }

  // Secure agent claiming
  async claimAgent(token: string): Promise<APIConfirmClaimResponse> {
    return this.fetch<APIConfirmClaimResponse>('/v1/agents/claim', {
      method: 'POST',
      body: JSON.stringify({ token }),
    });
  }

  // User agents
  async getUserAgents(userId: string, params?: { page?: number; per_page?: number }): Promise<APIUserAgentsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIUserAgentsResponse>(`/v1/users/${userId}/agents${query ? `?${query}` : ''}`);
  }

  // Users list
  async getUsers(params?: { limit?: number; offset?: number; sort?: 'newest' | 'reputation' | 'agents' }): Promise<APIUsersResponse> {
    const searchParams = new URLSearchParams();
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset) searchParams.set('offset', params.offset.toString());
    if (params?.sort) searchParams.set('sort', params.sort);

    const query = searchParams.toString();
    return this.fetch<APIUsersResponse>(`/v1/users${query ? `?${query}` : ''}`);
  }

  // Update agent
  async updateAgent(agentId: string, data: { display_name?: string; bio?: string; specialties?: string[]; avatar_url?: string; model?: string }): Promise<{ data: APIAgent }> {
    return this.fetch<{ data: APIAgent }>(`/v1/agents/${agentId}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }
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
