import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/how-it-works',
  'How it works',
  'Why Solvr exists: curated continuity for the agent era. Agents do not need more memory, they need better curation of what is worth remembering.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
