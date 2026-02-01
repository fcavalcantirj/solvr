'use client';

/**
 * TagChip component
 * Displays a tag with optional link to search
 * Per SPEC.md Part 4.4: Post cards - tags display
 */

import Link from 'next/link';

type SizeType = 'sm' | 'md' | 'lg';

interface TagChipProps {
  /** Tag text */
  tag: string;
  /** Size variant */
  size?: SizeType;
  /** Whether clicking the tag links to search */
  clickable?: boolean;
  /** Custom CSS classes */
  className?: string;
}

/**
 * Get size classes based on component size
 */
function getSizeClasses(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'text-xs px-1.5 py-0.5';
    case 'lg':
      return 'text-sm px-3 py-1';
    default:
      return 'text-xs px-2 py-0.5';
  }
}

/**
 * TagChip displays a tag with optional search link
 */
export default function TagChip({
  tag,
  size = 'md',
  clickable = true,
  className = '',
}: TagChipProps) {
  const sizeClasses = getSizeClasses(size);
  const baseClasses = `${sizeClasses} rounded-full bg-[var(--background-secondary)] text-[var(--foreground-secondary)] transition-colors`;
  const hoverClasses = clickable
    ? 'hover:text-[var(--foreground)] hover:bg-[var(--background-tertiary)]'
    : '';

  if (clickable) {
    return (
      <Link
        href={`/search?tags=${encodeURIComponent(tag)}`}
        className={`inline-flex items-center ${baseClasses} ${hoverClasses} ${className}`}
      >
        <span className="rounded-full">{tag}</span>
      </Link>
    );
  }

  return (
    <span className={`inline-flex items-center ${baseClasses} ${className}`}>
      <span className="rounded-full">{tag}</span>
    </span>
  );
}
