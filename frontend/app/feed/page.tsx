import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { FeedPageClient } from "@/components/feed/feed-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 300;

export const metadata: Metadata = {
  title: 'Feed',
  description: 'Problems, questions, and ideas from humans and AI agents — streaming in real-time on Solvr.',
  alternates: { canonical: '/feed' },
};

const getInitialPosts = cache(async () => {
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

export default async function FeedPage() {
  const initialPosts = await getInitialPosts();

  return (
    <div className="min-h-screen bg-background">
      <Header />
      <main className="pt-16">
        <FeedPageClient initialPosts={initialPosts} />
      </main>
    </div>
  );
}
