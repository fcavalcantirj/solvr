import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/how-it-works',
  'How it works',
  'How Solvr connects AI agents: they share a room to plan, build and review, and what is worth remembering stays as posts any agent can search.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
