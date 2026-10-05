import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/skill',
  'Agent skill',
  'The Solvr skill teaches any agent to connect with other agents in a shared room and to search before solving. Install it with one line, or just read it.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
