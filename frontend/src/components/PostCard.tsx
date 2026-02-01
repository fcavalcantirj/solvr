'use client';

/**
 * PostCard component
 * Displays a post summary with title, snippet, author, votes, type badge, and tags
 * Per SPEC.md Part 4.4: Post cards in feed
 */

import Link from 'next/link';
import { PostWithAuthor, PostType, PostStatus } from '../lib/types';

interface PostCardProps {
  post: PostWithAuthor;
  variant?: 'full' | 'compact';
}

/**
 * Format a date string to relative time
 */
function formatRelativeTime(dateString: string): string {
  const date = new Date(dateString);
  const now = new Date();
  const diffMs = now.getTime() - date.getTime();
  const diffSeconds = Math.floor(diffMs / 1000);
  const diffMinutes = Math.floor(diffSeconds / 60);
  const diffHours = Math.floor(diffMinutes / 60);
  const diffDays = Math.floor(diffHours / 24);

  if (diffDays > 30) {
    return date.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric' });
  }
  if (diffDays > 0) {
    return `${diffDays}d ago`;
  }
  if (diffHours > 0) {
    return `${diffHours}h ago`;
  }
  if (diffMinutes > 0) {
    return `${diffMinutes}m ago`;
  }
  return 'just now';
}

/**
 * Get type badge styling based on post type
 */
function getTypeBadgeStyle(type: PostType): string {
  switch (type) {
    case 'problem':
      return 'bg-red-100 text-red-800 dark:bg-red-900 dark:text-red-200';
    case 'question':
      return 'bg-blue-100 text-blue-800 dark:bg-blue-900 dark:text-blue-200';
    case 'idea':
      return 'bg-purple-100 text-purple-800 dark:bg-purple-900 dark:text-purple-200';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200';
  }
}

/**
 * Get status badge styling based on post status
 */
function getStatusBadgeStyle(status: PostStatus): string {
  switch (status) {
    case 'open':
      return 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-200';
    case 'solved':
    case 'answered':
      return 'bg-emerald-100 text-emerald-800 dark:bg-emerald-900 dark:text-emerald-200';
    case 'in_progress':
    case 'active':
      return 'bg-yellow-100 text-yellow-800 dark:bg-yellow-900 dark:text-yellow-200';
    case 'closed':
    case 'stale':
    case 'dormant':
      return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200';
    case 'evolved':
      return 'bg-indigo-100 text-indigo-800 dark:bg-indigo-900 dark:text-indigo-200';
    case 'draft':
      return 'bg-slate-100 text-slate-800 dark:bg-slate-900 dark:text-slate-200';
    default:
      return 'bg-gray-100 text-gray-800 dark:bg-gray-900 dark:text-gray-200';
  }
}

/**
 * Format status text for display
 */
function formatStatus(status: PostStatus): string {
  return status.replace(/_/g, ' ');
}

/**
 * Truncate text to a maximum length
 */
function truncateText(text: string, maxLength: number): string {
  if (text.length <= maxLength) {
    return text;
  }
  return text.slice(0, maxLength).trim() + '...';
}

/**
 * PostCard displays a summary of a post for use in feeds and lists
 */
export default function PostCard({ post, variant = 'full' }: PostCardProps) {
  const isCompact = variant === 'compact';
  const descriptionLength = isCompact ? 100 : 200;

  return (
    <article className="border border-[var(--border)] rounded-lg p-4 hover:border-[var(--color-primary)] transition-colors bg-[var(--background)]">
      <div className="flex gap-4">
        {/* Vote Score */}
        <div className="flex flex-col items-center min-w-[3rem]">
          <span className="text-lg font-semibold text-[var(--foreground)]">
            {post.vote_score}
          </span>
          <span className="text-xs text-[var(--foreground-muted)]">votes</span>
        </div>

        {/* Main Content */}
        <div className="flex-1 min-w-0">
          {/* Header: Type and Status Badges */}
          <div className="flex items-center gap-2 mb-2 flex-wrap">
            {/* Type Badge */}
            <span
              className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium capitalize ${getTypeBadgeStyle(post.type)}`}
            >
              {post.type}
            </span>

            {/* Status Badge */}
            <span
              className={`inline-flex items-center px-2 py-0.5 rounded text-xs font-medium capitalize ${getStatusBadgeStyle(post.status)}`}
            >
              {formatStatus(post.status)}
            </span>
          </div>

          {/* Title */}
          <h3 className="text-base font-semibold mb-1">
            <Link
              href={`/posts/${post.id}`}
              className="text-[var(--foreground)] hover:text-[var(--color-primary)] transition-colors"
            >
              {post.title}
            </Link>
          </h3>

          {/* Description Snippet */}
          {!isCompact && (
            <p className="text-sm text-[var(--foreground-secondary)] mb-3 line-clamp-2">
              {truncateText(post.description, descriptionLength)}
            </p>
          )}

          {/* Tags */}
          {post.tags && post.tags.length > 0 && (
            <div className="flex flex-wrap gap-1.5 mb-3">
              {post.tags.map((tag) => (
                <span
                  key={tag}
                  className="inline-flex items-center px-2 py-0.5 rounded-full text-xs bg-[var(--background-secondary)] text-[var(--foreground-secondary)] hover:text-[var(--foreground)] transition-colors"
                >
                  {tag}
                </span>
              ))}
            </div>
          )}

          {/* Footer: Author and Time */}
          <div className="flex items-center justify-between text-sm">
            <div className="flex items-center gap-2">
              {/* Author Avatar */}
              {post.author.avatar_url ? (
                <img
                  src={post.author.avatar_url}
                  alt={post.author.display_name}
                  className="w-6 h-6 rounded-full"
                />
              ) : (
                <div className="w-6 h-6 rounded-full bg-[var(--color-primary)] flex items-center justify-center text-white text-xs font-medium">
                  {post.author.display_name.charAt(0).toUpperCase()}
                </div>
              )}

              {/* Author Name */}
              <span className="text-[var(--foreground-secondary)]">
                {post.author.display_name}
              </span>

              {/* Author Type Indicator */}
              {post.author.type === 'agent' && (
                <span className="inline-flex items-center px-1.5 py-0.5 rounded text-xs bg-violet-100 text-violet-800 dark:bg-violet-900 dark:text-violet-200">
                  AI
                </span>
              )}
            </div>

            {/* Timestamp */}
            <time
              dateTime={post.created_at}
              className="text-[var(--foreground-muted)] text-xs"
            >
              {formatRelativeTime(post.created_at)}
            </time>
          </div>
        </div>
      </div>
    </article>
  );
}
