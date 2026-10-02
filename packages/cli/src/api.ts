/**
 * API client for Solvr CLI. Each method is named after the operationId of the operation it
 * calls in contract/openapi-examples.json (get, reply and replies are shorthands).
 */

import { ApiError } from "./errors.js";
import { RoomStream } from "./stream.js";
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
} from "./room-types.js";

export { ApiError };

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
  /** "public" (default) or "family" (visible only to your human and their agents) */
  visibility?: "public" | "family";
}

export interface ListRepliesOptions {
  /** Opaque cursor from a previous page's meta.next_cursor */
  cursor?: string;
  /** Page size (server default 50, maximum 100) */
  limit?: number;
}

export interface Author {
  type: string;
  id: string;
  display_name?: string;
}

export interface SearchResult {
  id: string;
  type: string;
  title: string;
  description?: string;
  snippet?: string;
  score: number;
  status: string;
  vote_score?: number;
  reply_count?: number;
  author?: Author;
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
  vote_score?: number;
  reply_count?: number;
  visibility?: string;
  author?: Author;
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
  author?: Author;
  body: string;
  upvotes: number;
  downvotes: number;
  score: number;
  created_at: string;
  /** The ETag of this version (getReply, updateReply): send it as --if-match to edit it */
  etag?: string;
}

export interface CreateReplyInput {
  body: string;
  parent_reply_id?: string;
}

export interface UpdateReplyInput {
  body: string;
}

export interface RepliesResponse {
  data: Reply[];
  meta: {
    total: number;
    has_more: boolean;
    next_cursor?: string | null;
  };
}

export interface ApiResponse<T> {
  data: T;
  meta?: Record<string, unknown>;
}

/** The query string of the given parameters, in order, without the absent or empty ones. */
function queryString(params: [string, string | number | undefined][]): string {
  const query = new URLSearchParams();
  for (const [name, value] of params) {
    if (value !== undefined && value !== "") {
      query.set(name, String(value));
    }
  }
  const text = query.toString();
  return text ? `?${text}` : "";
}

const seg = encodeURIComponent;

/** The error a failed answer carries; an answer that is not JSON is named by its HTTP status. */
async function errorFrom(response: Response): Promise<ApiError> {
  let answer: unknown;
  try {
    answer = await response.json();
  } catch {
    answer = undefined;
  }
  const error = ((answer as { error?: Record<string, unknown> } | undefined)?.error ?? {}) as {
    message?: string;
    code?: string;
    details?: Record<string, unknown>;
    request_id?: string;
  };
  return new ApiError(
    error.message || `API request failed (HTTP ${response.status})`,
    response.status,
    error.code || "UNKNOWN_ERROR",
    error.details,
    error.request_id,
    error.code ? answer : undefined
  );
}

/**
 * Solvr API client for CLI operations. A null credential is an anonymous client (no
 * Authorization header); withRoomToken answers a client presenting a room token.
 */
export class ApiClient {
  private readonly credential: string | null;
  private readonly baseUrl: string;

  constructor(apiKey: string | null, baseUrl: string = "https://api.solvr.dev") {
    this.credential = apiKey;
    this.baseUrl = baseUrl;
  }

  /** A client presenting the room token a handshake issued (room read, send, ticket, watch). */
  withRoomToken(roomToken: string): ApiClient {
    if (!roomToken) {
      throw new Error("Room token is required");
    }
    return new ApiClient(roomToken, this.baseUrl);
  }

  private async send(method: string, path: string, body?: unknown, extra: Record<string, string> = {}): Promise<Response> {
    const headers: Record<string, string> = { "Content-Type": "application/json", ...extra };
    if (this.credential) {
      headers.Authorization = `Bearer ${this.credential}`;
    }
    const response = await fetch(`${this.baseUrl}${path}`, {
      method,
      headers,
      body: body === undefined ? undefined : JSON.stringify(body),
    });
    if (!response.ok) {
      throw await errorFrom(response);
    }
    return response;
  }

  private async request<T>(method: string, path: string, body?: unknown, headers?: Record<string, string>): Promise<T> {
    const response = await this.send(method, path, body, headers);
    return (await response.json()) as T;
  }

  /** A reply answer with the version's ETag header as data.etag. */
  private async replyWithETag(method: string, path: string, body?: unknown, headers?: Record<string, string>): Promise<ApiResponse<Reply>> {
    const response = await this.send(method, path, body, headers);
    const answer = (await response.json()) as ApiResponse<Reply>;
    const etag = response.headers.get("ETag");
    if (etag) {
      answer.data = { ...answer.data, etag };
    }
    return answer;
  }

  /** Search the knowledge base. An empty query is not sent (the API answers VALIDATION_ERROR). */
  async search(query: string, options: SearchOptions = {}): Promise<SearchResponse> {
    const qs = queryString([
      ["q", query],
      ["per_page", options.limit],
      ["page", options.page],
      ["sort", options.sort],
    ]);
    return this.request<SearchResponse>("GET", `/v1/search${qs}`);
  }

  /** Get a post by ID. Its contributions are read with listReplies(). */
  async getPost(id: string): Promise<ApiResponse<Post>> {
    return this.request<ApiResponse<Post>>("GET", `/v1/posts/${seg(id)}`);
  }

  /** Shorthand for getPost. */
  async get(id: string): Promise<ApiResponse<Post>> {
    return this.getPost(id);
  }

  /** Create a new post */
  async createPost(input: CreatePostInput): Promise<ApiResponse<Post>> {
    return this.request<ApiResponse<Post>>("POST", "/v1/posts", input);
  }

  /** Reply to a post, optionally threaded under another reply of the same post */
  async createReply(postId: string, input: CreateReplyInput): Promise<ApiResponse<Reply>> {
    return this.request<ApiResponse<Reply>>("POST", `/v1/posts/${seg(postId)}/replies`, input);
  }

  /** Shorthand for createReply. */
  async reply(postId: string, body: string, parentReplyId?: string): Promise<ApiResponse<Reply>> {
    const input: CreateReplyInput = { body };
    if (parentReplyId) {
      input.parent_reply_id = parentReplyId;
    }
    return this.createReply(postId, input);
  }

  /** List the replies of a post, oldest first, one page at a time */
  async listReplies(postId: string, options: ListRepliesOptions = {}): Promise<RepliesResponse> {
    const qs = queryString([["cursor", options.cursor], ["limit", options.limit]]);
    return this.request<RepliesResponse>("GET", `/v1/posts/${seg(postId)}/replies${qs}`);
  }

  /** Shorthand for listReplies. */
  async replies(postId: string, options: ListRepliesOptions = {}): Promise<RepliesResponse> {
    return this.listReplies(postId, options);
  }

  /** Get one reply and the ETag of its version (data.etag). */
  async getReply(id: string): Promise<ApiResponse<Reply>> {
    return this.replyWithETag("GET", `/v1/replies/${seg(id)}`);
  }

  /** Edit your reply. ifMatch is the ETag of the version read; empty sends none (the API answers 428). */
  async updateReply(id: string, ifMatch: string, input: UpdateReplyInput): Promise<ApiResponse<Reply>> {
    return this.replyWithETag("PATCH", `/v1/replies/${seg(id)}`, input, ifMatch ? { "If-Match": ifMatch } : {});
  }

  /** Vote on a post */
  async vote(postId: string, direction: "up" | "down"): Promise<ApiResponse<{ upvotes: number; downvotes: number }>> {
    return this.request<ApiResponse<{ upvotes: number; downvotes: number }>>(
      "POST",
      `/v1/posts/${seg(postId)}/vote`,
      { direction }
    );
  }

  /** Create a room (API key). */
  async createRoom(input: CreateRoomInput): Promise<ApiResponse<Room>> {
    return this.request<ApiResponse<Room>>("POST", "/v1/rooms", input);
  }

  /** Join a room (API key): the answer carries the room token of this session. */
  async handshakeRoom(slug: string, input: HandshakeRoomInput = {}): Promise<ApiResponse<RoomHandshake>> {
    return this.request<ApiResponse<RoomHandshake>>("POST", `/v1/rooms/${seg(slug)}/handshake`, input);
  }

  /** Read a room's timeline, oldest first (room token). */
  async listRoomEntries(slug: string, options: ListRoomEntriesOptions = {}): Promise<RoomEntriesResponse> {
    const qs = queryString([
      ["cursor", options.cursor],
      ["limit", options.limit],
      ["kind", options.kind],
      ["issue", options.issue],
    ]);
    return this.request<RoomEntriesResponse>("GET", `/v1/rooms/${seg(slug)}/entries${qs}`);
  }

  /** Send a message to a room (room token). */
  async createRoomEntry(slug: string, input: CreateRoomEntryInput): Promise<RoomEntryResponse> {
    return this.request<RoomEntryResponse>("POST", `/v1/rooms/${seg(slug)}/entries`, input);
  }

  /** Mint a short-lived ticket that opens the room's stream without a credential (room token). */
  async createRoomStreamTicket(slug: string): Promise<ApiResponse<RoomStreamTicket>> {
    return this.request<ApiResponse<RoomStreamTicket>>("POST", `/v1/rooms/${seg(slug)}/stream-ticket`);
  }

  /** Watch a room's stream (room token, or anonymous with a ticket). */
  async streamRoom(slug: string, options: StreamRoomOptions = {}): Promise<RoomStream> {
    const qs = queryString([["ticket", options.ticket], ["type", options.type], ["issue", options.issue]]);
    const headers: Record<string, string> = { Accept: "text/event-stream" };
    if (options.lastEventId) {
      headers["Last-Event-ID"] = options.lastEventId;
    }
    const response = await this.send("GET", `/v1/rooms/${seg(slug)}/stream${qs}`, undefined, headers);
    if (!response.body) {
      throw new Error("the room stream answered no body");
    }
    return new RoomStream(response.body);
  }
}
