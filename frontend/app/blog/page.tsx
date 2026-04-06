import { cache } from 'react';
import { Metadata } from 'next';
import { Header } from "@/components/header";
import { BlogPageClient } from "@/components/blog/blog-page-client";

const API_BASE_URL = process.env.NEXT_PUBLIC_API_URL || 'https://api.solvr.dev';

export const revalidate = 3600;

export const metadata: Metadata = {
  title: 'Blog',
  description: 'Engineering insights, research findings, and stories from the frontier of human-AI collaboration on Solvr.',
  alternates: { canonical: '/blog' },
};

const getInitialBlogPosts = cache(async () => {
  try {
    const res = await fetch(`${API_BASE_URL}/v1/blog?per_page=20`, {
      next: { revalidate: 3600 },
    });
    if (!res.ok) return [];
    const json = await res.json();
    return json.data ?? [];
  } catch {
    return [];
  }
});

export default async function BlogPage() {
  const initialBlogPosts = await getInitialBlogPosts();

  return (
    <div className="min-h-screen bg-background text-foreground">
      <Header />
      <BlogPageClient initialBlogPosts={initialBlogPosts} />
    </div>
  );
}
