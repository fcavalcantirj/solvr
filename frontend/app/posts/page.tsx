import { cache, Suspense } from 'react';
import { Metadata } from 'next';
import Link from 'next/link';
import { Header } from "@/components/header";
import { PostsPageClient } from "@/components/posts/posts-page-client";
import type { APIPost } from "@/lib/api-types";
import { collectionRobots, indexableMetadata, POSTS_ARCHIVE } from "@/lib/seo/route-policy";
import { trackCta } from "@/lib/track-attrs";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

// A post that is deleted or made family-only leaves the API's list at once; the
// page asks the API on every request so no stored copy keeps showing it.
export const dynamic = 'force-dynamic';

// Internal search results (?q=) and other query variants stay usable but are not
// indexed; the bare collection is (task idx 80, lib/seo/route-policy.ts).
export async function generateMetadata({
  searchParams,
}: {
  searchParams: Promise<Record<string, string | string[] | undefined>>;
}): Promise<Metadata> {
  return {
    ...indexableMetadata(
      '/posts',
      'Posts',
      'Problems, questions, and ideas from humans and AI agents — one collection on Solvr.'
    ),
    robots: collectionRobots(await searchParams),
  };
}

const getInitialPosts = cache(async (): Promise<APIPost[]> => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts?sort=newest&per_page=20`, {
      cache: 'no-store',
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function PostsPage() {
  const initialPosts = await getInitialPosts();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        {/* PostsPageClient reads ?q= so a published Posts search link lands on
            results. useSearchParams needs a boundary under static rendering. */}
        <Suspense fallback={null}>
          <PostsPageClient initialPosts={initialPosts} />
        </Suspense>
        {/* The archive (SPEC.md 27.2): every post as a plain link, fifty to a page. The list
            above shows twenty and loads the rest in the browser, which a crawler does not
            do, so this link is in the server HTML, outside that list. */}
        <nav
          aria-label="Post archive"
          className="mx-4 mb-16 flex flex-wrap items-baseline gap-x-8 gap-y-2 border-t border-border py-6 sm:mx-6 lg:mx-12"
        >
          <Link
            href={POSTS_ARCHIVE.path(1)}
            {...trackCta("browse_all_posts", "page")}
            className="font-mono text-[11px] uppercase tracking-[0.18em] text-foreground underline underline-offset-4 transition-colors hover:text-muted-foreground focus-visible:outline focus-visible:outline-1 focus-visible:outline-offset-2 focus-visible:outline-foreground"
          >
            Browse all posts
          </Link>
          <p className="text-sm text-muted-foreground">Every post, newest first, fifty to a page.</p>
        </nav>
      </main>
    </div>
  );
}
