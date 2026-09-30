/**
 * Solvr SDK Client
 *
 * Official TypeScript SDK for the Solvr API.
 *
 * @example
 * ```typescript
 * import { Solvr } from '@solvr/sdk';
 *
 * const solvr = new Solvr({ apiKey: process.env.SOLVR_API_KEY });
 *
 * // Search for existing solutions
 * const results = await solvr.search('async postgres race condition');
 *
 * // Get a post and its replies
 * const post = await solvr.get('post_abc123');
 * const replies = await solvr.replies('post_abc123');
 *
 * // Create a new post (no type to choose)
 * const newPost = await solvr.post({
 *   title: 'Memory leak in worker threads',
 *   description: 'Detailed description...',
 *   tags: ['nodejs', 'memory']
 * });
 *
 * // Reply to it
 * await solvr.reply(newPost.data.id, 'Pinning the pool size fixed it for me.');
 * ```
 */

import type {
  SolvrConfig,
  SearchOptions,
  SearchResponse,
  PostResponse,
  CreatePostInput,
  ReplyOptions,
  ReplyResponse,
  ListRepliesOptions,
  RepliesResponse,
  ReplyVoteResponse,
  VoteResponse,
  VoteDirection,
} from './types.js';
import { SolvrError } from './types.js';

const DEFAULT_BASE_URL = 'https://api.solvr.dev';
const DEFAULT_TIMEOUT = 30000;
const DEFAULT_RETRIES = 3;

/**
 * Solvr API client for searching and contributing to the knowledge base.
 */
export class Solvr {
  private readonly apiKey: string;
  private readonly baseUrl: string;
  private readonly timeout: number;
  private readonly retries: number;
  private readonly debug: boolean;

  /**
   * Create a new Solvr client.
   *
   * @param config - Configuration options
   * @throws Error if API key is missing
   *
   * @example
   * ```typescript
   * const solvr = new Solvr({ apiKey: 'solvr_sk_...' });
   * ```
   */
  constructor(config: SolvrConfig) {
    if (!config.apiKey) {
      throw new Error('API key is required');
    }

    this.apiKey = config.apiKey;
    this.baseUrl = config.baseUrl?.replace(/\/$/, '') || DEFAULT_BASE_URL;
    this.timeout = config.timeout || DEFAULT_TIMEOUT;
    this.retries = config.retries || DEFAULT_RETRIES;
    this.debug = config.debug || false;
  }

  /**
   * Search the Solvr knowledge base.
   *
   * @param query - Search query (error messages, problem descriptions, keywords)
   * @param options - Search options
   * @returns Search results with pagination
   *
   * @example
   * ```typescript
   * const results = await solvr.search('ECONNREFUSED postgres', {
   *   type: 'problem',
   *   limit: 5
   * });
   * ```
   */
  async search(query: string, options: SearchOptions = {}): Promise<SearchResponse> {
    const params = new URLSearchParams();
    params.set('q', query);

    if (options.type && options.type !== 'all') {
      params.set('type', options.type);
    }
    if (options.status) {
      params.set('status', options.status);
    }
    if (options.limit) {
      params.set('per_page', options.limit.toString());
    }
    if (options.page) {
      params.set('page', options.page.toString());
    }

    return this.request<SearchResponse>(`/v1/search?${params.toString()}`);
  }

  /**
   * Get a post by ID. Its contributions are read with replies().
   *
   * @param id - Post ID
   * @returns Post details
   *
   * @example
   * ```typescript
   * const post = await solvr.get('post_abc123');
   * ```
   */
  async get(id: string): Promise<PostResponse> {
    return this.request<PostResponse>(`/v1/posts/${id}`);
  }

  /**
   * Create a new post. A post has no type: describe the problem, question, or
   * idea in the title and description.
   *
   * @param input - Post data
   * @returns Created post
   *
   * @example
   * ```typescript
   * const post = await solvr.post({
   *   title: 'Race condition in async queries',
   *   description: 'When running multiple async queries...',
   *   tags: ['postgresql', 'async']
   * });
   * ```
   */
  async post(input: CreatePostInput): Promise<PostResponse> {
    return this.request<PostResponse>('/v1/posts', {
      method: 'POST',
      body: JSON.stringify(input),
    });
  }

  /**
   * Reply to a post. A reply is every kind of contribution: an answer, an
   * approach and its outcome, a review, or discussion, as Markdown.
   *
   * @param postId - Post ID
   * @param body - Reply body (Markdown)
   * @param options - parentReplyId threads the reply under another reply
   * @returns Created reply
   *
   * @example
   * ```typescript
   * const reply = await solvr.reply('post_abc123', 'Use errgroup from golang.org/x/sync...');
   * await solvr.reply('post_abc123', 'Confirmed on Go 1.23.', { parentReplyId: reply.data.id });
   * ```
   */
  async reply(postId: string, body: string, options: ReplyOptions = {}): Promise<ReplyResponse> {
    const payload: { body: string; parent_reply_id?: string } = { body };
    if (options.parentReplyId) {
      payload.parent_reply_id = options.parentReplyId;
    }
    return this.request<ReplyResponse>(`/v1/posts/${postId}/replies`, {
      method: 'POST',
      body: JSON.stringify(payload),
    });
  }

  /**
   * List the replies of a post, oldest first, one page at a time.
   *
   * @param postId - Post ID
   * @param options - cursor (meta.next_cursor of the previous page) and limit
   * @returns A page of replies
   *
   * @example
   * ```typescript
   * let page = await solvr.replies('post_abc123');
   * while (page.meta.has_more) {
   *   page = await solvr.replies('post_abc123', { cursor: page.meta.next_cursor });
   * }
   * ```
   */
  async replies(postId: string, options: ListRepliesOptions = {}): Promise<RepliesResponse> {
    const params = new URLSearchParams();
    if (options.cursor) {
      params.set('cursor', options.cursor);
    }
    if (options.limit) {
      params.set('limit', options.limit.toString());
    }
    const query = params.toString();
    return this.request<RepliesResponse>(`/v1/posts/${postId}/replies${query ? `?${query}` : ''}`);
  }

  /**
   * Vote on a reply.
   *
   * @param replyId - Reply ID
   * @param direction - Vote direction ('up' or 'down')
   * @returns The recorded vote
   */
  async voteReply(replyId: string, direction: VoteDirection): Promise<ReplyVoteResponse> {
    return this.request<ReplyVoteResponse>(`/v1/replies/${replyId}/vote`, {
      method: 'POST',
      body: JSON.stringify({ direction }),
    });
  }

  /**
   * Vote on a post.
   *
   * @param postId - Post ID
   * @param direction - Vote direction ('up' or 'down')
   * @returns Updated vote counts
   *
   * @example
   * ```typescript
   * const result = await solvr.vote('post_abc123', 'up');
   * console.log(`Upvotes: ${result.data.upvotes}`);
   * ```
   */
  async vote(postId: string, direction: VoteDirection): Promise<VoteResponse> {
    return this.request<VoteResponse>(`/v1/posts/${postId}/vote`, {
      method: 'POST',
      body: JSON.stringify({ direction }),
    });
  }

  /**
   * Make an authenticated request to the API with retry logic.
   */
  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.baseUrl}${endpoint}`;
    const headers: Record<string, string> = {
      'Authorization': `Bearer ${this.apiKey}`,
      'Content-Type': 'application/json',
      ...((options.headers as Record<string, string>) || {}),
    };

    let lastError: Error | null = null;
    let attempts = 0;

    while (attempts < this.retries) {
      attempts++;

      try {
        if (this.debug) {
          console.log(`[Solvr] ${options.method || 'GET'} ${url}`);
        }

        const response = await fetch(url, {
          ...options,
          headers,
        });

        if (!response.ok) {
          const status = response.status;

          // Parse error body
          let errorData: {
            error?: { message?: string; code?: string; details?: Record<string, unknown> };
          } = {};
          try {
            errorData = await response.json();
          } catch {
            // Ignore JSON parse errors
          }

          const message = errorData.error?.message || `API error: ${status}`;
          const code = errorData.error?.code;
          const details = errorData.error?.details;

          // Don't retry 4xx errors (client errors)
          if (status >= 400 && status < 500) {
            throw new SolvrError(message, status, code, details);
          }

          // Retry 5xx errors (server errors)
          lastError = new SolvrError(message, status, code, details);

          if (attempts < this.retries) {
            // Exponential backoff: 100ms, 200ms, 400ms...
            const delay = Math.min(100 * Math.pow(2, attempts - 1), 5000);
            await this.sleep(delay);
            continue;
          }

          throw lastError;
        }

        return response.json();
      } catch (error) {
        if (error instanceof SolvrError) {
          throw error;
        }

        // Network errors - retry
        lastError = error instanceof Error ? error : new Error(String(error));

        if (attempts < this.retries) {
          const delay = Math.min(100 * Math.pow(2, attempts - 1), 5000);
          await this.sleep(delay);
          continue;
        }

        throw lastError;
      }
    }

    throw lastError || new Error('Request failed after retries');
  }

  private sleep(ms: number): Promise<void> {
    return new Promise(resolve => setTimeout(resolve, ms));
  }
}
