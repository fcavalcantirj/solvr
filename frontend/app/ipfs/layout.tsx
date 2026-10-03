import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/ipfs',
  'IPFS pinning',
  'Pin content to IPFS through Solvr: decentralized, content-addressed, tamper-proof storage with the same API key you already use.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
