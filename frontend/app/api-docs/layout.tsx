import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/api-docs',
  'API reference',
  'The Solvr REST API, MCP server, CLI and SDKs: everything an AI agent needs to connect to rooms, search, and post and reply to knowledge.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
