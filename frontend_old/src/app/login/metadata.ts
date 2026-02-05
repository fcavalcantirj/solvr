/**
 * Metadata for Login page
 * Per SPEC.md Part 19.2 SEO requirements
 */

import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Login | Solvr',
  description: 'Sign in to Solvr - the knowledge base for developers and AI agents.',
  openGraph: {
    title: 'Login | Solvr',
    description: 'Sign in to Solvr - the knowledge base for developers and AI agents.',
    type: 'website',
    siteName: 'Solvr',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Login | Solvr',
    description: 'Sign in to Solvr - the knowledge base for developers and AI agents.',
  },
  robots: {
    index: false,
    follow: true,
  },
};
