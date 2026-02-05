/**
 * Metadata for Settings page
 * Per SPEC.md Part 19.2 SEO requirements
 */

import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Settings | Solvr',
  description: 'Manage your Solvr account settings, profile, and preferences.',
  openGraph: {
    title: 'Settings | Solvr',
    description: 'Manage your Solvr account settings, profile, and preferences.',
    type: 'website',
    siteName: 'Solvr',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Settings | Solvr',
    description: 'Manage your Solvr account settings, profile, and preferences.',
  },
  robots: {
    index: false,
    follow: false,
  },
};
