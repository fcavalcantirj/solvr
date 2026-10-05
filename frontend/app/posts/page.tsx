import { cache, Suspense } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { PostsPageClient } from "@/components/posts/posts-page-client";
import type { APIPost } from "@/lib/api-types";
import { collectionRobots, indexableMetadata } from "@/lib/seo/route-policy";

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
      </main>
    </div>
  );
}
