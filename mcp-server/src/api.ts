/**
 * Solvr API client for the MCP server.
 * Handles all HTTP requests to the Solvr backend. The methods are named after the operationIds
 * of contract/openapi-examples.json (createReply keeps its positional form).
 */

import { ApiError } from './errors.js';
import { RoomStream } from './stream.js';
import type {
  CreateRoomEntryInput,
  CreateRoomInput,
  HandshakeRoomInput,
  ListRoomEntriesOptions,
  Room,
  RoomEntriesResponse,
  RoomEntryResponse,
  RoomHandshake,
  RoomStreamTicket,
  StreamRoomOptions,
} from './room-types.js';

export { ApiError };

/** Search covers every post: there is no type or status filter. */
export interface SearchOptions {
  limit?: number;
  page?: number;
  /** relevance (default), newest or votes */
  sort?: string;
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
  author?: {
    id: string;
    type: string;
    display_name?: string;
  };
  body: string;
  upvotes?: number;
  downvotes?: number;
  score?: number;
  created_at?: string;
  updated_at?: string;
  /** The ETag of this version (getReply, updateReply): send it as if_match to edit the reply */
  etag?: string;
}

export interface ReplyResponse {
  data: Reply;
}

export interface ApiResponse<T> {
  data: T;
  meta?: Record<string, unknown>;
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

const seg = encodeURIComponent;

/** The query string of the given parameters, in order, without the absent or empty ones. */
function queryString(params: [string, string | number | undefined][]): string {
  const query = new URLSearchParams();
  for (const [name, value] of params) {
    if (value !== undefined && value !== '') {
      query.set(name, String(value));
    }
  }
  const text = query.toString();
  return text ? `?${text}` : '';
}

/** A request carrying a JSON body. */
function withJSON(method: string, body: unknown, headers: Record<string, string> = {}): RequestInit {
  return {
    method,
    headers: { 'Content-Type': 'application/json', ...headers },
    body: JSON.stringify(body),
  };
}

export class SolvrApiClient {
  private apiKey: string | null;
  private apiUrl: string;

  /** apiKey is the bearer credential (an API key or a room token); null is an anonymous client. */
  constructor(apiKey: string | null, apiUrl: string) {
    this.apiKey = apiKey;
    this.apiUrl = apiUrl;
  }

  private async send(endpoint: string, options: RequestInit = {}): Promise<Response> {
    const url = `${this.apiUrl}${endpoint}`;
    const headers: Record<string, string> = {
      ...(this.apiKey ? { 'Authorization': `Bearer ${this.apiKey}` } : {}),
      ...((options.headers as Record<string, string>) || {}),
    };

    const response = await fetch(url, {
      ...options,
      headers,
    });

    if (!response.ok) {
      throw await apiErrorFrom(response);
    }
    return response;
  }

  private async request<T>(
    endpoint: string,
    options: RequestInit = {}
  ): Promise<T> {
    const response = await this.send(endpoint, options);
    return response.json();
  }

  /** A reply answer with the version's ETag header as data.etag. */
  private async replyWithETag(endpoint: string, options: RequestInit = {}): Promise<ReplyResponse> {
    const response = await this.send(endpoint, options);
    const answer = (await response.json()) as ReplyResponse;
    const etag = response.headers?.get('ETag');
    if (etag) {
      answer.data = { ...answer.data, etag };
    }
    return answer;
  }

  /** Search the knowledge base. An empty query is not sent (the API answers VALIDATION_ERROR). */
  async search(query: string, options: SearchOptions = {}): Promise<SearchResponse> {
    const qs = queryString([
      ['q', query],
      ['per_page', options.limit],
      ['page', options.page],
      ['sort', options.sort],
    ]);
    return this.request<SearchResponse>(`/v1/search${qs}`);
  }

  async getPost(id: string): Promise<PostResponse> {
    return this.request<PostResponse>(`/v1/posts/${seg(id)}`);
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
    return this.request<ReplyResponse>(`/v1/posts/${seg(postId)}/replies`, {
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
    return this.request<RepliesResponse>(`/v1/posts/${seg(postId)}/replies${query ? `?${query}` : ''}`);
  }

  /** Get one reply and the ETag of its version (data.etag). */
  async getReply(id: string): Promise<ReplyResponse> {
    return this.replyWithETag(`/v1/replies/${seg(id)}`);
  }

  /** Edit your reply. ifMatch is the ETag of the version read; empty sends none (the API answers 428). */
  async updateReply(id: string, ifMatch: string, body: string): Promise<ReplyResponse> {
    const headers: Record<string, string> = ifMatch ? { 'If-Match': ifMatch } : {};
    return this.replyWithETag(`/v1/replies/${seg(id)}`, withJSON('PATCH', { body }, headers));
  }

  /** Create a room (API key). */
  async createRoom(input: CreateRoomInput): Promise<ApiResponse<Room>> {
    return this.request<ApiResponse<Room>>('/v1/rooms', withJSON('POST', input));
  }

  /** Join a room (API key): the answer carries the room token of this session. */
  async handshakeRoom(slug: string, input: HandshakeRoomInput = {}): Promise<ApiResponse<RoomHandshake>> {
    return this.request<ApiResponse<RoomHandshake>>(`/v1/rooms/${seg(slug)}/handshake`, withJSON('POST', input));
  }

  /** Read a room's timeline, oldest first (room token). */
  async listRoomEntries(slug: string, options: ListRoomEntriesOptions = {}): Promise<RoomEntriesResponse> {
    const qs = queryString([
      ['cursor', options.cursor],
      ['limit', options.limit],
      ['kind', options.kind],
      ['issue', options.issue],
    ]);
    return this.request<RoomEntriesResponse>(`/v1/rooms/${seg(slug)}/entries${qs}`);
  }

  /** Send a message to a room (room token). */
  async createRoomEntry(slug: string, input: CreateRoomEntryInput): Promise<RoomEntryResponse> {
    return this.request<RoomEntryResponse>(`/v1/rooms/${seg(slug)}/entries`, withJSON('POST', input));
  }

  /** Mint a short-lived ticket that opens the room's stream without a credential (room token). */
  async createRoomStreamTicket(slug: string): Promise<ApiResponse<RoomStreamTicket>> {
    return this.request<ApiResponse<RoomStreamTicket>>(`/v1/rooms/${seg(slug)}/stream-ticket`, { method: 'POST' });
  }

  /** Watch a room's stream (room token, or anonymous with a ticket); signal closes it. */
  async streamRoom(slug: string, options: StreamRoomOptions = {}, signal?: AbortSignal): Promise<RoomStream> {
    const qs = queryString([['ticket', options.ticket], ['type', options.type], ['issue', options.issue]]);
    const headers: Record<string, string> = { 'Accept': 'text/event-stream' };
    if (options.lastEventId) {
      headers['Last-Event-ID'] = options.lastEventId;
    }
    const response = await this.send(`/v1/rooms/${seg(slug)}/stream${qs}`, { headers, signal });
    if (!response.body) {
      throw new Error('the room stream answered no body');
    }
    return new RoomStream(response.body);
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
 * The error of a failed answer. Reads the API error envelope ({error: {code, message,
 * request_id}}) so a caller sees why a request failed, e.g. a retired route's
 * ENDPOINT_RETIRED migration message, and which request to report.
 */
async function apiErrorFrom(response: Response): Promise<ApiError> {
  let error: { code?: string; message?: string; request_id?: string } | undefined;
  try {
    const body = (await response.json()) as { error?: { code?: string; message?: string; request_id?: string } };
    error = body.error;
  } catch {
    error = undefined;
  }
  const parts = [error?.code, error?.message].filter(Boolean);
  const detail = parts.length > 0 ? `: ${parts.join(': ')}` : '';
  return new ApiError(
    `API request failed: ${response.status} ${response.statusText}${detail}`,
    response.status,
    error?.code,
    error?.request_id
  );
}
