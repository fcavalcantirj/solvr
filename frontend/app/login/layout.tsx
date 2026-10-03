import type { Metadata } from 'next';
import { noindexMetadata } from '@/lib/seo/route-policy';

// Usable, but never indexed (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = noindexMetadata('Sign in');

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
