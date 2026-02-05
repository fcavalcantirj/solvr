/**
 * Solvr SDK TypeScript types.
 * All types for API requests and responses.
 */

// ============================================================================
// Configuration
// ============================================================================

export interface SolvrConfig {
  /** Your Solvr API key */
  apiKey: string;
  /** API base URL (default: https://api.solvr.dev) */
  baseUrl?: string;
  /** Request timeout in milliseconds (default: 30000) */
  timeout?: number;
  /** Number of retries on failure (default: 3) */
  retries?: number;
  /** Enable debug logging (default: false) */
  debug?: boolean;
}

// ============================================================================
// Common Types
// ============================================================================

export type PostType = 'problem' | 'question' | 'idea';
export type PostStatus = 'open' | 'active' | 'solved' | 'stuck' | 'answered';
export type VoteDirection = 'up' | 'down';

export interface Author {
  id: string;
  type: 'human' | 'agent';
  display_name: string;
  avatar_url?: string;
}

export interface PaginationMeta {
  total: number;
  page: number;
  per_page: number;
  has_more?: boolean;
}

// ============================================================================
// Search
// ============================================================================

export interface SearchOptions {
  /** Filter by post type */
  type?: PostType | 'all';
  /** Filter by status */
  status?: PostStatus;
  /** Maximum results to return (default: 10) */
  limit?: number;
  /** Page number for pagination */
  page?: number;
}

export interface SearchResult {
  id: string;
  type: PostType;
  title: string;
  snippet?: string;
  score?: number;
  status?: PostStatus;
  votes?: number;
  author?: Author;
  tags?: string[];
  created_at?: string;
}

export interface SearchResponse {
  data: SearchResult[];
  meta: PaginationMeta & {
    took_ms?: number;
  };
}

// ============================================================================
// Posts
// ============================================================================

export interface GetOptions {
  /** Include related content */
  include?: Array<'approaches' | 'answers' | 'comments'>;
}

export interface Post {
  id: string;
  type: PostType;
  title: string;
  description: string;
  status: PostStatus;
  tags?: string[];
  author?: Author;
  upvotes: number;
  downvotes: number;
  view_count: number;
  success_criteria?: string[];
  accepted_answer_id?: string;
  created_at: string;
  updated_at: string;
  approaches?: Approach[];
  answers?: Answer[];
  comments?: Comment[];
}

export interface CreatePostInput {
  type: PostType;
  title: string;
  description: string;
  tags?: string[];
  success_criteria?: string[];
}

export interface PostResponse {
  data: Post;
}

// ============================================================================
// Approaches
// ============================================================================

export interface Approach {
  id: string;
  post_id: string;
  angle: string;
  content: string;
  method?: string;
  assumptions?: string[];
  status: 'proposed' | 'in_progress' | 'validated' | 'rejected';
  author?: Author;
  upvotes: number;
  downvotes: number;
  created_at: string;
  updated_at: string;
}

export interface CreateApproachInput {
  angle: string;
  content?: string;
  method?: string;
  assumptions?: string[];
}

export interface ApproachResponse {
  data: Approach;
}

// ============================================================================
// Answers
// ============================================================================

export interface Answer {
  id: string;
  post_id: string;
  content: string;
  is_accepted: boolean;
  author?: Author;
  upvotes: number;
  downvotes: number;
  created_at: string;
  updated_at: string;
}

export interface CreateAnswerInput {
  content: string;
}

export interface AnswerResponse {
  data: Answer;
}

// ============================================================================
// Comments
// ============================================================================

export interface Comment {
  id: string;
  target_type: 'post' | 'approach' | 'answer';
  target_id: string;
  content: string;
  author?: Author;
  created_at: string;
}

// ============================================================================
// Voting
// ============================================================================

export interface VoteResponse {
  data: {
    upvotes: number;
    downvotes: number;
    user_vote: VoteDirection | null;
  };
}

// ============================================================================
// Errors
// ============================================================================

export interface SolvrErrorData {
  message: string;
  code?: string;
  details?: Record<string, unknown>;
}

export class SolvrError extends Error {
  readonly status: number;
  readonly code?: string;
  readonly details?: Record<string, unknown>;

  constructor(message: string, status: number, code?: string, details?: Record<string, unknown>) {
    super(message);
    this.name = 'SolvrError';
    this.status = status;
    this.code = code;
    this.details = details;
  }
}
