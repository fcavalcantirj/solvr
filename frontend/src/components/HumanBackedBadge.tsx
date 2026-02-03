'use client';

/**
 * HumanBackedBadge component
 * Displays a badge indicating an AI agent has been verified by a human.
 * Per PRD requirement: Human-Backed badge display on agent profile and posts
 */

type SizeType = 'sm' | 'md' | 'lg';

interface HumanBackedBadgeProps {
  /** Size variant */
  size?: SizeType;
  /** Username of the human who verified this agent */
  humanUsername?: string;
  /** Whether to show the human's username */
  showHumanHandle?: boolean;
  /** Compact mode - show only icon with sr-only text */
  compact?: boolean;
}

/**
 * Get text size classes based on component size
 */
function getTextSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'text-xs';
    case 'lg':
      return 'text-sm';
    default:
      return 'text-xs';
  }
}

/**
 * Get padding classes based on component size
 */
function getPadding(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'px-1 py-0.5';
    case 'lg':
      return 'px-2 py-1';
    default:
      return 'px-1.5 py-0.5';
  }
}

/**
 * Get icon size based on component size
 */
function getIconSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'w-3 h-3';
    case 'lg':
      return 'w-4 h-4';
    default:
      return 'w-3.5 h-3.5';
  }
}

/**
 * Checkmark icon for verified status
 */
function CheckIcon({ className }: { className?: string }) {
  return (
    <svg
      className={className}
      viewBox="0 0 16 16"
      fill="currentColor"
      aria-hidden="true"
    >
      <path
        fillRule="evenodd"
        d="M12.416 3.376a.75.75 0 0 1 .208 1.04l-5 7.5a.75.75 0 0 1-1.154.114l-3-3a.75.75 0 0 1 1.06-1.06l2.353 2.353 4.493-6.74a.75.75 0 0 1 1.04-.207Z"
        clipRule="evenodd"
      />
    </svg>
  );
}

/**
 * HumanBackedBadge displays verification status for AI agents
 */
export default function HumanBackedBadge({
  size = 'md',
  humanUsername,
  showHumanHandle = false,
  compact = false,
}: HumanBackedBadgeProps) {
  const textSize = getTextSize(size);
  const padding = getPadding(size);
  const iconSize = getIconSize(size);

  // Build title/tooltip text
  const titleText = humanUsername && showHumanHandle
    ? `This agent is verified by ${humanUsername}`
    : 'This agent is verified by a human';

  return (
    <span
      data-testid="human-backed-badge"
      role="status"
      aria-label="Human-Backed verified agent"
      title={titleText}
      className={`
        inline-flex items-center gap-1
        ${padding}
        ${textSize}
        rounded
        bg-emerald-100 text-emerald-800
        dark:bg-emerald-900 dark:text-emerald-200
        font-medium
      `}
    >
      <CheckIcon className={iconSize} />
      <span className={compact ? 'sr-only' : ''}>
        Human-Backed
      </span>
      {showHumanHandle && humanUsername && !compact && (
        <span className="text-emerald-600 dark:text-emerald-300">
          @{humanUsername}
        </span>
      )}
    </span>
  );
}
