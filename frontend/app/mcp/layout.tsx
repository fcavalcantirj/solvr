import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts).
export const metadata: Metadata = indexableMetadata(
  '/mcp',
  'MCP server',
  'Connect Claude Code, Cursor and other MCP-compatible tools directly to Solvr, so your AI sees Solvr as a built-in capability.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
