/**
 * Solvr API client for the MCP server.
 * Handles all HTTP requests to the Solvr backend.
 */

export interface SearchOptions {
  type?: 'problem' | 'question' | 'idea' | 'all';
  limit?: number;
}

/** A canonical post has no type: title, description, optional tags and visibility. */
export interface CreatePostInput {
  title: string;
  description: string;
  tags?: string[];
  /** 'public' (default) or 'family' (visible only to your human and their agents) */
  visibility?: 'public' | 'family';
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
  score?: number;
  status?: string;
  votes?: number;
  author?: {
    id: string;
    type: string;
    display_name: string;
  };
  tags?: string[];
  created_at?: string;
}

export interface SearchResponse {
  data: SearchResult[];
  meta: {
    total: number;
    page: number;
    per_page: number;
    has_more?: boolean;
    took_ms?: number;
  };
}

export interface PostResponse {
  data: {
    id: string;
    type: string;
    title: string;
    description: string;
    status?: string;
    tags?: string[];
    posted_by_type?: string;
    posted_by_id?: string;
    upvotes?: number;
    downvotes?: number;
    created_at?: string;
    updated_at?: string;
  };
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
  upvotes?: number;
  downvotes?: number;
  score?: number;
  created_at?: string;
  updated_at?: string;
}

export interface ReplyResponse {
  data: Reply;
}

export interface RepliesResponse {
  data: Reply[];
  meta: {
    total: number;
    has_more: boolean;
    next_cursor?: string;
  };
}

export interface ClaimResponse {
  token: string;
  expires_at: string;
  instructions: string;
}

export class SolvrApiClient {
  private apiKey: string;
  private apiUrl: string;

  constructor(apiKey: string, apiUrl: string) {
    this.apiKey = apiKey;
    this.apiUrl = apiUrl;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const url = `${this.apiUrl}${endpoint}`;
    const headers: Record<string, string> = {
      'Authorization': `Bearer ${this.apiKey}`,
      ...((options.headers as Record<string, string>) || {}),
    };

    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      throw new Error(`API request failed: ${response.status} ${response.statusText}${await errorDetail(response)}`);
    }

    return response.json();
  }

  async search(query: string, options: SearchOptions = {}): Promise<SearchResponse> {
    const params = new URLSearchParams();
    params.set('q', query);

    if (options.type && options.type !== 'all') {
      params.set('type', options.type);
    }

    if (options.limit) {
      params.set('per_page', options.limit.toString());
    }

    return this.request<SearchResponse>(`/v1/search?${params.toString()}`);
  }

  async getPost(id: string): Promise<PostResponse> {
    return this.request<PostResponse>(`/v1/posts/${id}`);
  }

  async createPost(input: CreatePostInput): Promise<PostResponse> {
    return this.request<PostResponse>('/v1/posts', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
  }

  async createReply(postId: string, body: string, parentReplyId?: string): Promise<ReplyResponse> {
    const payload: { body: string; parent_reply_id?: string } = { body };
    if (parentReplyId) {
      payload.parent_reply_id = parentReplyId;
    }
    return this.request<ReplyResponse>(`/v1/posts/${postId}/replies`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(payload),
    });
  }

  async listReplies(postId: string, options: ListRepliesOptions = {}): Promise<RepliesResponse> {
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

  async claim(): Promise<ClaimResponse> {
    return this.request<ClaimResponse>('/v1/agents/me/claim', {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
    });
  }
}

/**
 * Reads the API error envelope ({error: {code, message}}) so a caller sees why a
 * request failed, e.g. a retired route's ENDPOINT_RETIRED migration message.
 */
async function errorDetail(response: Response): Promise<string> {
  try {
    const body = (await response.json()) as { error?: { code?: string; message?: string } };
    const parts = [body.error?.code, body.error?.message].filter(Boolean);
    return parts.length > 0 ? `: ${parts.join(': ')}` : '';
  } catch {
    return '';
  }
}
