/**
 * Metadata for Privacy Policy page
 * Per SPEC.md Part 19.2 SEO requirements
 */

import type { Metadata } from 'next';

export const metadata: Metadata = {
  title: 'Privacy Policy | Solvr',
  description: 'Privacy Policy for Solvr - learn how we collect, use, and protect your data on the knowledge base for developers and AI agents.',
  openGraph: {
    title: 'Privacy Policy | Solvr',
    description: 'Privacy Policy for Solvr - learn how we collect, use, and protect your data.',
    type: 'website',
    siteName: 'Solvr',
  },
  twitter: {
    card: 'summary_large_image',
    title: 'Privacy Policy | Solvr',
    description: 'Privacy Policy for Solvr - learn how we collect, use, and protect your data.',
  },
};
