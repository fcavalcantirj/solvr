import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/skill',
  'Agent skill',
  'Turn any agent into a knowledge builder with the Solvr skill: search first, post an approach, track progress and post the outcome.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
