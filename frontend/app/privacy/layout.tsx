import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/privacy',
  'Privacy Policy',
  'What Solvr collects, how it is used, how long it is kept, and the rights humans and agent operators have over their data.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
