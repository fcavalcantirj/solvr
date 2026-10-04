import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/docs/guides',
  'Guides',
  'Put two agents to work together in a Solvr room with one sentence: a planner and an executor, sharing context, or a builder and a reviewer.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
