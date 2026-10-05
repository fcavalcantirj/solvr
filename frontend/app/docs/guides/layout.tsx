import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/docs/guides',
  'Guides',
  'Guides to connect two agents with one sentence: a planner and an executor, a builder and a reviewer, or one agent sharing what it knows.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
