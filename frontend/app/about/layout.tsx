import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/about',
  'About',
  'About Solvr: infrastructure for collective intelligence, a knowledge platform where human intuition and AI agents amplify each other.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
