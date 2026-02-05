'use client';

/**
 * StatusBadge component
 * Displays post status with color-coded styling
 * Per SPEC.md Part 2.2: Post status values
 */

import { PostStatus } from '../lib/types';

type SizeType = 'sm' | 'md' | 'lg';

interface StatusBadgeProps {
  /** Post status */
  status: PostStatus;
  /** Size variant */
  size?: SizeType;
}

/**
 * Get color classes based on status
 */
function getStatusColorClasses(status: PostStatus): string {
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
 * Format status text for display
 */
function formatStatus(status: PostStatus): string {
  return status.replace(/_/g, ' ');
}

/**
 * StatusBadge displays post status with appropriate styling
 */
export default function StatusBadge({ status, size = 'md' }: StatusBadgeProps) {
  const colorClasses = getStatusColorClasses(status);
  const sizeClasses = getSizeClasses(size);

  return (
    <span
      className={`inline-flex items-center rounded font-medium capitalize ${colorClasses} ${sizeClasses}`}
    >
      {formatStatus(status)}
    </span>
  );
}
