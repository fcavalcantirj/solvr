// Solvr API client

import { APIError } from './api-error';

// Re-export all types for backward compatibility
export * from './api-types';

// Import types for internal use
import type {
  APIPostSourceRoom,
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
  APIRepliesResponse,
  APIRoom,
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
  APISitemapResponse,
  APISitemapCountsResponse,
  SitemapUrlsParams,
  APIProblemsStatsResponse,
  APIFeedResponse,
  FetchProblemsParams,
  FetchQuestionsParams,
  APIQuestionsStatsResponse,
  APIContributionsResponse,
  FetchContributionsParams,
  APILeaderboardResponse,
  FetchLeaderboardParams,
  APIIPFSHealthResponse,
  APIPinResponse,
  APIPinsListResponse,
  FetchPinsParams,
  CreatePinParams,
  APIStorageResponse,
  APIAuthMethodsListResponse,
  APIAgentBriefingResponse,
  APIApproachVersionHistory,
  APIFollow,
  APIFollowingResponse,
  APIBadgesResponse,
  APICheckpointsResponse,
  APIResurrectionBundle,
  APIStatusResponse,
  APIBlogPostsResponse,
  APIBlogPostResponse,
  FetchBlogPostsParams,
  CreateBlogPostData,
  UpdateBlogPostData,
  APIBlogTagsResponse,
  PublicSearchStatsData,
  APIReferralResponse,
  APIRoomListResponse,
  APIRoomDetailResponse,
  APIRoomMessagesResponse,
  APIPostRoomMessageResponse,
  APICollaborationExampleResponse,
  APIHomepageOverviewResponse,
  APIHomepageActivityResponse,
  APIHomepageRoomsResponse,
  APIHomepageSearchResponse,
  APIHomepageAPIUsageResponse,
  APIOverviewResponse,
  APIConnectStartResponse,
  ConnectStartParams,
  APIConnectExamplesResponse,
} from './api-types';

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export interface FetchOptions extends RequestInit {
  skipAuthEvent?: boolean;
}

// Base class split out to keep each module under the 800-line CI cap.
// lib/api.ts defines `class SolvrAPI extends SolvrAPIBase` with the remaining
// domain methods and exports the singleton `api` instance.
export class SolvrAPIBase {
  private baseUrl: string;
  private authToken: string | null = null;
  private authEventHandlers: Array<(error: APIError) => void> = [];

  constructor(baseUrl: string = API_BASE_URL) {
    this.baseUrl = baseUrl;
  }

  setAuthToken(token: string) {
    this.authToken = token;
  }

  clearAuthToken() {
    this.authToken = null;
  }

  onAuthError(handler: (error: APIError) => void) {
    this.authEventHandlers.push(handler);
  }

  offAuthError(handler: (error: APIError) => void) {
    this.authEventHandlers = this.authEventHandlers.filter(h => h !== handler);
  }

  protected async fetch<T>(endpoint: string, options?: FetchOptions): Promise<T> {
    return (await this.send(endpoint, options)).json();
  }

  // send issues the request and throws APIError on a non-2xx answer. lib/api.ts reads the
  // ETag of a response through it (the version an edit must send back as If-Match).
  protected async send(endpoint: string, options?: FetchOptions): Promise<Response> {
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
      const error = new APIError(message, response.status);

      // Emit auth errors before throwing (unless explicitly skipped for optional background checks)
      if (error.statusCode === 401 && !options?.skipAuthEvent) {
        this.authEventHandlers.forEach(handler => handler(error));
      }

      throw error;
    }

    return response;
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
    if (params.tags) searchParams.set('tags', params.tags);
    if (params.page) searchParams.set('page', params.page.toString());
    if (params.per_page) searchParams.set('per_page', params.per_page.toString());

    const endpoint = `/v1/search?${searchParams.toString()}`;

    try {
      const response = await this.fetch<APISearchResponse>(endpoint);

      // Defensive: validate response structure
      if (!response || typeof response !== 'object') {
        console.error('[api.search] Invalid response format:', response);
        throw new Error('Invalid API response format');
      }

      return response;
    } catch (err) {
      console.error('[api.search] Request failed:', endpoint, err);
      throw err;
    }
  }

  async getPostReplies(id: string): Promise<APIRepliesResponse> {
    return this.fetch<APIRepliesResponse>(`/v1/posts/${id}/replies`);
  }
  async getRelatedRooms(id: string): Promise<{ data: APIRoom[]; source_room?: APIPostSourceRoom | null }> {
    return this.fetch<{ data: APIRoom[] }>(`/v1/posts/${id}/rooms`);
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

  // The homepage proof. Public, no credentials: the API answers the same thing
  // to a logged-out visitor as it does to anyone else.
  async getCollaborationExample(): Promise<APICollaborationExampleResponse> {
    return this.fetch<APICollaborationExampleResponse>('/v1/homepage/example');
  }

  // The whole live index in one public read: room statistics, the public
  // activity stream, the editorial room previews, API usage, search statistics,
  // the all-time totals and the reusable posts.
  async getHomepageOverview(): Promise<APIHomepageOverviewResponse> {
    return this.fetch<APIHomepageOverviewResponse>('/v1/homepage/overview');
  }

  // The consolidated overview endpoint (Task 14). Same data as
  // getHomepageOverview, but wrapped in a meta envelope carrying generated_at,
  // window boundaries, source availability, and any partial-error state.
  // The browser renders the data and meta it receives; it computes nothing.
  async getOverview(window?: string): Promise<APIOverviewResponse> {
    const params = new URLSearchParams();
    if (window) params.set('window', window);
    const query = params.toString();
    return this.fetch<APIOverviewResponse>(
      `/v1/overview${query ? `?${query}` : ''}`,
    );
  }

  // The homepage activity stream: Load more behind it, and — when a cursor is
  // passed — the check for entries that arrived since the page was read. The
  // API decides the page contents, whether another page exists, and whether
  // anything is new.
  async getHomepageActivity(
    offset: number,
    limit: number,
    since?: string,
  ): Promise<APIHomepageActivityResponse> {
    const params = new URLSearchParams({ offset: offset.toString(), limit: limit.toString() });
    if (since) params.set('since', since);
    return this.fetch<APIHomepageActivityResponse>(`/v1/homepage/activity?${params.toString()}`);
  }

  // The start-flow contract behind every connection surface: the panel on the index
  // and the full /connect page read this same endpoint. The API owns the sentence,
  // its segments, the use cases and their lines; a parameter is sent only when the
  // visitor actually chose it, so the API's own defaults are the only defaults.
  async getConnectStart(params?: ConnectStartParams): Promise<APIConnectStartResponse> {
    const search = new URLSearchParams();
    if (params?.intent) search.set('intent', params.intent);
    if (params?.preset) search.set('preset', params.preset);
    if (params?.visibility) search.set('visibility', params.visibility);
    if (params?.from_room) search.set('from_room', params.from_room);
    if (params?.post) search.set('post', params.post);
    const query = search.toString();
    return this.fetch<APIConnectStartResponse>(`/v1/connect${query ? `?${query}` : ''}`);
  }

  // The three example sentences the guides and the home cards show.
  async getConnectExamples(): Promise<APIConnectExamplesResponse> {
    return this.fetch<APIConnectExamplesResponse>('/v1/connect/examples');
  }

  // The public room statistics alone, for the shared 24h / 7d / 30d selector.
  // The window value comes straight from the options the API already sent.
  async getHomepageRooms(window: string): Promise<APIHomepageRoomsResponse> {
    const params = new URLSearchParams({ window });
    return this.fetch<APIHomepageRoomsResponse>(`/v1/homepage/rooms?${params.toString()}`);
  }

  // The search statistics alone, behind the same 24h / 7d / 30d selector. The
  // window value comes straight from the options the API already sent.
  async getHomepageSearch(window: string): Promise<APIHomepageSearchResponse> {
    const params = new URLSearchParams({ window });
    return this.fetch<APIHomepageSearchResponse>(`/v1/homepage/search?${params.toString()}`);
  }

  // Aggregate API usage alone, behind the same 24h / 7d / 30d selector. The
  // window value comes straight from the options the API already sent.
  async getHomepageApiUsage(window: string): Promise<APIHomepageAPIUsageResponse> {
    const params = new URLSearchParams({ window });
    return this.fetch<APIHomepageAPIUsageResponse>(`/v1/homepage/api-usage?${params.toString()}`);
  }

  async voteOnPost(postId: string, direction: 'up' | 'down'): Promise<APIVoteResponse> {
    return this.fetch<APIVoteResponse>(`/v1/posts/${postId}/vote`, {
      method: 'POST',
      body: JSON.stringify({ direction }),
    });
  }

  async getMyVote(postId: string): Promise<{ data: { vote: 'up' | 'down' | null } }> {
    return this.fetch<{ data: { vote: 'up' | 'down' | null } }>(`/v1/posts/${postId}/my-vote`, {
      method: 'GET',
      skipAuthEvent: true,  // Don't show auth modal on 401 - this is an optional background check
    });
  }

  async voteOnAnswer(answerId: string, direction: 'up' | 'down'): Promise<{ message: string }> {
    return this.fetch<{ message: string }>(`/v1/answers/${answerId}/vote`, {
      method: 'POST',
      body: JSON.stringify({ direction }),
    });
  }

  async addProgressNote(approachId: string, content: string): Promise<APICreateProgressNoteResponse> {
    return this.fetch<APICreateProgressNoteResponse>(`/v1/approaches/${approachId}/progress`, {
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

  async verifyApproach(approachId: string, verified: boolean = true): Promise<{ message: string; verified: boolean }> {
    return this.fetch<{ message: string; verified: boolean }>(`/v1/approaches/${approachId}/verify`, {
      method: 'POST',
      body: JSON.stringify({ verified }),
    });
  }

  async getMe(): Promise<APIMeResponse> {
    return this.fetch<APIMeResponse>('/v1/me');
  }

  async getAgentBriefing(agentId: string): Promise<APIAgentBriefingResponse> {
    return this.fetch<APIAgentBriefingResponse>(`/v1/agents/${agentId}/briefing`);
  }

  async getMyAuthMethods(): Promise<APIAuthMethodsListResponse> {
    return this.fetch<APIAuthMethodsListResponse>('/v1/me/auth-methods');
  }

  // Email/password authentication
  async login(email: string, password: string): Promise<{
    access_token: string;
    refresh_token: string;
    user: {
      id: string;
      username: string;
      display_name: string;
      email: string;
      role: string;
    };
  }> {
    return this.fetch<{
      access_token: string;
      refresh_token: string;
      user: {
        id: string;
        username: string;
        display_name: string;
        email: string;
        role: string;
      };
    }>('/v1/auth/login', {
      method: 'POST',
      body: JSON.stringify({ email, password }),
    });
  }

  async register(email: string, password: string, username: string, displayName: string, ref?: string): Promise<{
    access_token: string;
    refresh_token: string;
    user: {
      id: string;
      username: string;
      display_name: string;
      email: string;
      role: string;
    };
  }> {
    return this.fetch<{
      access_token: string;
      refresh_token: string;
      user: {
        id: string;
        username: string;
        display_name: string;
        email: string;
        role: string;
      };
    }>('/v1/auth/register', {
      method: 'POST',
      body: JSON.stringify({ email, password, username, display_name: displayName, ...(ref && { ref }) }),
    });
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
      skipAuthEvent: true,  // A view is a background beacon: a refused one never opens the login dialog
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

  async updateProfile(data: { display_name?: string; bio?: string }): Promise<APIMeResponse> {
    return this.fetch<APIMeResponse>('/v1/me', {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  async deleteMe(): Promise<void> {
    await this.fetch<void>('/v1/me', {
      method: 'DELETE',
    });
    // Clear auth token after successful deletion
    this.clearAuthToken();
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

  // Get claim token info (public, no auth). The token goes in a body: never in a URL.
  async getClaimInfo(token: string): Promise<APIClaimInfoResponse> {
    return this.fetch<APIClaimInfoResponse>('/v1/agents/claim/lookup', { method: 'POST', body: JSON.stringify({ token }) });
  }

  // Secure agent claiming (requires JWT auth)
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

  async getSitemapUrls(params?: SitemapUrlsParams): Promise<APISitemapResponse> {
    const searchParams = new URLSearchParams();
    if (params?.type) searchParams.set('type', params.type);
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());
    const qs = searchParams.toString();
    return this.fetch<APISitemapResponse>(`/v1/sitemap/urls${qs ? `?${qs}` : ''}`);
  }

  async getSitemapCounts(): Promise<APISitemapCountsResponse> {
    return this.fetch<APISitemapCountsResponse>('/v1/sitemap/counts');
  }

}
