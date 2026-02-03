'use client';

/**
 * VoteButtons component
 * Upvote/downvote buttons with score display and optimistic updates
 * Per SPEC.md Part 2.9: Voting rules
 */

import { useState, useCallback } from 'react';

type VoteDirection = 'up' | 'down' | null;
type TargetType = 'post' | 'answer' | 'response' | 'approach';
type LayoutType = 'vertical' | 'horizontal';
type SizeType = 'sm' | 'md' | 'lg';

interface VoteButtonsProps {
  /** Current vote score */
  score: number;
  /** Target entity ID */
  targetId: string;
  /** Target entity type */
  targetType: TargetType;
  /** User's current vote (null if not voted) */
  userVote?: VoteDirection;
  /** Callback when vote changes */
  onVote?: (direction: VoteDirection) => void;
  /** Loading state */
  loading?: boolean;
  /** Disable voting */
  disabled?: boolean;
  /** Layout direction */
  layout?: LayoutType;
  /** Button size */
  size?: SizeType;
}

/**
 * Get icon size based on component size
 */
function getIconSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'h-4 w-4';
    case 'lg':
      return 'h-6 w-6';
    default:
      return 'h-5 w-5';
  }
}

/**
 * Get text size based on component size
 */
function getTextSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'text-sm';
    case 'lg':
      return 'text-lg';
    default:
      return 'text-base';
  }
}

/**
 * Get button padding based on component size
 */
function getButtonPadding(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'p-1';
    case 'lg':
      return 'p-2';
    default:
      return 'p-1.5';
  }
}

/**
 * VoteButtons provides upvote/downvote functionality with optimistic UI updates
 */
export default function VoteButtons({
  score,
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  targetId,
  // eslint-disable-next-line @typescript-eslint/no-unused-vars
  targetType,
  userVote,
  onVote,
  loading = false,
  disabled = false,
  layout = 'vertical',
  size = 'md',
}: VoteButtonsProps) {
  // Track optimistic score and vote state
  const [optimisticScore, setOptimisticScore] = useState(score);
  const [optimisticVote, setOptimisticVote] = useState<VoteDirection>(userVote ?? null);

  // Calculate display score based on optimistic state
  const displayScore = optimisticScore;

  const handleVote = useCallback(
    (direction: 'up' | 'down') => {
      if (loading || disabled) return;

      let newVote: VoteDirection;
      let scoreDelta = 0;

      // Determine new vote state and score change
      if (optimisticVote === direction) {
        // Clicking same vote = remove vote
        newVote = null;
        scoreDelta = direction === 'up' ? -1 : 1;
      } else if (optimisticVote === null) {
        // No current vote = add vote
        newVote = direction;
        scoreDelta = direction === 'up' ? 1 : -1;
      } else {
        // Switching vote = change by 2
        newVote = direction;
        scoreDelta = direction === 'up' ? 2 : -2;
      }

      // Optimistically update state
      setOptimisticVote(newVote);
      setOptimisticScore((prev) => prev + scoreDelta);

      // Call parent handler
      onVote?.(newVote);
    },
    [optimisticVote, loading, disabled, onVote]
  );

  const isUpvoteActive = optimisticVote === 'up';
  const isDownvoteActive = optimisticVote === 'down';
  const isDisabled = loading || disabled;

  const iconSize = getIconSize(size);
  const textSize = getTextSize(size);
  const buttonPadding = getButtonPadding(size);

  const baseButtonClass = `${buttonPadding} rounded transition-colors focus:outline-none focus:ring-2 focus:ring-[var(--color-primary)] focus:ring-offset-1`;
  const activeUpClass = 'text-green-600 bg-green-100 dark:bg-green-900 dark:text-green-400';
  const activeDownClass = 'text-red-600 bg-red-100 dark:bg-red-900 dark:text-red-400';
  const inactiveClass =
    'text-[var(--foreground-muted)] hover:text-[var(--foreground)] hover:bg-[var(--background-secondary)]';
  const disabledClass = 'opacity-50 cursor-not-allowed';

  return (
    <div
      className={`flex items-center gap-1 ${layout === 'vertical' ? 'flex-col' : 'flex-row'}`}
    >
      {/* Upvote Button */}
      <button
        type="button"
        onClick={() => handleVote('up')}
        disabled={isDisabled}
        aria-label="Upvote"
        aria-pressed={isUpvoteActive}
        data-active={isUpvoteActive ? 'true' : undefined}
        className={`${baseButtonClass} ${
          isUpvoteActive ? activeUpClass : inactiveClass
        } ${isDisabled ? disabledClass : ''}`}
      >
        <svg
          className={iconSize}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M5 15l7-7 7 7"
          />
        </svg>
      </button>

      {/* Score */}
      <span
        className={`font-semibold ${textSize} ${
          displayScore > 0
            ? 'text-green-600 dark:text-green-400'
            : displayScore < 0
              ? 'text-red-600 dark:text-red-400'
              : 'text-[var(--foreground-muted)]'
        }`}
      >
        {displayScore}
      </span>

      {/* Downvote Button */}
      <button
        type="button"
        onClick={() => handleVote('down')}
        disabled={isDisabled}
        aria-label="Downvote"
        aria-pressed={isDownvoteActive}
        data-active={isDownvoteActive ? 'true' : undefined}
        className={`${baseButtonClass} ${
          isDownvoteActive ? activeDownClass : inactiveClass
        } ${isDisabled ? disabledClass : ''}`}
      >
        <svg
          className={iconSize}
          fill="none"
          stroke="currentColor"
          viewBox="0 0 24 24"
          aria-hidden="true"
        >
          <path
            strokeLinecap="round"
            strokeLinejoin="round"
            strokeWidth={2}
            d="M19 9l-7 7-7-7"
          />
        </svg>
      </button>
    </div>
  );
}
