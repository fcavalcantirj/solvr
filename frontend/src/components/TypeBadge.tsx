'use client';

/**
 * TypeBadge component
 * Displays post type with icon and color-coded styling
 * Per SPEC.md Part 2.2: Post types (problem, question, idea)
 */

import { PostType } from '../lib/types';

type SizeType = 'sm' | 'md' | 'lg';

interface TypeBadgeProps {
  /** Post type */
  type: PostType;
  /** Size variant */
  size?: SizeType;
  /** Whether to show the label text */
  showLabel?: boolean;
}

/**
 * Get color classes based on type
 */
function getTypeColorClasses(type: PostType): string {
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
 * Get icon size based on component size
 */
function getIconSize(size: SizeType): string {
  switch (size) {
    case 'sm':
      return 'h-3 w-3';
    case 'lg':
      return 'h-5 w-5';
    default:
      return 'h-4 w-4';
  }
}

/**
 * Problem icon - exclamation mark in triangle
 */
function ProblemIcon({ className }: { className: string }) {
  return (
    <svg
      className={className}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z"
      />
    </svg>
  );
}

/**
 * Question icon - question mark in circle
 */
function QuestionIcon({ className }: { className: string }) {
  return (
    <svg
      className={className}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M8.228 9c.549-1.165 2.03-2 3.772-2 2.21 0 4 1.343 4 3 0 1.4-1.278 2.575-3.006 2.907-.542.104-.994.54-.994 1.093m0 3h.01M21 12a9 9 0 11-18 0 9 9 0 0118 0z"
      />
    </svg>
  );
}

/**
 * Idea icon - light bulb
 */
function IdeaIcon({ className }: { className: string }) {
  return (
    <svg
      className={className}
      fill="none"
      stroke="currentColor"
      viewBox="0 0 24 24"
      aria-hidden="true"
    >
      <path
        strokeLinecap="round"
        strokeLinejoin="round"
        strokeWidth={2}
        d="M9.663 17h4.673M12 3v1m6.364 1.636l-.707.707M21 12h-1M4 12H3m3.343-5.657l-.707-.707m2.828 9.9a5 5 0 117.072 0l-.548.547A3.374 3.374 0 0014 18.469V19a2 2 0 11-4 0v-.531c0-.895-.356-1.754-.988-2.386l-.548-.547z"
      />
    </svg>
  );
}

/**
 * Get the appropriate icon component for a type
 */
function TypeIcon({ type, className }: { type: PostType; className: string }) {
  switch (type) {
    case 'problem':
      return <ProblemIcon className={className} />;
    case 'question':
      return <QuestionIcon className={className} />;
    case 'idea':
      return <IdeaIcon className={className} />;
    default:
      return null;
  }
}

/**
 * TypeBadge displays post type with icon and appropriate styling
 */
export default function TypeBadge({
  type,
  size = 'md',
  showLabel = true,
}: TypeBadgeProps) {
  const colorClasses = getTypeColorClasses(type);
  const sizeClasses = getSizeClasses(size);
  const iconSize = getIconSize(size);

  return (
    <span
      className={`inline-flex items-center gap-1 rounded font-medium capitalize ${colorClasses} ${sizeClasses}`}
    >
      <TypeIcon type={type} className={iconSize} />
      {showLabel && <span>{type}</span>}
    </span>
  );
}
