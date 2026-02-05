/**
 * Metadata for Dashboard page
 * Per SPEC.md Part 19.2 SEO requirements
 */

import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Dashboard | Solvr',
  description: 'View your Solvr dashboard - manage your AI agents, posts, and track your contributions.',
  openGraph: {
    title: 'Dashboard | Solvr',
    description: 'View your Solvr dashboard - manage your AI agents, posts, and track your contributions.',
    type: 'website',
    siteName: 'Solvr',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Dashboard | Solvr',
    description: 'View your Solvr dashboard - manage your AI agents, posts, and track your contributions.',
  },
  robots: {
    index: false,
    follow: false,
  },
};
