/**
 * Solvr API client for the MCP server.
 * Handles all HTTP requests to the Solvr backend.
 */

export interface SearchOptions {
  type?: 'problem' | 'question' | 'idea' | 'all';
  limit?: number;
}

export interface GetPostOptions {
  include?: Array<'approaches' | 'answers' | 'comments'>;
}

export interface CreatePostInput {
  type: 'problem' | 'question' | 'idea';
  title: string;
  description: string;
  tags?: string[];
  success_criteria?: string[];
}

export interface CreateApproachInput {
  angle: string;
  content: string;
  method?: string;
  assumptions?: string[];
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
    approaches?: Array<Record<string, unknown>>;
    answers?: Array<Record<string, unknown>>;
    comments?: Array<Record<string, unknown>>;
  };
}

export interface AnswerResponse {
  data: {
    id: string;
    content: string;
    author_type?: string;
    author_id?: string;
    created_at?: string;
  };
}

export interface ApproachResponse {
  data: {
    id: string;
    angle: string;
    content?: string;
    status?: string;
    created_at?: string;
  };
}

export interface ClaimResponse {
  claim_url: string;
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
      throw new Error(`API request failed: ${response.status} ${response.statusText}`);
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

  async getPost(id: string, options: GetPostOptions = {}): Promise<PostResponse> {
    let endpoint = `/v1/posts/${id}`;

    if (options.include && options.include.length > 0) {
      const params = new URLSearchParams();
      params.set('include', options.include.join(','));
      endpoint += `?${params.toString()}`;
    }

    return this.request<PostResponse>(endpoint);
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

  async createAnswer(questionId: string, content: string): Promise<AnswerResponse> {
    return this.request<AnswerResponse>(`/v1/questions/${questionId}/answers`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify({ content }),
    });
  }

  async createApproach(problemId: string, input: CreateApproachInput): Promise<ApproachResponse> {
    return this.request<ApproachResponse>(`/v1/problems/${problemId}/approaches`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(input),
    });
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
