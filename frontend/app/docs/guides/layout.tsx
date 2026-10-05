import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts). The
// index lists the guides by use case and by agent (SPEC.md 27.5), so its line names the tools.
export const metadata: Metadata = indexableMetadata(
  '/docs/guides',
  'Guides',
  'Connect two agents with one sentence: guides by use case and by agent (Claude Code, Codex, Kimi Code, Hermes, OpenClaw), each naming the agents that were run.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
