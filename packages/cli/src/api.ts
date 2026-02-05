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

export interface GetOptions {
  include?: string[];
}

export interface CreatePostInput {
  type: "problem" | "question" | "idea";
  title: string;
  description: string;
  tags?: string[];
  success_criteria?: string[];
}

export interface CreateApproachInput {
  angle: string;
  method?: string;
  assumptions?: string[];
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
  success_criteria?: string[];
  approaches?: unknown[];
  answers?: unknown[];
  created_at: string;
  updated_at: string;
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
   * Get a post by ID
   */
  async get(id: string, options: GetOptions = {}): Promise<ApiResponse<Post>> {
    let path = `/v1/posts/${id}`;
    if (options.include && options.include.length > 0) {
      const params = new URLSearchParams();
      params.set("include", options.include.join(","));
      path += `?${params.toString()}`;
    }
    return this.request<ApiResponse<Post>>("GET", path);
  }

  /**
   * Create a new post
   */
  async createPost(input: CreatePostInput): Promise<ApiResponse<Post>> {
    return this.request<ApiResponse<Post>>("POST", "/v1/posts", input);
  }

  /**
   * Create an answer to a question
   */
  async createAnswer(
    questionId: string,
    content: string
  ): Promise<ApiResponse<{ id: string; content: string }>> {
    return this.request<ApiResponse<{ id: string; content: string }>>(
      "POST",
      `/v1/questions/${questionId}/answers`,
      { content }
    );
  }

  /**
   * Create an approach to a problem
   */
  async createApproach(
    problemId: string,
    input: CreateApproachInput
  ): Promise<ApiResponse<{ id: string; angle: string }>> {
    return this.request<ApiResponse<{ id: string; angle: string }>>(
      "POST",
      `/v1/problems/${problemId}/approaches`,
      input
    );
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
