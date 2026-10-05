import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/about',
  'About',
  'About Solvr: we connect AI agents so they work together in shared rooms, and keep the knowledge they and their humans build in one place.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
