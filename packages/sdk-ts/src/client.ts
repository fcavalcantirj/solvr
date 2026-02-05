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
 * // Get post details
 * const post = await solvr.get('post_abc123', { include: ['approaches'] });
 *
 * // Create a new problem
 * const newPost = await solvr.post({
 *   type: 'problem',
 *   title: 'Memory leak in worker threads',
 *   description: 'Detailed description...',
 *   tags: ['nodejs', 'memory']
 * });
 * ```
 */

import type {
  SolvrConfig,
  SearchOptions,
  SearchResponse,
  GetOptions,
  PostResponse,
  CreatePostInput,
  CreateApproachInput,
  ApproachResponse,
  AnswerResponse,
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
   * Get a post by ID with optional related content.
   *
   * @param id - Post ID
   * @param options - Options for included content
   * @returns Post details
   *
   * @example
   * ```typescript
   * const post = await solvr.get('post_abc123', {
   *   include: ['approaches', 'answers']
   * });
   * ```
   */
  async get(id: string, options: GetOptions = {}): Promise<PostResponse> {
    let endpoint = `/v1/posts/${id}`;

    if (options.include && options.include.length > 0) {
      const params = new URLSearchParams();
      params.set('include', options.include.join(','));
      endpoint += `?${params.toString()}`;
    }

    return this.request<PostResponse>(endpoint);
  }

  /**
   * Create a new post (problem, question, or idea).
   *
   * @param input - Post data
   * @returns Created post
   *
   * @example
   * ```typescript
   * const post = await solvr.post({
   *   type: 'problem',
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
   * Add an approach to a problem.
   *
   * @param problemId - Problem post ID
   * @param input - Approach data
   * @returns Created approach
   *
   * @example
   * ```typescript
   * await solvr.approach('post_abc123', {
   *   angle: 'Connection pool isolation',
   *   content: 'Use separate connection pools...',
   *   method: 'Tested with pg-pool v3.5'
   * });
   * ```
   */
  async approach(problemId: string, input: CreateApproachInput): Promise<ApproachResponse> {
    return this.request<ApproachResponse>(`/v1/problems/${problemId}/approaches`, {
      method: 'POST',
      body: JSON.stringify(input),
    });
  }

  /**
   * Add an answer to a question.
   *
   * @param questionId - Question post ID
   * @param content - Answer content
   * @returns Created answer
   *
   * @example
   * ```typescript
   * await solvr.answer('question_123', 'You can use errgroup from golang.org/x/sync...');
   * ```
   */
  async answer(questionId: string, content: string): Promise<AnswerResponse> {
    return this.request<AnswerResponse>(`/v1/questions/${questionId}/answers`, {
      method: 'POST',
      body: JSON.stringify({ content }),
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
          let errorData: { error?: { message?: string; code?: string } } = {};
          try {
            errorData = await response.json();
          } catch {
            // Ignore JSON parse errors
          }

          const message = errorData.error?.message || `API error: ${status}`;
          const code = errorData.error?.code;

          // Don't retry 4xx errors (client errors)
          if (status >= 400 && status < 500) {
            throw new SolvrError(message, status, code);
          }

          // Retry 5xx errors (server errors)
          lastError = new SolvrError(message, status, code);

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
