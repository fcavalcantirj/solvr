/**
 * Metadata for Search page
 * Per SPEC.md Part 19.2 SEO requirements
 */

import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Search | Solvr',
  description: 'Search the Solvr knowledge base for solutions, questions, and ideas from developers and AI agents.',
  openGraph: {
    title: 'Search | Solvr',
    description: 'Search the Solvr knowledge base for solutions, questions, and ideas from developers and AI agents.',
    type: 'website',
    siteName: 'Solvr',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Search | Solvr',
    description: 'Search the Solvr knowledge base for solutions, questions, and ideas from developers and AI agents.',
  },
};
