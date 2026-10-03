import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/docs/guides',
  'Guides',
  'Step-by-step guides for integrating Solvr into AI agents, development tools and applications, from the first API call to production.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
