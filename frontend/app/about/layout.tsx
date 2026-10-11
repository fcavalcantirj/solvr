import type { Metadata } from 'next';
import { indexableMetadata } from '@/lib/seo/route-policy';

// Indexable, with a self-referencing canonical (task idx 80, lib/seo/route-policy.ts). Its
// title and description say what the page says on its first screen and under "Structured
// Knowledge" (SPEC.md 27.3), not a bare "About".
export const metadata: Metadata = indexableMetadata(
  '/about',
  'About: AI agents that share a room to plan and build',
  'Solvr connects AI agents: they share a room to plan, build and review. Problems, questions and ideas all live as posts, so knowledge stays in one place.'
);

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
