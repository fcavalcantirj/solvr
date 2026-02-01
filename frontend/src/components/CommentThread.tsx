'use client';

/**
 * CommentThread component
 * Displays a list of comments with add new comment form and delete functionality
 * Per SPEC.md Part 2.6: Comments
 */

import { useState, useEffect, useCallback } from 'react';
import { api, getAuthToken } from '../lib/api';

/** Target types that can have comments per SPEC.md */
type CommentTargetType = 'approach' | 'answer' | 'response';

/** Author type */
type AuthorType = 'human' | 'agent';

/** Comment author information */
interface CommentAuthor {
  id: string;
  type: AuthorType;
  display_name: string;
  avatar_url?: string;
}

/** Comment data structure per backend models */
interface Comment {
  id: string;
  target_type: CommentTargetType;
  target_id: string;
  author_type: AuthorType;
  author_id: string;
  content: string;
  created_at: string;
  deleted_at?: string;
  author: CommentAuthor;
}

interface CommentThreadProps {
  /** Target type: approach, answer, or response */
  targetType: CommentTargetType;
  /** Target entity ID */
  targetId: string;
  /** Current user ID (for showing delete button) */
  currentUserId?: string;
  /** Whether current user is admin */
  isAdmin?: boolean;
}

/**
 * Format a date string for display
 */
function formatDate(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffMins = Math.floor(diffMs / 60000);
  const diffHours = Math.floor(diffMs / 3600000);
  const diffDays = Math.floor(diffMs / 86400000);

  if (diffMins < 1) return 'just now';
  if (diffMins < 60) return `${diffMins}m ago`;
  if (diffHours < 24) return `${diffHours}h ago`;
  if (diffDays < 7) return `${diffDays}d ago`;

  return date.toLocaleDateString('en-US', {
    month: 'short',
    day: 'numeric',
    year: date.getFullYear() !== now.getFullYear() ? 'numeric' : undefined,
  });
}

/**
 * Get the API endpoint path for comments based on target type
 */
function getCommentsPath(targetType: CommentTargetType, targetId: string): string {
  // Handle irregular plurals
  const pluralMap: Record<CommentTargetType, string> = {
    approach: 'approaches',
    answer: 'answers',
    response: 'responses',
  };
  return `/v1/${pluralMap[targetType]}/${targetId}/comments`;
}

/**
 * CommentThread displays comments and allows adding/deleting comments
 */
export default function CommentThread({
  targetType,
  targetId,
  currentUserId,
  isAdmin = false,
}: CommentThreadProps) {
  const [comments, setComments] = useState<Comment[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [newComment, setNewComment] = useState('');
  const [submitting, setSubmitting] = useState(false);
  const [submitError, setSubmitError] = useState<string | null>(null);
  const [deleteError, setDeleteError] = useState<string | null>(null);

  const isAuthenticated = !!getAuthToken();

  // Fetch comments on mount
  useEffect(() => {
    let mounted = true;

    async function fetchComments() {
      setLoading(true);
      setError(null);

      try {
        const path = getCommentsPath(targetType, targetId);
        const data = await api.get<Comment[]>(path);
        if (mounted) {
          setComments(data);
          setLoading(false);
        }
      } catch (err) {
        if (mounted) {
          setError('Error loading comments');
          setLoading(false);
        }
      }
    }

    fetchComments();

    return () => {
      mounted = false;
    };
  }, [targetType, targetId]);

  // Handle new comment submission
  const handleSubmit = useCallback(
    async (e: React.FormEvent) => {
      e.preventDefault();

      if (!newComment.trim() || submitting) return;

      setSubmitting(true);
      setSubmitError(null);

      try {
        const path = getCommentsPath(targetType, targetId);
        const createdComment = await api.post<Comment>(path, {
          content: newComment.trim(),
        });

        // Add the new comment to the list
        setComments((prev) => [...prev, createdComment]);
        setNewComment('');
      } catch (err) {
        setSubmitError('Failed to post comment');
      } finally {
        setSubmitting(false);
      }
    },
    [targetType, targetId, newComment, submitting]
  );

  // Handle comment deletion
  const handleDelete = useCallback(
    async (commentId: string) => {
      setDeleteError(null);

      try {
        await api.delete(`/v1/comments/${commentId}`);
        // Remove the deleted comment from the list
        setComments((prev) => prev.filter((c) => c.id !== commentId));
      } catch (err) {
        setDeleteError('Failed to delete comment');
      }
    },
    []
  );

  // Check if current user can delete a comment
  const canDelete = (comment: Comment): boolean => {
    if (!currentUserId) return false;
    if (isAdmin) return true;
    return comment.author_type === 'human' && comment.author_id === currentUserId;
  };

  // Comment count text
  const commentCountText = comments.length === 1 ? '1 comment' : `${comments.length} comments`;

  return (
    <div className="space-y-4">
      {/* Header with count */}
      <h3 className="text-lg font-semibold text-[var(--foreground)]">
        {loading ? 'Comments' : commentCountText}
      </h3>

      {/* Error states */}
      {error && (
        <div className="p-3 rounded-md bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 text-sm">
          {error}
        </div>
      )}

      {deleteError && (
        <div className="p-3 rounded-md bg-red-50 dark:bg-red-900/20 text-red-600 dark:text-red-400 text-sm">
          {deleteError}
        </div>
      )}

      {/* Loading state */}
      {loading && (
        <div className="text-[var(--foreground-muted)] text-sm">Loading comments...</div>
      )}

      {/* Empty state */}
      {!loading && !error && comments.length === 0 && (
        <div className="text-[var(--foreground-muted)] text-sm py-4">
          No comments yet. Be the first to comment!
        </div>
      )}

      {/* Comments list */}
      {!loading && !error && comments.length > 0 && (
        <ul role="list" className="space-y-4">
          {comments.map((comment) => (
            <li
              key={comment.id}
              className="p-4 rounded-lg bg-[var(--background-secondary)] border border-[var(--border)]"
            >
              {/* Comment header */}
              <div className="flex items-center justify-between mb-2">
                <div className="flex items-center gap-2">
                  {/* Avatar */}
                  {comment.author.avatar_url ? (
                    <img
                      src={comment.author.avatar_url}
                      alt=""
                      className="w-6 h-6 rounded-full"
                    />
                  ) : (
                    <div className="w-6 h-6 rounded-full bg-[var(--primary)] flex items-center justify-center text-white text-xs font-medium">
                      {comment.author.display_name.charAt(0).toUpperCase()}
                    </div>
                  )}
                  {/* Author name */}
                  <span className="font-medium text-sm text-[var(--foreground)]">
                    {comment.author.display_name}
                  </span>
                  {/* Author type badge */}
                  {comment.author.type === 'agent' && (
                    <span className="px-1.5 py-0.5 text-xs rounded bg-purple-100 dark:bg-purple-900/30 text-purple-600 dark:text-purple-400">
                      AI
                    </span>
                  )}
                  {/* Timestamp */}
                  <span className="text-xs text-[var(--foreground-muted)]">
                    {formatDate(comment.created_at)}
                  </span>
                </div>

                {/* Delete button */}
                {canDelete(comment) && (
                  <button
                    type="button"
                    onClick={() => handleDelete(comment.id)}
                    aria-label="Delete comment"
                    className="text-[var(--foreground-muted)] hover:text-red-600 dark:hover:text-red-400 transition-colors p-1"
                  >
                    <svg
                      className="w-4 h-4"
                      fill="none"
                      stroke="currentColor"
                      viewBox="0 0 24 24"
                      aria-hidden="true"
                    >
                      <path
                        strokeLinecap="round"
                        strokeLinejoin="round"
                        strokeWidth={2}
                        d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16"
                      />
                    </svg>
                  </button>
                )}
              </div>

              {/* Comment content */}
              <p className="text-sm text-[var(--foreground)] whitespace-pre-wrap">
                {comment.content}
              </p>
            </li>
          ))}
        </ul>
      )}

      {/* New comment form */}
      {isAuthenticated ? (
        <form onSubmit={handleSubmit} className="space-y-3">
          <textarea
            value={newComment}
            onChange={(e) => setNewComment(e.target.value)}
            placeholder="Add a comment..."
            aria-label="Add a comment"
            rows={3}
            maxLength={2000}
            className="w-full px-3 py-2 rounded-lg border border-[var(--border)] bg-[var(--background)] text-[var(--foreground)] placeholder-[var(--foreground-muted)] focus:outline-none focus:ring-2 focus:ring-[var(--primary)] resize-none"
          />

          {submitError && (
            <div className="text-red-600 dark:text-red-400 text-sm">{submitError}</div>
          )}

          <div className="flex justify-end">
            <button
              type="submit"
              disabled={!newComment.trim() || submitting}
              className="px-4 py-2 rounded-lg bg-[var(--primary)] text-white font-medium text-sm hover:opacity-90 disabled:opacity-50 disabled:cursor-not-allowed transition-opacity"
            >
              {submitting ? 'Posting...' : 'Post Comment'}
            </button>
          </div>
        </form>
      ) : (
        <div className="text-center py-4 text-[var(--foreground-muted)] text-sm">
          <a
            href="/auth/login"
            className="text-[var(--primary)] hover:underline"
          >
            Log in to comment
          </a>
        </div>
      )}
    </div>
  );
}
