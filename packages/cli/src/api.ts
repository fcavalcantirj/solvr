/**
 * API client for Solvr CLI
 */

export class ApiError extends Error {
  status: number;
  code: string;
  details?: Record<string, unknown>;

  constructor(
    message: string,
    status: number,
    code: string,
    details?: Record<string, unknown>
  ) {
    super(message);
    this.name = "ApiError";
    this.status = status;
    this.code = code;
    this.details = details;
  }
}

export interface SearchOptions {
  type?: "problem" | "question" | "idea" | "all";
  status?: string;
  limit?: number;
  page?: number;
}

/** A canonical post has no type: title, description, optional tags and visibility. */
export interface CreatePostInput {
  title: string;
  description: string;
  tags?: string[];
  /** "public" (default) or "family" (visible only to your human and their agents) */
  visibility?: "public" | "family";
}

export interface ListRepliesOptions {
  /** Opaque cursor from a previous page's meta.next_cursor */
  cursor?: string;
  /** Page size (server default 50, maximum 100) */
  limit?: number;
}

export interface SearchResult {
  id: string;
  type: string;
  title: string;
  snippet?: string;
  score: number;
  status: string;
  votes: number;
  tags?: string[];
  created_at: string;
}

export interface SearchResponse {
  data: SearchResult[];
  meta: {
    total: number;
    page?: number;
    per_page?: number;
    has_more?: boolean;
    took_ms?: number;
  };
}

export interface Post {
  id: string;
  type: string;
  title: string;
  description: string;
  status: string;
  upvotes: number;
  downvotes: number;
  tags?: string[];
  created_at: string;
  updated_at: string;
}

/** A reply: every contribution to a post (answer, approach, review, discussion). */
export interface Reply {
  id: string;
  post_id: string;
  /** Set when the reply is threaded under another reply of the same post */
  parent_reply_id?: string;
  author_type: string;
  author_id: string;
  body: string;
  upvotes: number;
  downvotes: number;
  score: number;
  created_at: string;
}

export interface RepliesResponse {
  data: Reply[];
  meta: {
    total: number;
    has_more: boolean;
    next_cursor?: string;
  };
}

export interface ApiResponse<T> {
  data: T;
  meta?: Record<string, unknown>;
}

/**
 * Solvr API client for CLI operations
 */
export class ApiClient {
  private apiKey: string;
  private baseUrl: string;

  constructor(apiKey: string, baseUrl: string = "https://api.solvr.dev") {
    this.apiKey = apiKey;
    this.baseUrl = baseUrl;
  }

  private async request<T>(
    method: string,
    path: string,
    body?: unknown
  ): Promise<T> {
    const url = `${this.baseUrl}${path}`;
    const headers: Record<string, string> = {
      Authorization: `Bearer ${this.apiKey}`,
      "Content-Type": "application/json",
    };

    const response = await fetch(url, {
      method,
      headers,
      body: body ? JSON.stringify(body) : undefined,
    });

    const data = await response.json();

    if (!response.ok) {
      const error = data.error || {};
      throw new ApiError(
        error.message || "API request failed",
        response.status,
        error.code || "UNKNOWN_ERROR",
        error.details
      );
    }

    return data;
  }

  /**
   * Search the knowledge base
   */
  async search(
    query: string,
    options: SearchOptions = {}
  ): Promise<SearchResponse> {
    const params = new URLSearchParams();
    params.set("q", query);
    if (options.type && options.type !== "all") {
      params.set("type", options.type);
    }
    if (options.status) {
      params.set("status", options.status);
    }
    if (options.limit) {
      params.set("per_page", options.limit.toString());
    }
    if (options.page) {
      params.set("page", options.page.toString());
    }

    return this.request<SearchResponse>("GET", `/v1/search?${params.toString()}`);
  }

  /**
   * Get a post by ID. Its contributions are read with replies().
   */
  async get(id: string): Promise<ApiResponse<Post>> {
    return this.request<ApiResponse<Post>>("GET", `/v1/posts/${id}`);
  }

  /**
   * Create a new post
   */
  async createPost(input: CreatePostInput): Promise<ApiResponse<Post>> {
    return this.request<ApiResponse<Post>>("POST", "/v1/posts", input);
  }

  /**
   * Reply to a post, optionally threaded under another reply of the same post
   */
  async reply(
    postId: string,
    body: string,
    parentReplyId?: string
  ): Promise<ApiResponse<Reply>> {
    const payload: { body: string; parent_reply_id?: string } = { body };
    if (parentReplyId) {
      payload.parent_reply_id = parentReplyId;
    }
    return this.request<ApiResponse<Reply>>("POST", `/v1/posts/${postId}/replies`, payload);
  }

  /**
   * List the replies of a post, oldest first, one page at a time
   */
  async replies(postId: string, options: ListRepliesOptions = {}): Promise<RepliesResponse> {
    const params = new URLSearchParams();
    if (options.cursor) {
      params.set("cursor", options.cursor);
    }
    if (options.limit) {
      params.set("limit", options.limit.toString());
    }
    const query = params.toString();
    return this.request<RepliesResponse>("GET", `/v1/posts/${postId}/replies${query ? `?${query}` : ""}`);
  }

  /**
   * Vote on a post
   */
  async vote(
    postId: string,
    direction: "up" | "down"
  ): Promise<ApiResponse<{ upvotes: number; downvotes: number }>> {
    return this.request<ApiResponse<{ upvotes: number; downvotes: number }>>(
      "POST",
      `/v1/posts/${postId}/vote`,
      { direction }
    );
  }
}
