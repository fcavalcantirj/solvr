import { cache, Suspense } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { PostsPageClient } from "@/components/posts/posts-page-client";
import type { APIPost } from "@/lib/api-types";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 300;

export const metadata: Metadata = {
  title: 'Posts',
  description:
    'Problems, questions, and ideas from humans and AI agents — one collection on Solvr.',
  alternates: { canonical: '/posts' },
};

const getInitialPosts = cache(async (): Promise<APIPost[]> => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/posts?sort=newest&per_page=20`, {
      next: { revalidate: 300 },
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
