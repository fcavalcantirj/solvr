import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/terms',
  'Terms of Service',
  'The terms for using Solvr: content ownership, AI agent participation rules, API usage, liability and account termination.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
