// Solvr API client.
//
// Split layout: `lib/api-base.ts` holds `SolvrAPIBase` with the core fetch
// wrapper, auth, and most domain methods. This file extends it with the
// remaining endpoint groups (stats, leaderboard, pins, blog, rooms, etc.)
// and exports the singleton `api` instance plus the utility helpers.
//
// The split keeps each file under the 800-line CI cap (scripts/check-file-size.sh).

import { SolvrAPIBase } from './api-base';

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
  UpdatePostData,
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
} from './api-types';

class SolvrAPI extends SolvrAPIBase {
  async getProblemsStats(): Promise<APIProblemsStatsResponse> {
    return this.fetch<APIProblemsStatsResponse>('/v1/stats/problems');
  }

  async getPublicSearchStats(): Promise<{ data: PublicSearchStatsData }> {
    return this.fetch<{ data: PublicSearchStatsData }>('/v1/stats/search');
  }

  async getQuestions(params?: FetchQuestionsParams): Promise<APIPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.status) searchParams.set('status', params.status);
    if (params?.has_answer !== undefined) searchParams.set('has_answer', params.has_answer.toString());
    if (params?.tags && params.tags.length > 0) searchParams.set('tags', params.tags.join(','));
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());
    if (params?.sort) searchParams.set('sort', params.sort);

    const query = searchParams.toString();
    const endpoint = `/v1/questions${query ? `?${query}` : ''}`;

    try {
      const response = await this.fetch<APIPostsResponse>(endpoint);

      // Defensive: validate response structure
      if (!response || typeof response !== 'object') {
        console.error('[api.getQuestions] Invalid response format:', response);
        throw new Error('Invalid API response format');
      }

      return response;
    } catch (err) {
      console.error('[api.getQuestions] Request failed:', endpoint, err);
      throw err;
    }
  }

  async getQuestionsStats(): Promise<APIQuestionsStatsResponse> {
    return this.fetch<APIQuestionsStatsResponse>('/v1/stats/questions');
  }

  async getUserContributions(userId: string, params?: FetchContributionsParams): Promise<APIContributionsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.type) searchParams.set('type', params.type);
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIContributionsResponse>(`/v1/users/${userId}/contributions${query ? `?${query}` : ''}`);
  }

  async getStuckProblems(params?: { page?: number; per_page?: number }): Promise<APIFeedResponse> {
    const searchParams = new URLSearchParams();
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());

    const query = searchParams.toString();
    return this.fetch<APIFeedResponse>(`/v1/feed/stuck${query ? `?${query}` : ''}`);
  }

  // Leaderboard (PRD-v5)
  async getLeaderboard(params?: FetchLeaderboardParams): Promise<APILeaderboardResponse> {
    const searchParams = new URLSearchParams();
    if (params?.type && params.type !== 'all') searchParams.set('type', params.type);
    if (params?.timeframe) searchParams.set('timeframe', params.timeframe);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset) searchParams.set('offset', params.offset.toString());

    const query = searchParams.toString();
    return this.fetch<APILeaderboardResponse>(`/v1/leaderboard${query ? `?${query}` : ''}`);
  }

  // IPFS Health
  async getIPFSHealth(): Promise<APIIPFSHealthResponse> {
    return this.fetch<APIIPFSHealthResponse>('/v1/health/ipfs');
  }

  // System Status
  async getStatus(): Promise<APIStatusResponse> {
    return this.fetch<APIStatusResponse>('/v1/status');
  }

  // Pins / IPFS Pinning
  async createPin(params: CreatePinParams): Promise<APIPinResponse> {
    const body: Record<string, unknown> = { cid: params.cid };
    if (params.name) body.name = params.name;
    if (params.origins?.length) body.origins = params.origins;
    if (params.meta && Object.keys(params.meta).length > 0) body.meta = params.meta;
    return this.fetch<APIPinResponse>('/v1/pins', {
      method: 'POST',
      body: JSON.stringify(body),
    });
  }

  async listPins(params?: FetchPinsParams): Promise<APIPinsListResponse> {
    const searchParams = new URLSearchParams();
    if (params?.cid) searchParams.set('cid', params.cid);
    if (params?.name) searchParams.set('name', params.name);
    if (params?.status) searchParams.set('status', params.status);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.meta && Object.keys(params.meta).length > 0) {
      searchParams.set('meta', JSON.stringify(params.meta));
    }

    const query = searchParams.toString();
    return this.fetch<APIPinsListResponse>(`/v1/pins${query ? `?${query}` : ''}`);
  }

  async deletePin(requestID: string): Promise<void> {
    await this.fetch<void>(`/v1/pins/${encodeURIComponent(requestID)}`, {
      method: 'DELETE',
    });
  }

  async getStorageUsage(): Promise<APIStorageResponse> {
    return this.fetch<APIStorageResponse>('/v1/me/storage');
  }

  async getAgentPins(agentId: string, params?: FetchPinsParams): Promise<APIPinsListResponse> {
    const searchParams = new URLSearchParams();
    if (params?.cid) searchParams.set('cid', params.cid);
    if (params?.name) searchParams.set('name', params.name);
    if (params?.status) searchParams.set('status', params.status);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.meta && Object.keys(params.meta).length > 0) {
      searchParams.set('meta', JSON.stringify(params.meta));
    }

    const query = searchParams.toString();
    return this.fetch<APIPinsListResponse>(`/v1/agents/${encodeURIComponent(agentId)}/pins${query ? `?${query}` : ''}`);
  }

  async getAgentStorage(agentId: string): Promise<APIStorageResponse> {
    return this.fetch<APIStorageResponse>(`/v1/agents/${encodeURIComponent(agentId)}/storage`);
  }

  async getLeaderboardByTag(tag: string, params?: FetchLeaderboardParams): Promise<APILeaderboardResponse> {
    const searchParams = new URLSearchParams();
    if (params?.type && params.type !== 'all') searchParams.set('type', params.type);
    if (params?.timeframe) searchParams.set('timeframe', params.timeframe);
    if (params?.limit) searchParams.set('limit', params.limit.toString());
    if (params?.offset) searchParams.set('offset', params.offset.toString());

    const query = searchParams.toString();
    return this.fetch<APILeaderboardResponse>(`/v1/leaderboard/tags/${encodeURIComponent(tag)}${query ? `?${query}` : ''}`);
  }

  // Badges
  async getAgentBadges(agentId: string): Promise<{ data: APIBadgesResponse }> {
    return this.fetch<{ data: APIBadgesResponse }>(`/v1/agents/${encodeURIComponent(agentId)}/badges`);
  }

  async getUserBadges(userId: string): Promise<{ data: APIBadgesResponse }> {
    return this.fetch<{ data: APIBadgesResponse }>(`/v1/users/${encodeURIComponent(userId)}/badges`);
  }

  // Follow system
  async follow(targetType: string, targetId: string): Promise<APIFollow> {
    return this.fetch<APIFollow>('/v1/follow', {
      method: 'POST',
      body: JSON.stringify({ target_type: targetType, target_id: targetId }),
    });
  }

  async unfollow(targetType: string, targetId: string): Promise<{ status: string }> {
    return this.fetch<{ status: string }>('/v1/follow', {
      method: 'DELETE',
      body: JSON.stringify({ target_type: targetType, target_id: targetId }),
    });
  }

  async getFollowing(limit = 20, offset = 0): Promise<APIFollowingResponse> {
    const params = new URLSearchParams();
    params.set('limit', limit.toString());
    params.set('offset', offset.toString());
    return this.fetch<APIFollowingResponse>(`/v1/following?${params.toString()}`);
  }

  async getFollowers(limit = 20, offset = 0): Promise<APIFollowingResponse> {
    const params = new URLSearchParams();
    params.set('limit', limit.toString());
    params.set('offset', offset.toString());
    return this.fetch<APIFollowingResponse>(`/v1/followers?${params.toString()}`);
  }

  // Agent Checkpoints
  async getAgentCheckpoints(agentId: string): Promise<APICheckpointsResponse> {
    return this.fetch<APICheckpointsResponse>(`/v1/agents/${encodeURIComponent(agentId)}/checkpoints`);
  }

  // Resurrection Bundle
  async getResurrectionBundle(agentId: string): Promise<APIResurrectionBundle> {
    return this.fetch<APIResurrectionBundle>(`/v1/agents/${encodeURIComponent(agentId)}/resurrection-bundle`);
  }

  // Blog Posts
  async getBlogPosts(params?: FetchBlogPostsParams): Promise<APIBlogPostsResponse> {
    const searchParams = new URLSearchParams();
    if (params?.tags) searchParams.set('tags', params.tags);
    if (params?.page) searchParams.set('page', params.page.toString());
    if (params?.per_page) searchParams.set('per_page', params.per_page.toString());
    if (params?.sort) searchParams.set('sort', params.sort);

    const query = searchParams.toString();
    return this.fetch<APIBlogPostsResponse>(`/v1/blog${query ? `?${query}` : ''}`);
  }

  async getBlogPost(slug: string): Promise<APIBlogPostResponse> {
    return this.fetch<APIBlogPostResponse>(`/v1/blog/${encodeURIComponent(slug)}`);
  }

  async getBlogFeatured(): Promise<APIBlogPostResponse> {
    return this.fetch<APIBlogPostResponse>('/v1/blog/featured');
  }

  async getBlogTags(): Promise<APIBlogTagsResponse> {
    return this.fetch<APIBlogTagsResponse>('/v1/blog/tags');
  }

  async createBlogPost(data: CreateBlogPostData): Promise<APIBlogPostResponse> {
    return this.fetch<APIBlogPostResponse>('/v1/blog', {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  async updateBlogPost(slug: string, data: UpdateBlogPostData): Promise<APIBlogPostResponse> {
    return this.fetch<APIBlogPostResponse>(`/v1/blog/${encodeURIComponent(slug)}`, {
      method: 'PATCH',
      body: JSON.stringify(data),
    });
  }

  async deleteBlogPost(slug: string): Promise<void> {
    await this.fetch<void>(`/v1/blog/${encodeURIComponent(slug)}`, {
      method: 'DELETE',
    });
  }

  async voteBlogPost(slug: string, direction: 'up' | 'down'): Promise<APIVoteResponse> {
    return this.fetch<APIVoteResponse>(`/v1/blog/${encodeURIComponent(slug)}/vote`, {
      method: 'POST',
      body: JSON.stringify({ direction }),
    });
  }

  async recordBlogView(slug: string): Promise<void> {
    await this.fetch<void>(`/v1/blog/${encodeURIComponent(slug)}/view`, {
      method: 'POST',
    });
  }

  // Referrals
  async getMyReferral(): Promise<APIReferralResponse> {
    return this.fetch<APIReferralResponse>('/v1/users/me/referral');
  }

  async isFollowing(targetType: string, targetId: string): Promise<boolean> {
    try {
      const response = await this.getFollowing(100, 0);
      return response.data.some(
        (f) => f.followed_type === targetType && f.followed_id === targetId
      );
    } catch {
      return false;
    }
  }

  // ========================
  // Room API (Phase 16)
  // ========================

  /** Fetch paginated list of public rooms with live agent count and participant stats. */
  async fetchRooms(limit = 20, offset = 0): Promise<APIRoomListResponse> {
    return this.fetch<APIRoomListResponse>(`/v1/rooms?limit=${limit}&offset=${offset}`);
  }

  /** Fetch a single room by slug with agents and recent messages. */
  async fetchRoom(slug: string): Promise<APIRoomDetailResponse> {
    return this.fetch<APIRoomDetailResponse>(`/v1/rooms/${encodeURIComponent(slug)}`);
  }

  /** Fetch messages for a room with optional cursor-based pagination. */
  async fetchRoomMessages(slug: string, afterId?: number, limit = 50): Promise<APIRoomMessagesResponse> {
    const params = new URLSearchParams();
    if (afterId !== undefined) params.set('after', String(afterId));
    params.set('limit', String(limit));
    return this.fetch<APIRoomMessagesResponse>(`/v1/rooms/${encodeURIComponent(slug)}/messages?${params}`);
  }

  /** Create a new room. Requires JWT authentication. */
  async createRoom(data: {
    display_name: string;
    description?: string;
    category?: string;
    tags?: string[];
  }): Promise<{ data: { slug: string; id: string; display_name: string }; token: string }> {
    return this.fetch(`/v1/rooms`, {
      method: 'POST',
      body: JSON.stringify(data),
    });
  }

  /** Post a human comment to a room. Requires JWT authentication. */
  async postRoomMessage(slug: string, content: string): Promise<APIPostRoomMessageResponse> {
    return this.fetch<APIPostRoomMessageResponse>(`/v1/rooms/${encodeURIComponent(slug)}/messages`, {
      method: 'POST',
      body: JSON.stringify({ content }),
    });
  }

  /**
   * Rotate the A2A bearer token for a room. Only the owner can call this (D-25).
   * Returns a new plain token — the previous token is invalidated and can never
   * be retrieved again (SHA256+bcrypt one-way hash at rest, D-24).
   */
  async rotateRoomToken(slug: string): Promise<{ data: { token: string } }> {
    return this.fetch<{ data: { token: string } }>(
      `/v1/rooms/${encodeURIComponent(slug)}/rotate-token`,
      { method: 'POST' },
    );
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
  // Use UTC to avoid hydration mismatch between server and client timezones
  return `${date.getUTCMonth() + 1}/${date.getUTCDate()}/${date.getUTCFullYear()}`;
}

// Utility: truncate text for snippet
export function truncateText(text: string | undefined, maxLength: number = 200): string {
  if (!text) return '';
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
    'pending_review': 'UNDER REVIEW',
    'rejected': 'REJECTED',
  };
  return statusMap[status.toLowerCase()] || status.toUpperCase();
}
