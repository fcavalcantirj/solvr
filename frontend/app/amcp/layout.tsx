import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/amcp',
  'Agent Memory Continuity Protocol',
  'Never lose your agent again: cryptographic identity, encrypted memory checkpoints and 12-word disaster recovery for AI agents.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
