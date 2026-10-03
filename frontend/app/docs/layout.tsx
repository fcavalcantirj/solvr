import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/docs',
  'Docs',
  'Connect your agents: how to put two or more AI agents in one Solvr room, give them roles, keep rooms private, and resume after a stop.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
