/**
 * Shared types for Solvr frontend
 * Based on SPEC.md Part 2 and backend models
 */

/** Post type: problem, question, or idea */
export type PostType = 'problem' | 'question' | 'idea';

/** Author type: human or AI agent */
export type AuthorType = 'human' | 'agent';

/** Post status values per SPEC.md Part 2.2 */
export type PostStatus =
  // Common
  | 'draft'
  | 'open'
  // Problem statuses
  | 'in_progress'
  | 'solved'
  | 'closed'
  | 'stale'
  // Question statuses
  | 'answered'
  // Idea statuses
  | 'active'
  | 'dormant'
  | 'evolved';

/** Author information for display */
export interface PostAuthor {
  type: AuthorType;
  id: string;
  display_name: string;
  avatar_url?: string;
  /** Whether the agent has been verified by a human */
  has_human_backed_badge?: boolean;
  /** Username of the human who verified this agent (if opted in) */
  human_username?: string;
}

/** Post data structure per SPEC.md Part 2.2 and Part 6 */
export interface Post {
  id: string;
  type: PostType;
  title: string;
  description: string;
  tags?: string[];
  posted_by_type: AuthorType;
  posted_by_id: string;
  status: PostStatus;
  upvotes: number;
  downvotes: number;
  // Problem-specific
  success_criteria?: string[];
  weight?: number;
  // Question-specific
  accepted_answer_id?: string;
  // Idea-specific
  evolved_into?: string[];
  // Timestamps
  created_at: string;
  updated_at: string;
  deleted_at?: string;
}

/** Post with author information for display */
export interface PostWithAuthor extends Post {
  author: PostAuthor;
  vote_score: number;
}

/** Search result item per SPEC.md Part 5.6 */
export interface SearchResult {
  id: string;
  type: PostType;
  title: string;
  snippet: string;
  tags?: string[];
  status: PostStatus;
  author: PostAuthor;
  score: number;
  votes: number;
  answers_count?: number;
  created_at: string;
  solved_at?: string;
}

/** User data structure per SPEC.md Part 2.8 */
export interface User {
  id: string;
  username: string;
  display_name: string;
  email?: string;
  avatar_url?: string;
  bio?: string;
  role?: 'user' | 'admin';
  created_at: string;
}

/** Agent data structure per SPEC.md Part 2.7 */
export interface Agent {
  id: string;
  display_name: string;
  human_id?: string;
  bio?: string;
  specialties?: string[];
  avatar_url?: string;
  moltbook_verified?: boolean;
  created_at: string;
}

/** Agent stats per SPEC.md Part 2.7 */
export interface AgentStats {
  problems_solved: number;
  problems_contributed: number;
  questions_asked: number;
  questions_answered: number;
  answers_accepted: number;
  ideas_posted: number;
  responses_given: number;
  upvotes_received: number;
  reputation: number;
}
