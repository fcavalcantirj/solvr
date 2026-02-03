'use client';

/**
 * AuthorBadge component
 * Displays author avatar, name, and type indicator with link to profile
 * Per SPEC.md Part 4.4: Post cards - author display
 * Per PRD: Show Human-Backed badge next to agent name on posts/answers
 */

import Link from 'next/link';
import { PostAuthor } from '../lib/types';
import HumanBackedBadge from './HumanBackedBadge';

type SizeType = 'sm' | 'md' | 'lg';

interface AuthorBadgeProps {
  /** Author information */
  author: PostAuthor;
  /** Size variant */
  size?: SizeType;
  /** Whether to show the author name */
  showName?: boolean;
}

/**
 * Get avatar size classes based on component size
 */
function getAvatarSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'w-5 h-5';
    case 'lg':
      return 'w-10 h-10';
    default:
      return 'w-6 h-6';
  }
}

/**
 * Get text size classes based on component size
 */
function getTextSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'text-xs';
    case 'lg':
      return 'text-base';
    default:
      return 'text-sm';
  }
}

/**
 * Get badge size classes based on component size
 */
function getBadgeSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'text-[10px] px-1 py-0.5';
    case 'lg':
      return 'text-xs px-2 py-1';
    default:
      return 'text-xs px-1.5 py-0.5';
  }
}

/**
 * AuthorBadge displays author information with link to profile
 */
export default function AuthorBadge({
  author,
  size = 'md',
  showName = true,
}: AuthorBadgeProps) {
  const profileUrl =
    author.type === 'agent'
      ? `/agents/${author.id}`
      : `/users/${author.id}`;

  const avatarSize = getAvatarSize(size);
  const textSize = getTextSize(size);
  const badgeSize = getBadgeSize(size);

  return (
    <Link
      href={profileUrl}
      className="inline-flex items-center gap-2 text-[var(--foreground-secondary)] hover:text-[var(--foreground)] transition-colors"
    >
      {/* Avatar */}
      {author.avatar_url ? (
        <img
          src={author.avatar_url}
          alt={author.display_name}
          className={`${avatarSize} rounded-full`}
        />
      ) : (
        <div
          className={`${avatarSize} rounded-full bg-[var(--color-primary)] flex items-center justify-center text-white font-medium`}
          style={{ fontSize: size === 'sm' ? '10px' : size === 'lg' ? '14px' : '12px' }}
        >
          {author.display_name.charAt(0).toUpperCase()}
        </div>
      )}

      {/* Name */}
      {showName && (
        <span className={textSize}>{author.display_name}</span>
      )}

      {/* AI Badge for agents */}
      {author.type === 'agent' && (
        <span
          className={`${badgeSize} rounded bg-violet-100 text-violet-800 dark:bg-violet-900 dark:text-violet-200 font-medium`}
        >
          AI
        </span>
      )}

      {/* Human-Backed Badge for verified agents */}
      {author.type === 'agent' && author.has_human_backed_badge && (
        <HumanBackedBadge size={size} compact />
      )}
    </Link>
  );
}
