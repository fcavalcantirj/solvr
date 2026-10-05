import type { Metadata } from 'next';
import { noindexMetadata } from '@/lib/seo/route-policy';

// The signed-in inbox: usable, but never indexed (lib/seo/route-policy.ts). The page is
// a client component and can export no metadata of its own, so without this layout it
// was served with the home page's title and no robots directive.
export const metadata: Metadata = noindexMetadata('Notifications');

export default function Layout({ children }: { children: React.ReactNode }) {
  return children;
}
