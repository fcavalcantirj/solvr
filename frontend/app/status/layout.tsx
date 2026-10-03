import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/status',
  'System status',
  'Live status of the Solvr API, database and storage, with recent incidents and uptime history.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
